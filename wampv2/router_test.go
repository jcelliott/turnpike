package wampv2

import "testing"
import "time"

const test_realm = URI("test.realm")

type basicEndpoint struct {
	*localEndpoint
}

func (ep *basicEndpoint) Receive() <-chan Message {
	return ep.incoming
}
func (ep *basicEndpoint) Send(msg Message) error {
	if msg.MessageType() == GOODBYE {
		ep.localEndpoint.Send(&Goodbye{})
	}
	return ep.localEndpoint.Send(msg)
}

// satisfy ErrorHandler
func (ep *basicEndpoint) SendError(msg *Error) {
	ep.Send(msg)
}

// satisfy Publisher
func (ep *basicEndpoint) SendPublished(msg *Published) {
	ep.Send(msg)
}

// satisfy Subscriber
func (ep *basicEndpoint) SendEvent(msg *Event) {
	ep.Send(msg)
}
func (ep *basicEndpoint) SendUnsubscribed(msg *Unsubscribed) {
	ep.Send(msg)
}
func (ep *basicEndpoint) SendSubscribed(msg *Subscribed) {
	ep.Send(msg)
}

func (ep *basicEndpoint) Close() error {
	close(ep.outgoing)
	return nil
}

func basicConnect(t *testing.T, ep *basicEndpoint, server Endpoint) *BasicRouter {
	r := NewBasicRouter()
	r.RegisterRealm(test_realm, NewBasicRealm())

	ep.Send(&Hello{Realm: test_realm})
	if err := r.Accept(server); err != nil {
		t.Fatal(err)
	}

	if len(ep.incoming) != 1 {
		t.Fatal("Expected 1 message in the handshake, received %d", len(ep.incoming))
	}

	if msg := <-ep.incoming; msg.MessageType() != WELCOME {
		t.Fatal("Expected first message sent to be a wescome message")
	}
	return r
}

func TestHandshake(t *testing.T) {
	client, server := pipe()

	ep := &basicEndpoint{client}
	r := basicConnect(t, ep, server)
	defer r.Close()

	ep.outgoing <- &Goodbye{}
	select {
	case <-time.After(time.Millisecond):
		t.Errorf("No goodbye message received after sending goodbye")
	case msg := <-ep.incoming:
		if _, ok := msg.(*Goodbye); !ok {
			t.Errorf("Expected GOODBYE, actually got: %s", msg.MessageType())
		}
	}
}

func TestInvalidRealm(t *testing.T) {
	r := NewBasicRouter()
	defer r.Close()

	client, server := pipe()

	ep := &basicEndpoint{client}
	ep.Send(&Hello{Realm: "does.not.exist"})
	err := r.Accept(server)
	if err != nil {
		t.Fatal(err)
	}

	if len(ep.incoming) != 1 {
		t.Fatalf("Expected a single message in the handshake, received %d", len(ep.incoming))
	}

	if msg := <-ep.incoming; msg.MessageType() != ABORT {
		t.Errorf("Expected the handshake to be aborted")
	}
}

func TestPublishNoAcknowledge(t *testing.T) {
	client, server := pipe()
	ep := &basicEndpoint{client}
	r := basicConnect(t, ep, server)
	defer r.Close()

	id := NewID()
	ep.outgoing <- &Publish{Request: id, Options: map[string]interface{}{"acknowledge": false}, Topic: "some.uri"}
	select {
	case <-time.After(time.Millisecond):
	case msg := <-ep.incoming:
		if _, ok := msg.(*Published); ok {
			t.Fatalf("Sent acknowledge=false, but received PUBLISHED: %s", msg.MessageType())
		}
	}
}

func TestPublishAbsentAcknowledge(t *testing.T) {
	client, server := pipe()
	ep := &basicEndpoint{client}
	r := basicConnect(t, ep, server)
	defer r.Close()

	id := NewID()
	ep.outgoing <- &Publish{Request: id, Topic: "some.uri"}
	select {
	case <-time.After(time.Millisecond):
	case msg := <-ep.incoming:
		if _, ok := msg.(*Published); ok {
			t.Fatalf("Sent acknowledge=false, but received PUBLISHED: %s", msg.MessageType())
		}
	}
}

func TestPublishAcknowledge(t *testing.T) {
	client, server := pipe()
	ep := &basicEndpoint{client}
	r := basicConnect(t, ep, &basicEndpoint{server})
	defer r.Close()

	id := NewID()
	ep.outgoing <- &Publish{Request: id, Options: map[string]interface{}{"acknowledge": true}, Topic: "some.uri"}
	select {
	case <-time.After(time.Millisecond):
		t.Error("Sent acknowledge=true, but timed out waiting for PUBLISHED")
	case msg := <-ep.incoming:
		if pub, ok := msg.(*Published); !ok {
			t.Errorf("Sent acknowledge=true, but received %s instead of PUBLISHED: %+v", msg.MessageType(), msg)
		} else if pub.Request != id {
			t.Errorf("Request id does not match the one sent: %d != %d", pub.Request, id)
		}
	}
}

func TestSubscribe(t *testing.T) {
	const test_topic = URI("some.uri")

	subClient, subServer := pipe()
	sub := &basicEndpoint{subClient}
	r := basicConnect(t, sub, &basicEndpoint{subServer})
	defer r.Close()

	subscribeId := NewID()
	sub.Send(&Subscribe{Request: subscribeId, Topic: test_topic})

	var subscriptionId ID
	select {
	case <-time.After(time.Millisecond):
		t.Fatal("Timed out waiting for SUBSCRIBED")
	case msg := <-sub.incoming:
		if subMsg, ok := msg.(*Subscribed); !ok {
			t.Fatalf("Expected SUBSCRIBED, but received %s instead: %+v", msg.MessageType(), msg)
		} else if subMsg.Request != subscribeId {
			t.Fatalf("Request id does not match the one sent: %d != %d", subMsg.Request, subscribeId)
		} else {
			subscriptionId = subMsg.Subscription
		}
	}

	pubClient, pubServer := pipe()
	pub := &basicEndpoint{pubClient}
	pub.Send(&Hello{Realm: test_realm})
	if err := r.Accept(&basicEndpoint{pubServer}); err != nil {
		t.Fatal("Error pubing publisher")
	}
	pub_id := NewID()
	pub.Send(&Publish{Request: pub_id, Topic: test_topic})

	select {
	case <-time.After(time.Millisecond):
		t.Fatal("Timed out waiting for EVENT")
	case msg := <-sub.incoming:
		if event, ok := msg.(*Event); !ok {
			t.Errorf("Expected EVENT, but received %s instead: %+v", msg.MessageType(), msg)
		} else if event.Subscription != subscriptionId {
			t.Errorf("Subscription id does not match the one sent: %d != %d", event.Subscription, subscriptionId)
		}
		// TODO: check Details, Arguments, ArgumentsKw
	}
}
