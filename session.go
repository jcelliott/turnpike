package turnpike

import (
	"fmt"
)

// Session represents an active WAMP session
type Session struct {
	Peer
	Id      ID
	Details map[string]interface{}

	kill chan URI
}

func (s Session) String() string {
	return fmt.Sprintf("%d", s.Id)
}

// localPipe creates two linked sessions. Messages sent to one will
// appear in the Receive of the other. This is useful for implementing
// client sessions
func localPipe() (*localPeer, *localPeer) {
	aToB := make(chan Message, 10)
	bToA := make(chan Message, 10)

	a := &localPeer{
		incoming: bToA,
		outgoing: aToB,
	}
	b := &localPeer{
		incoming: aToB,
		outgoing: bToA,
	}

	return a, b
}

type localPeer struct {
	outgoing chan<- Message
	incoming <-chan Message
}

func (s *localPeer) Receive() <-chan Message {
	return s.incoming
}

func (s *localPeer) Send(msg Message) error {
	s.outgoing <- msg
	return nil
}

func (s *localPeer) Close() error {
	close(s.outgoing)
	return nil
}
