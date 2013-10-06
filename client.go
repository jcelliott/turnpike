package turnpike

import (
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
)

const (
	wampProtocolId = "wamp"
)

var clientBacklog = 10

// Client represents a WAMP client that handles RPC and pub/sub.
type Client struct {
	// SessionId is a ID of the session in UUID4 format received at the start of the session.
	SessionId string
	// ProtocolVersion is the version of the WAMP protocol received at the start of the session.
	ProtocolVersion int
	// ServerIdent is the server ID (ie "turnpike, autobahn") received at the start of the session.
	ServerIdent         string
	ws                  *websocket.Conn
	messages            chan string
	prefixes            prefixMap
	sessionOpenCallback func(string)
}

// NewClient creates a new WAMP client.
func NewClient() *Client {
	return &Client{
		messages: make(chan string, clientBacklog),
		prefixes: make(prefixMap),
	}
}

// Prefix sets a CURIE prefix at the server for later use when interacting with
// the server. prefix is the first part of a CURIE (ie "calc") and URI is a full
// identifier (ie "http://example.com/simple/calc#") that is mapped to the prefix.
//
// Ref: http://wamp.ws/spec#prefix_message
func (c *Client) Prefix(prefix, URI string) error {
	log.Trace("sending prefix")
	err := c.prefixes.registerPrefix(prefix, URI)
	if err != nil {
		return fmt.Errorf("turnpike: %s", err)
	}
	msg, err := createPrefix(prefix, URI)
	if err != nil {
		return fmt.Errorf("turnpike: %s", err)
	}
	c.messages <- string(msg)
	return nil
}

// Call makes a RPC call on the server identified by procURI (in either full URI
// or CURIE format) with zero or more args.
//
// Ref: http://wamp.ws/spec#call_message
func (c *Client) Call(procURI string, args ...interface{}) error {
	log.Trace("sending call")
	msg, err := createCall(newId(16), procURI, args...)
	if err != nil {
		return fmt.Errorf("turnpike: %s", err)
	}
	c.messages <- string(msg)
	return nil
}

// Subscribe adds a subscription at the server for events with topicURI lasting
// for the session or until Unsubscribe is called.
//
// Ref: http://wamp.ws/spec#subscribe_message
func (c *Client) Subscribe(topicURI string) error {
	log.Trace("sending subscribe")
	msg, err := createSubscribe(topicURI)
	if err != nil {
		return fmt.Errorf("turnpike: %s", err)
	}
	c.messages <- string(msg)
	return nil
}

// Unsubscribe removes a previous subscription with topicURI at the server.
//
// Ref: http://wamp.ws/spec#unsubscribe_message
func (c *Client) Unsubscribe(topicURI string) error {
	log.Trace("sending unsubscribe")
	msg, err := createUnsubscribe(topicURI)
	if err != nil {
		return fmt.Errorf("turnpike: %s", err)
	}
	c.messages <- string(msg)
	return nil
}

// Publish publishes an event to the topicURI that gets sent to all subscribers
// of that topicURI by the server. opts can can be either empty, one boolean
// that can be used to exclude outself from receiving the event or two lists;
// the first a list of clients to exclude and the second a list of clients that
// are eligible to receive the event. Either list can be empty.
//
// Ref: http://wamp.ws/spec#publish_message
func (c *Client) Publish(topicURI string, event interface{}, opts ...interface{}) error {
	log.Trace("sending publish)")
	msg, err := createPublish(topicURI, event, opts...)
	if err != nil {
		return fmt.Errorf("turnpike: %s", err)
	}
	c.messages <- string(msg)
	return nil
}

// PublishExcludeMe is a short hand for Publish(tobicURI, event, true) that will
// not send the event to ourself.
func (c *Client) PublishExcludeMe(topicURI string, event interface{}) error {
	return c.Publish(topicURI, event, true)
}

