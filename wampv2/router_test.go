package wampv2

import "testing"

type basicEndpoint struct {
	outgoing chan Message
	incoming chan Message
}
func (ep basicEndpoint) Receive() (Message, error) {
	return <-ep.outgoing, nil
}
func (ep basicEndpoint) Send(msg Message) error {
	if msg.MessageType() == GOODBYE {
		ep.outgoing<-&Goodbye{}
	}
	ep.incoming<-msg
	return nil
}
func (ep basicEndpoint) Close() error {
	close(ep.outgoing)
	close(ep.incoming)
	return nil
}

func TestHandshake(t *testing.T) {
	const test_realm = "test.realm"

	r := NewRouter()
	r.RegisterRealm(test_realm, 3)

	ep := basicEndpoint{outgoing: make(chan Message, 5), incoming: make(chan Message, 5)}
	ep.outgoing<-&Hello{Realm: test_realm}
	err := r.Accept(ep)
	if err != nil {
		t.Error(err)
	}

	if len(ep.incoming) < 2 {
		t.Fatalf("Expected 2 messages in the handshake, received %d", len(ep.incoming))
	} else if len(ep.incoming) != 2 {
		t.Errorf("Expected 2 messages in the handshake, received %d", len(ep.incoming))
	}

	if msg := <-ep.incoming; msg.MessageType() != WELCOME {
		t.Errorf("Expected first message sent t obe a wescome message")
	}
}

func TestInvalidRealm(t *testing.T) {
	r := NewRouter()
	ep := basicEndpoint{outgoing: make(chan Message, 5), incoming: make(chan Message, 5)}
	ep.outgoing<-&Hello{Realm: "does.not.exist"}
	err := r.Accept(ep)
	if err == nil {
		t.Error("Expected error when invalid realm provided")
	}

	if len(ep.incoming) != 1 {
		t.Fatalf("Expected a single message in the handshake, received %d", len(ep.incoming))
	}

	if msg := <-ep.incoming; msg.MessageType() != ABORT {
		t.Errorf("Expected the handshake to be aborted")
	}
}
