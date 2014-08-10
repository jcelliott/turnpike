package turnpike

type Sender interface {
	// Send a message to the client
	Send(Message) error
}

// Client is the interface that must be implemented by all WAMP clients.
type Client interface {
	Sender

	// Closes the client connection and any channel returned from Receive().
	// Multiple calls to Close() will have no effect.
	Close() error
	// Receive returns a channel of messages coming from the client.
	Receive() <-chan Message
}