func (c *Client) handleCallResult(msg callResultMsg) {
	log.Trace("Handling call result message")
	// TODO:
}

func (c *Client) handleCallError(msg callErrorMsg) {
	log.Trace("Handling call error message")
	// TODO:
}

func (c *Client) handleEvent(msg eventMsg) {
	log.Trace("Handling event message")
	// TODO:
}

func (c *Client) receiveWelcome() error {
	log.Trace("Receive welcome")
	var rec string
	err := websocket.Message.Receive(c.ws, &rec)
	if err != nil {
		return fmt.Errorf("Error receiving welcome message: %s", err)
	}
	if typ := parseMessageType(rec); typ != msgWelcome {
		return fmt.Errorf("First message received was not welcome")
	}
	var msg welcomeMsg
	err = json.Unmarshal([]byte(rec), &msg)
	if err != nil {
		return fmt.Errorf("Error unmarshalling welcome message: %s", err)
	}
	c.SessionId = msg.SessionId
	log.Debug("Session id: %s", c.SessionId)
	c.ProtocolVersion = msg.ProtocolVersion
	log.Debug("Protocol version: %d", c.ProtocolVersion)
	c.ServerIdent = msg.ServerIdent
	log.Debug("Server ident: %s", c.ServerIdent)

	if c.sessionOpenCallback != nil {
		c.sessionOpenCallback(c.SessionId)
	}

	return nil
}

func (c *Client) receive() {
	for {
		var rec string
		err := websocket.Message.Receive(c.ws, &rec)
		if err != nil {
			if err != io.EOF {
				log.Error("Error receiving message, aborting connection: %s", err)
			}
			break
		}
		log.Trace("Message received: %s", rec)

		data := []byte(rec)

		switch typ := parseMessageType(rec); typ {
		case msgCallResult:
			var msg callResultMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				log.Error("Error unmarshalling call result message: %s", err)
				continue
			}
			c.handleCallResult(msg)
		case msgCallError:
			var msg callErrorMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				log.Error("Error unmarshalling call error message: %s", err)
				continue
			}
			c.handleCallError(msg)
		case msgEvent:
			var msg eventMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				log.Error("Error unmarshalling event message: %s", err)
				continue
			}
			c.handleEvent(msg)
		case msgPrefix, msgCall, msgSubscribe, msgUnsubscribe, msgPublish:
			log.Error("Client -> server message received, ignored: %s", messageTypeString(typ))
		case msgWelcome:
			log.Error("Received extraneous welcome message, ignored")
		default:
			log.Error("Invalid message format, message dropped: %s", data)
		}
	}
}

func (c *Client) send() {
	for msg := range c.messages {
		log.Trace("Sending message: %s", msg)
		if err := websocket.Message.Send(c.ws, msg); err != nil {
			log.Error("Error sending message: %s", err)
		}
	}
}

// Connect will connect to server with an optional origin.
// More details here: http://godoc.org/code.google.com/p/go.net/websocket#Dial
func (c *Client) Connect(server, origin string) error {
	log.Trace("connect")
	var err error
	if c.ws, err = websocket.Dial(server, wampProtocolId, origin); err != nil {
		return fmt.Errorf("Error connecting to websocket server: %s", err)
	}

	// Receive welcome message
	if err = c.receiveWelcome(); err != nil {
		return err
	}
	log.Info("Connected to server: %s", server)

	go c.receive()
	go c.send()

	return nil
}

// SetSessionOpenCallback adds a callback function that is run when a new session begins.
// The callback function must accept a string argument that is the session ID.
func (c *Client) SetSessionOpenCallback(f func(string)) {
	c.sessionOpenCallback = f
}

// newId generates a random string of fixed size.
func newId(size int) string {
	const alpha = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz0123456789-_"
	buf := make([]byte, size)
	for i := 0; i < size; i++ {
		buf[i] = alpha[rand.Intn(len(alpha))]
	}
	return string(buf)
}
