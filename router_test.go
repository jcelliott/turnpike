package turnpike

import "testing"
import "time"

const testRealm = URI("test.realm")

type basicPeer struct {
	*localPeer
}

func (client *basicPeer) Receive() <-chan Message {
	return client.incoming
}
func (client *basicPeer) Send(msg Message) error {
	if msg.MessageType() == GOODBYE {
		client.localPeer.Send(&Goodbye{})
	}
	return client.localPeer.Send(msg)
}

func (client *basicPeer) Close() error {
	close(client.outgoing)
	return nil
}

func basicConnect(t *testing.T, client *basicPeer, server Peer) Router {
	r := NewDefaultRouter()
	r.RegisterRealm(testRealm, Realm{})

	client.Send(&Hello{Realm: testRealm})
	if err := r.Accept(server); err != nil {
		t.Fatal(err)
	}

	if len(client.incoming) != 1 {
		t.Fatalf("Expected 1 message in the handshake, received %d", len(client.incoming))
	}

	if msg := <-client.incoming; msg.MessageType() != WELCOME {
		t.Fatal("Expected first message sent to be a welcome message")
	}
	return r
}

func TestHandshake(t *testing.T) {
	c, server := localPipe()

	client := &basicPeer{c}
	r := basicConnect(t, client, server)
	defer r.Close()

	client.outgoing <- &Goodbye{}
	select {
	case <-time.After(time.Millisecond):
		t.Errorf("No goodbye message received after sending goodbye")
	case msg := <-client.incoming:
		if _, ok := msg.(*Goodbye); !ok {
			t.Errorf("Expected GOODBYE, actually got: %s", msg.MessageType())
		}
	}
}

func TestInvalidRealm(t *testing.T) {
	r := NewDefaultRouter()
	defer r.Close()

	c, server := localPipe()

	client := &basicPeer{c}
	client.Send(&Hello{Realm: "does.not.exist"})
	err := r.Accept(server)
	if err == nil {
		t.Error(err)
	}

	if len(client.incoming) != 1 {
		t.Fatalf("Expected a single message in the handshake, received %d", len(client.incoming))
	}

	if msg := <-client.incoming; msg.MessageType() != ABORT {
		t.Errorf("Expected the handshake to be aborted")
	}
}

func TestPublishNoAcknowledge(t *testing.T) {
	c, server := localPipe()
	client := &basicPeer{c}
	r := basicConnect(t, client, server)
	defer r.Close()

	id := NewID()
	client.outgoing <- &Publish{Request: id, Options: map[string]interface{}{"acknowledge": false}, Topic: "some.uri"}
	select {
	case <-time.After(time.Millisecond):
	case msg := <-client.incoming:
		if _, ok := msg.(*Published); ok {
			t.Fatalf("Sent acknowledge=false, but received PUBLISHED: %s", msg.MessageType())
		}
	}
}

func TestPublishAbsentAcknowledge(t *testing.T) {
	c, server := localPipe()
	client := &basicPeer{c}
	r := basicConnect(t, client, server)
	defer r.Close()

	id := NewID()
	client.outgoing <- &Publish{Request: id, Topic: "some.uri"}
	select {
	case <-time.After(time.Millisecond):
	case msg := <-client.incoming:
		if _, ok := msg.(*Published); ok {
			t.Fatalf("Sent acknowledge=false, but received PUBLISHED: %s", msg.MessageType())
		}
	}
}

func TestPublishAcknowledge(t *testing.T) {
	c, server := localPipe()
	client := &basicPeer{c}
	r := basicConnect(t, client, &basicPeer{server})
	defer r.Close()

	id := NewID()
	client.outgoing <- &Publish{Request: id, Options: map[string]interface{}{"acknowledge": true}, Topic: "some.uri"}
	select {
	case <-time.After(time.Millisecond):
		t.Error("Sent acknowledge=true, but timed out waiting for PUBLISHED")
	case msg := <-client.incoming:
		if pub, ok := msg.(*Published); !ok {
			t.Errorf("Sent acknowledge=true, but received %s instead of PUBLISHED: %+v", msg.MessageType(), msg)
		} else if pub.Request != id {
			t.Errorf("Request id does not match the one sent: %d != %d", pub.Request, id)
		}
	}
}

