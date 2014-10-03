package turnpike

type Sender interface {
	// Send a message to the peer
	Send(Message) error
}

// Peer is the interface that must be implemented by all WAMP peers.
type Peer interface {
	Sender

	// Closes the peer connection and any channel returned from Receive().
	// Multiple calls to Close() will have no effect.
	Close() error
	// TODO: implement this
	// TODO: rename this to Receive and Receive to ReceiveChan or similar
	// ReceiveMsg returns a single message from the peer
	// ReceiveMsg() Message

	// Receive returns a channel of messages coming from the peer.
	Receive() <-chan Message
}
