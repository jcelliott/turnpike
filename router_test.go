package turnpike

import "testing"
import "time"

const testRealm = URI("test.realm")

type basicClient struct {
	*localClient
}

func (client *basicClient) Receive() <-chan Message {
	return client.incoming
}
func (client *basicClient) Send(msg Message) error {
	if msg.MessageType() == GOODBYE {
		client.localClient.Send(&Goodbye{})
	}
	return client.localClient.Send(msg)
}

// satisfy ErrorHandler
func (client *basicClient) SendError(msg *Error) { client.Send(msg) }

// satisfy Publisher
func (client *basicClient) SendPublished(msg *Published) { client.Send(msg) }

// satisfy Subscriber
func (client *basicClient) SendEvent(msg *Event)               { client.Send(msg) }
func (client *basicClient) SendUnsubscribed(msg *Unsubscribed) { client.Send(msg) }
func (client *basicClient) SendSubscribed(msg *Subscribed)     { client.Send(msg) }

// satisfy Callee
func (client *basicClient) SendRegistered(msg *Registered)     { client.Send(msg) }
func (client *basicClient) SendUnregistered(msg *Unregistered) { client.Send(msg) }
func (client *basicClient) SendInvocation(msg *Invocation)     { client.Send(msg) }

// satisfy Caller
func (client *basicClient) SendResult(msg *Result) { client.Send(msg) }

func (client *basicClient) Close() error {
	close(client.outgoing)
	return nil
}

func basicConnect(t *testing.T, client *basicClient, server Client) *DefaultRouter {
	r := NewDefaultRouter()
	r.RegisterRealm(testRealm, NewDefaultRealm())

	client.Send(&Hello{Realm: testRealm})
	if err := r.Accept(server); err != nil {
		t.Fatal(err)
	}

	if len(client.incoming) != 1 {
		t.Fatal("Expected 1 message in the handshake, received %d", len(client.incoming))
	}

	if msg := <-client.incoming; msg.MessageType() != WELCOME {
		t.Fatal("Expected first message sent to be a welcome message")
	}
	return r
}

func TestHandshake(t *testing.T) {
	c, server := pipe()

	client := &basicClient{c}
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

	c, server := pipe()

	client := &basicClient{c}
	client.Send(&Hello{Realm: "does.not.exist"})
	err := r.Accept(server)
	if err != nil {
		t.Fatal(err)
	}

	if len(client.incoming) != 1 {
		t.Fatalf("Expected a single message in the handshake, received %d", len(client.incoming))
	}

	if msg := <-client.incoming; msg.MessageType() != ABORT {
		t.Errorf("Expected the handshake to be aborted")
	}
}

func TestPublishNoAcknowledge(t *testing.T) {
	c, server := pipe()
	client := &basicClient{c}
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
	c, server := pipe()
	client := &basicClient{c}
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
	c, server := pipe()
	client := &basicClient{c}
	r := basicConnect(t, client, &basicClient{server})
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

func TestSubscribe(t *testing.T) {
	const testTopic = URI("some.uri")

	subClient, subServer := pipe()
	sub := &basicClient{subClient}
	r := basicConnect(t, sub, &basicClient{subServer})
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
	pub := &basicClient{pubClient}
	pub.Send(&Hello{Realm: testRealm})
	if err := r.Accept(&basicClient{pubServer}); err != nil {
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
	callee := &basicClient{calleeClient}
	r := basicConnect(t, callee, &basicClient{calleeServer})
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
	caller := &basicClient{callerClient}
	caller.Send(&Hello{Realm: testRealm})
	if err := r.Accept(&basicClient{callerServer}); err != nil {
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
