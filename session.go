package turnpike

type Session struct {
	Endpoint
	Id ID

	kill chan URI
}

// Pipe creates two linked sessions. Messages sent to one will
// appear in the Receive of the other. This is useful for implementing
// client sessions
func pipe() (*localEndpoint, *localEndpoint) {
	aToB := make(chan Message, 10)
	bToA := make(chan Message, 10)

	a := &localEndpoint{
		incoming: bToA,
		outgoing: aToB,
	}
	b := &localEndpoint{
		incoming: aToB,
		outgoing: bToA,
	}

	return a, b
}

type localEndpoint struct {
	outgoing chan<- Message
	incoming <-chan Message
}

// Creates a local session that passes messages directly
// into the Router.
func NewLocalEndpoint(r Router) Endpoint {
	a, b := pipe()
	go r.Accept(a)
	return b
}

func (s *localEndpoint) Receive() <-chan Message {
	return s.incoming
}

func (s *localEndpoint) Send(msg Message) error {
	s.outgoing <- msg
	return nil
}

func (s *localEndpoint) Close() error {
	close(s.outgoing)
	return nil
}
