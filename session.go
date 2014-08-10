package turnpike

type Session struct {
	Client
	Id ID

	kill chan URI
}

// Pipe creates two linked sessions. Messages sent to one will
// appear in the Receive of the other. This is useful for implementing
// client sessions
func pipe() (*localClient, *localClient) {
	aToB := make(chan Message, 10)
	bToA := make(chan Message, 10)

	a := &localClient{
		incoming: bToA,
		outgoing: aToB,
	}
	b := &localClient{
		incoming: aToB,
		outgoing: bToA,
	}

	return a, b
}

type localClient struct {
	outgoing chan<- Message
	incoming <-chan Message
}

// Creates a local session that passes messages directly
// into the Router.
func NewLocalClient(r Router) Client {
	a, b := pipe()
	go r.Accept(a)
	return b
}

func (s *localClient) Receive() <-chan Message {
	return s.incoming
}

func (s *localClient) Send(msg Message) error {
	s.outgoing <- msg
	return nil
}

func (s *localClient) Close() error {
	close(s.outgoing)
	return nil
}
