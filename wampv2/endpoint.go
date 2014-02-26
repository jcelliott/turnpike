package wampv2

type Sender interface {
	// Send a message to the client
	Send(Message) error
}

// Endpoint is the interface implemented by all WAMP endpoints.
type Endpoint interface {
	Sender

	// Closes the endpoint and any channel returned from Receive().
	// Multiple calls to Close() will have no effect.
	Close() error
	// Receive returns a channel of messages coming from the endpoint.
	Receive() <-chan Message
}