func TestRouterSubscribe(t *testing.T) {
	const testTopic = URI("some.uri")

	subPeer, subServer := localPipe()
	sub := &basicPeer{subPeer}
	r := basicConnect(t, sub, &basicPeer{subServer})
	defer r.Close()

	subscribeID := NewID()
	sub.Send(&Subscribe{Request: subscribeID, Topic: testTopic})

	var subscriptionID ID
	select {
	case <-time.After(time.Millisecond):
		t.Fatal("Timed out waiting for SUBSCRIBED")
	case msg := <-sub.incoming:
		if subMsg, ok := msg.(*Subscribed); !ok {
			t.Fatalf("Expected SUBSCRIBED, but received %s instead: %+v", msg.MessageType(), msg)
		} else if subMsg.Request != subscribeID {
			t.Fatalf("Request id does not match the one sent: %d != %d", subMsg.Request, subscribeID)
		} else {
			subscriptionID = subMsg.Subscription
		}
	}

	pubClient, pubServer := localPipe()
	pub := &basicPeer{pubClient}
	pub.Send(&Hello{Realm: testRealm})
	if err := r.Accept(&basicPeer{pubServer}); err != nil {
		t.Fatal("Error pubing publisher")
	}
	pubID := NewID()
	pub.Send(&Publish{Request: pubID, Topic: testTopic})

	select {
	case <-time.After(time.Millisecond):
		t.Fatal("Timed out waiting for EVENT")
	case msg := <-sub.incoming:
		if event, ok := msg.(*Event); !ok {
			t.Errorf("Expected EVENT, but received %s instead: %+v", msg.MessageType(), msg)
		} else if event.Subscription != subscriptionID {
			t.Errorf("Subscription id does not match the one sent: %d != %d", event.Subscription, subscriptionID)
		}
		// TODO: check Details, Arguments, ArgumentsKw
	}
}

type basicCallee struct{}

func TestRouterCall(t *testing.T) {
	const testProcedure = URI("turnpike.test.endpoint")
	calleeClient, calleeServer := localPipe()
	callee := &basicPeer{calleeClient}
	r := basicConnect(t, callee, &basicPeer{calleeServer})
	defer r.Close()

	registerID := NewID()
	// callee registers remote procedure
	callee.Send(&Register{Request: registerID, Procedure: testProcedure})

	var registrationID ID
	select {
	case <-time.After(time.Millisecond):
		t.Fatal("Timed out waiting for REGISTERED")
	case msg := <-callee.incoming:
		if registered, ok := msg.(*Registered); !ok {
			t.Fatalf("Expected REGISTERED, but received %s instead: %+v", msg.MessageType(), msg)
		} else if registered.Request != registerID {
			t.Fatalf("Request id does not match the one sent: %d != %d", registered.Request, registerID)
		} else {
			registrationID = registered.Registration
		}
	}

	callerClient, callerServer := localPipe()
	caller := &basicPeer{callerClient}
	caller.Send(&Hello{Realm: testRealm})
	if err := r.Accept(&basicPeer{callerServer}); err != nil {
		t.Fatal("Error connecting caller")
	}
	if msg := <-caller.incoming; msg.MessageType() != WELCOME {
		t.Fatal("Expected first message sent to be a welcome message")
	}
	callID := NewID()
	// caller calls remote procedure
	caller.Send(&Call{Request: callID, Procedure: testProcedure})

	var invocationID ID
	select {
	case <-time.After(time.Millisecond):
		t.Fatal("Timed out waiting for INVOCATION")
	case msg := <-callee.incoming:
		if invocation, ok := msg.(*Invocation); !ok {
			t.Errorf("Expected INVOCATION, but received %s instead: %+v", msg.MessageType(), msg)
		} else if invocation.Registration != registrationID {
			t.Errorf("Registration id does not match the one assigned: %d != %d", invocation.Registration, registrationID)
		} else {
			invocationID = invocation.Request
		}
	}

	// callee returns result of remove procedure
	callee.Send(&Yield{Request: invocationID})

	select {
	case <-time.After(time.Millisecond):
		t.Fatal("Timed out waiting for RESULT")
	case msg := <-caller.incoming:
		if result, ok := msg.(*Result); !ok {
			t.Errorf("Expected RESULT, but received %s instead: %+v", msg.MessageType(), msg)
		} else if result.Request != callID {
			t.Errorf("Result id does not match the call id: %d != %d", result.Request, callID)
		}
	}
}
