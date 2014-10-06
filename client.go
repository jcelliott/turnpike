package turnpike

import (
	"fmt"
	"time"
)

const (
	PUBLISHER = 1 << iota
	SUBSCRIBER
	CALLEE
	CALLER
	ALL = PUBLISHER | SUBSCRIBER | CALLEE | CALLER
)

type Client struct {
	Peer
	ReceiveTimeout time.Duration
	roles          int
	listeners      map[ID]chan Message
	events         map[ID]EventHandler
	calls          map[ID]chan Message
	welcome        chan Message
	requestCount   uint
}

func NewWebsocketClient(serialization int, url string, realm URI, roles int) (*Client, error) {
	p, err := NewWebsocketPeer(serialization, url, "")
	if err != nil {
		return nil, err
	}
	c := &Client{
		Peer:           p,
		ReceiveTimeout: 5 * time.Second,
		roles:          roles,
		listeners:      make(map[ID]chan Message),
		calls:          make(map[ID]chan Message),
		events:         make(map[ID]EventHandler),
		welcome:        make(chan Message),
		requestCount:   0,
	}
	go c.Receive()

	roles_map := make(map[string]interface{})
	roles_map["publisher"] = make(map[string]interface{})
	roles_map["subscriber"] = make(map[string]interface{})
	roles_map["callee"] = make(map[string]interface{})
	roles_map["caller"] = make(map[string]interface{})

	details := make(map[string]interface{})
	details["roles"] = roles_map

	c.Send(&Hello{Realm: realm, Details: details})
	return c, nil
}

func (c *Client) WaitForSession() (msg Message, err error) {
	// wait to receive WELCOME message
	select {
	case msg := <-c.welcome:
		return msg, nil
	case <-time.After(c.ReceiveTimeout):
		return nil, fmt.Errorf("timeout while waiting for message")
	}
}

func (c *Client) nextID() ID {
	c.requestCount++
	return ID(c.requestCount)
}

func (c *Client) Receive() {
	for msg := range c.Peer.Receive() {
		switch msg := msg.(type) {
		case *Event:
			if fn, ok := c.events[msg.Subscription]; ok {
				go fn(msg.Arguments, msg.ArgumentsKw)
			} else {
				log.Println("no handler registered for subscription:", msg.Subscription)
			}

		case *Result:
			if l, ok := c.calls[msg.Request]; ok {
				l <- msg
			} else {
				log.Println("no handler registered for call:", msg.Request)
			}

		case *Welcome:
			c.welcome <- msg

		case *Subscribed:
			if l, ok := c.listeners[msg.Request]; ok {
				l <- msg
			} else {
				log.Println("no listener for subscribed:", msg.Request)
			}
		default:
			log.Println(msg.MessageType(), msg)
		}
	}
	log.Fatal("Receive buffer closed")
}

func (c *Client) registerListener(id ID) {
	log.Println("register listener:", id)
	wait := make(chan Message, 1)
	c.listeners[id] = wait
}

func (c *Client) waitOnListener(id ID) (msg Message, err error) {
	log.Println("wait on listener:", id)
	if wait, ok := c.listeners[id]; !ok {
		return nil, fmt.Errorf("unknown listener ID: %v", id)
	} else {
		select {
		case msg = <-wait:
			return
		case <-time.After(c.ReceiveTimeout):
			err = fmt.Errorf("timeout while waiting for message")
			return
		}
	}
}

type EventHandler func(args []interface{}, kwargs map[string]interface{})

func (c *Client) Subscribe(topic URI, fn EventHandler) error {
	id := c.nextID()
	c.registerListener(id)
	if err := c.Send(&Subscribe{Request: id, Topic: topic}); err != nil {
		return err
	}
	// wait to receive SUBSCRIBED message
	msg, err := c.waitOnListener(id)
	if err != nil {
		return err
	} else if e, ok := msg.(*Error); ok {
		return fmt.Errorf("error subscribing to topic '%v': %v", topic, e.Error)
	} else if subscribed, ok := msg.(*Subscribed); !ok {
		return fmt.Errorf("unexpected message received: %s", msg.MessageType())
	} else {
		// register the event handler with this subscription
		c.events[subscribed.Subscription] = fn
	}
	return nil
}

func (c *Client) Publish(topic URI, args []interface{}, kwargs map[string]interface{}) error {
	return c.Send(&Publish{
		Request:     c.nextID(),
		Topic:       topic,
		Arguments:   args,
		ArgumentsKw: kwargs,
	})
}

func (c *Client) Call(procedure URI, args []interface{}, kwargs map[string]interface{}) (msg Message, err error) {
	callId := c.nextID()
	result := make(chan Message, 1)

	c.calls[callId] = result

	if err := c.Send(&Call{
		Request:     callId,
		Procedure:   procedure,
		Options:     make(map[string]interface{}),
		Arguments:   args,
		ArgumentsKw: kwargs,
	}); err != nil {
		return nil, err
	}

	// wait to receive RESULT message
	select {
	case msg = <-result:
		return msg, nil
	case <-time.After(c.ReceiveTimeout):
		return nil, fmt.Errorf("timeout while waiting for message")
	}
}
