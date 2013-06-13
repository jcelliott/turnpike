package turnpike

import (
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	"fmt"
	"io"
)

const (
	WAMP_SUBPROTOCOL_ID = "wamp"
)

var clientBacklog = 10

type Client struct {
	ws              *websocket.Conn
	messages        chan string
	prefixes        PrefixMap
	SessionId       string
	ProtocolVersion int
	ServerIdent     string
}

func NewClient() *Client {
	return &Client{
		messages: make(chan string, clientBacklog),
		prefixes: make(PrefixMap),
	}
}

func (c *Client) Prefix(prefix, URI string) error {
	log.Trace("sending prefix")
	err := c.prefixes.RegisterPrefix(prefix, URI)
	if err != nil {
		return fmt.Errorf("turnpike: %s", err)
	}
	msg, err := CreatePrefix(prefix, URI)
	if err != nil {
		return fmt.Errorf("turnpike: %s", err)
	}
	c.messages <- string(msg)
	return nil
}

func (c *Client) Call(callID, procURI string, args ...interface{}) error {
	log.Trace("sending call")
	msg, err := CreateCall(callID, procURI, args...)
	if err != nil {
		return fmt.Errorf("turnpike: %s", err)
	}
	c.messages <- string(msg)
	return nil
}

func (c *Client) Subscribe(topicURI string) error {
	log.Trace("sending subscribe")
	msg, err := CreateSubscribe(topicURI)
	if err != nil {
		return fmt.Errorf("turnpike: %s", err)
	}
	c.messages <- string(msg)
	return nil
}

func (c *Client) Unsubscribe(topicURI string) error {
	log.Trace("sending unsubscribe")
	msg, err := CreateUnsubscribe(topicURI)
	if err != nil {
		return fmt.Errorf("turnpike: %s", err)
	}
	c.messages <- string(msg)
	return nil
}

func (c *Client) Publish(topicURI string, event interface{}, opts ...interface{}) error {
	log.Trace("sending publish)")
	msg, err := CreatePublish(topicURI, event, opts...)
	if err != nil {
		return fmt.Errorf("turnpike: %s", err)
	}
	c.messages <- string(msg)
	return nil
}

func (c *Client) PublishExcludeMe(topicURI string, event interface{}) error {
	return c.Publish(topicURI, event, true)
}

func (c *Client) handleCallResult(msg CallResultMsg) {
	log.Trace("Handling call result message")
	// TODO:
}

func (c *Client) handleCallError(msg CallErrorMsg) {
	log.Trace("Handling call error message")
	// TODO:
}

func (c *Client) handleEvent(msg EventMsg) {
	log.Trace("Handling event message")
	// TODO:
}

func (c *Client) ReceiveWelcome() error {
	log.Trace("Receive welcome")
	var rec string
	err := websocket.Message.Receive(c.ws, &rec)
	if err != nil {
		return fmt.Errorf("Error receiving welcome message: %s", err)
	}
	if typ := ParseType(rec); typ != WELCOME {
		return fmt.Errorf("First message received was not welcome")
	}
	var msg WelcomeMsg
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
	return nil
}

func (c *Client) Listen() {
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

		switch typ := ParseType(rec); typ {
		case CALLRESULT:
			var msg CallResultMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				log.Error("Error unmarshalling call result message: %s", err)
			}
			c.handleCallResult(msg)
		case CALLERROR:
			var msg CallErrorMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				log.Error("Error unmarshalling call error message: %s", err)
			}
			c.handleCallError(msg)
		case EVENT:
			var msg EventMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				log.Error("Error unmarshalling event message: %s", err)
			}
			c.handleEvent(msg)
		case PREFIX, CALL, SUBSCRIBE, UNSUBSCRIBE, PUBLISH:
			log.Error("Client -> server message received, ignored: %s", TypeString(typ))
		case WELCOME:
			log.Error("Received extraneous welcome message, ignored")
		default:
			log.Error("Invalid message format, message dropped: %s", data)
		}
	}
}

func (c *Client) Send() {
	for msg := range c.messages {
		log.Trace("Sending message: %s", msg)
		if err := websocket.Message.Send(c.ws, msg); err != nil {
			log.Error("Error sending message: %s", err)
		}
	}
}

func (c *Client) Connect(server, origin string) error {
	log.Trace("connect")
	var err error
	if c.ws, err = websocket.Dial(server, WAMP_SUBPROTOCOL_ID, origin); err != nil {
		return fmt.Errorf("Error connecting to websocket server: %s", err)
	}

	// Receive welcome message
	if err = c.ReceiveWelcome(); err != nil {
		return err
	}
	log.Info("Connected to server: %s", server)

	go c.Listen()
	go c.Send()

	return nil
}
