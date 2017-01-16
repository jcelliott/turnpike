package turnpike

import (
	"fmt"
)

const (
	MAX_REQUEST_ID = 1 << 53
)

// Session represents an active WAMP session
type Session struct {
	Peer
	Id      ID
	Details map[string]interface{}

	lastRequestId ID
	kill          chan URI
}

func (s Session) String() string {
	return fmt.Sprintf("%d", s.Id)
}

func (s *Session) NextRequestId() ID {
	s.lastRequestId++
	// max value is 2^53
	if s.lastRequestId > MAX_REQUEST_ID {
		s.lastRequestId = 1
	}
	return s.lastRequestId
}

// localPipe creates two linked sessions. Messages sent to one will
// appear in the Receive of the other. This is useful for implementing
// client sessions
func localPipe() (*localPeer, *localPeer) {
	aToB := make(chan Message, 100)
	bToA := make(chan Message, 100)

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

func (s *localPeer) Send(msg Message) (err error) {
	defer func() {
		// just in case Close is called before Send
		if r := recover(); r != nil {
			err = fmt.Errorf("Attempt to write after Close()")
		}
	}()
	s.outgoing <- msg
	return
}

func (s *localPeer) Close() error {
	if s.outgoing != nil {
		close(s.outgoing)
		s.outgoing = nil
	}
	return nil
}
