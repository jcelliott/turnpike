package turnpike

import "testing"
import "time"

const testRealm = URI("test.realm")

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
func (ep *basicEndpoint) SendError(msg *Error) { ep.Send(msg) }

// satisfy Publisher
func (ep *basicEndpoint) SendPublished(msg *Published) { ep.Send(msg) }

// satisfy Subscriber
func (ep *basicEndpoint) SendEvent(msg *Event)               { ep.Send(msg) }
func (ep *basicEndpoint) SendUnsubscribed(msg *Unsubscribed) { ep.Send(msg) }
func (ep *basicEndpoint) SendSubscribed(msg *Subscribed)     { ep.Send(msg) }

// satisfy Callee
func (ep *basicEndpoint) SendRegistered(msg *Registered)     { ep.Send(msg) }
func (ep *basicEndpoint) SendUnregistered(msg *Unregistered) { ep.Send(msg) }
func (ep *basicEndpoint) SendInvocation(msg *Invocation)     { ep.Send(msg) }

// satisfy Caller
func (ep *basicEndpoint) SendResult(msg *Result) { ep.Send(msg) }

func (ep *basicEndpoint) Close() error {
	close(ep.outgoing)
	return nil
}

func basicConnect(t *testing.T, ep *basicEndpoint, server Endpoint) *DefaultRouter {
	r := NewDefaultRouter()
	r.RegisterRealm(testRealm, NewDefaultRealm())

	ep.Send(&Hello{Realm: testRealm})
	if err := r.Accept(server); err != nil {
		t.Fatal(err)
	}

	if len(ep.incoming) != 1 {
		t.Fatal("Expected 1 message in the handshake, received %d", len(ep.incoming))
	}

	if msg := <-ep.incoming; msg.MessageType() != WELCOME {
		t.Fatal("Expected first message sent to be a welcome message")
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
	r := NewDefaultRouter()
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
	const testTopic = URI("some.uri")

	subClient, subServer := pipe()
	sub := &basicEndpoint{subClient}
	r := basicConnect(t, sub, &basicEndpoint{subServer})
	defer r.Close()

	subscribeId := NewID()
	sub.Send(&Subscribe{Request: subscribeId, Topic: testTopic})

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
	pub.Send(&Hello{Realm: testRealm})
	if err := r.Accept(&basicEndpoint{pubServer}); err != nil {
		t.Fatal("Error pubing publisher")
	}
	pubId := NewID()
	pub.Send(&Publish{Request: pubId, Topic: testTopic})

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

type basicCallee struct{}

func TestCall(t *testing.T) {
	const testProcedure = URI("turnpike.test.endpoint")
	calleeClient, calleeServer := pipe()
	callee := &basicEndpoint{calleeClient}
	r := basicConnect(t, callee, &basicEndpoint{calleeServer})
	defer r.Close()

	registerId := NewID()
	// callee registers remote procedure
	callee.Send(&Register{Request: registerId, Procedure: testProcedure})

	var registrationId ID
	select {
	case <-time.After(time.Millisecond):
		t.Fatal("Timed out waiting for REGISTERED")
	case msg := <-callee.incoming:
		if registered, ok := msg.(*Registered); !ok {
			t.Fatalf("Expected REGISTERED, but received %s instead: %+v", msg.MessageType(), msg)
		} else if registered.Request != registerId {
			t.Fatalf("Request id does not match the one sent: %d != %d", registered.Request, registerId)
		} else {
			registrationId = registered.Registration
		}
	}

	callerClient, callerServer := pipe()
	caller := &basicEndpoint{callerClient}
	caller.Send(&Hello{Realm: testRealm})
	if err := r.Accept(&basicEndpoint{callerServer}); err != nil {
		t.Fatal("Error connecting caller")
	}
	if msg := <-caller.incoming; msg.MessageType() != WELCOME {
		t.Fatal("Expected first message sent to be a welcome message")
	}
	callId := NewID()
	// caller calls remote procedure
	caller.Send(&Call{Request: callId, Procedure: testProcedure})

	var invocationId ID
	select {
	case <-time.After(time.Millisecond):
		t.Fatal("Timed out waiting for INVOCATION")
	case msg := <-callee.incoming:
		if invocation, ok := msg.(*Invocation); !ok {
			t.Errorf("Expected INVOCATION, but received %s instead: %+v", msg.MessageType(), msg)
		} else if invocation.Registration != registrationId {
			t.Errorf("Registration id does not match the one assigned: %d != %d", invocation.Registration, registrationId)
		} else {
			invocationId = invocation.Request
		}
	}

	// callee returns result of remove procedure
	callee.Send(&Yield{Request: invocationId})

	select {
	case <-time.After(time.Millisecond):
		t.Fatal("Timed out waiting for RESULT")
	case msg := <-caller.incoming:
		if result, ok := msg.(*Result); !ok {
			t.Errorf("Expected RESULT, but received %s instead: %+v", msg.MessageType(), msg)
		} else if result.Request != callId {
			t.Errorf("Result id does not match the call id: %d != %d", result.Request, callId)
		}
	}
}
