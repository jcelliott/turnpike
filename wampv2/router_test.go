package wampv2

import "testing"
import "time"

const test_realm = URI("test.realm")

type basicEndpoint struct {
	outgoing chan Message
	incoming chan Message
}

func (ep *basicEndpoint) Receive() <-chan Message {
	return ep.outgoing
}
func (ep *basicEndpoint) Send(msg Message) error {
	if msg.MessageType() == GOODBYE {
		ep.outgoing <- &Goodbye{}
	}
	ep.incoming <- msg
	return nil
}

// satisfy ErrorHandler
func (ep *basicEndpoint) SendError(msg *Error) {
	ep.incoming <- msg
}

// satisfy Publisher
func (ep *basicEndpoint) SendPublished(msg *Published) {
	ep.incoming <- msg
}

// satisfy Subscriber
func (ep *basicEndpoint) SendEvent(msg *Event) {
	ep.incoming <- msg
}
func (ep *basicEndpoint) SendUnsubscribed(msg *Unsubscribed) {
	ep.incoming <- msg
}
func (ep *basicEndpoint) SendSubscribed(msg *Subscribed) {
	ep.incoming <- msg
}

func (ep *basicEndpoint) Close() error {
	close(ep.outgoing)
	close(ep.incoming)
	return nil
}

func basicConnect(t *testing.T, ep *basicEndpoint) *Router {
	r := NewRouter()
	r.RegisterRealm(test_realm, NewBasicRealm())

	ep.outgoing <- &Hello{Realm: test_realm}
	err := r.Accept(ep)
	if err != nil {
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
	ep := &basicEndpoint{outgoing: make(chan Message, 5), incoming: make(chan Message, 5)}
	r := basicConnect(t, ep)
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
	r := NewRouter()
	defer r.Close()

	ep := &basicEndpoint{outgoing: make(chan Message, 5), incoming: make(chan Message, 5)}
	ep.outgoing <- &Hello{Realm: "does.not.exist"}
	err := r.Accept(ep)
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
	ep := &basicEndpoint{outgoing: make(chan Message, 5), incoming: make(chan Message, 5)}
	r := basicConnect(t, ep)
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
	ep := &basicEndpoint{outgoing: make(chan Message, 5), incoming: make(chan Message, 5)}
	r := basicConnect(t, ep)
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
	ep := &basicEndpoint{outgoing: make(chan Message, 5), incoming: make(chan Message, 5)}
	r := basicConnect(t, ep)
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

	sub := &basicEndpoint{outgoing: make(chan Message, 5), incoming: make(chan Message, 5)}
	r := basicConnect(t, sub)
	defer r.Close()

	subscribeId := NewID()
	sub.outgoing <- &Subscribe{Request: subscribeId, Topic: test_topic}

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

	pub := &basicEndpoint{outgoing: make(chan Message, 5), incoming: make(chan Message, 5)}
	pub.outgoing <- &Hello{Realm: test_realm}
	if err := r.Accept(pub); err != nil {
		t.Fatal("Error pubing publisher")
	}
	pub_id := NewID()
	pub.outgoing <- &Publish{Request: pub_id, Topic: test_topic}

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
