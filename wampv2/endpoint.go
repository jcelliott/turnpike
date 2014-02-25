package wampv2

type Endpoint interface {
	Close() error
	Send(Message) error
	Receive() (Message, error)
}
