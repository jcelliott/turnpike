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
	ALLROLES = PUBLISHER | SUBSCRIBER | CALLEE | CALLER
)

var (
	abortUnexpectedMsg = &Abort{
		Details: map[string]interface{}{},
		Reason:  "turnpike.error.unexpected_message_type",
	}
	abortNoAuthHandler = &Abort{
		Details: map[string]interface{}{},
		Reason:  "turnpike.error.no_handler_for_authmethod",
	}
	abortAuthFailure = &Abort{
		Details: map[string]interface{}{},
		Reason:  "turnpike.error.authentication_failure",
	}
	goodbyeClient = &Goodbye{
		Details: map[string]interface{}{},
		Reason:  WAMP_ERROR_CLOSE_REALM,
	}
)

type Client struct {
	Peer
	ReceiveTimeout time.Duration
	// roles          int
	listeners    map[ID]chan Message
	events       map[ID]EventHandler
	procedures   map[ID]MethodHandler
	requestCount uint
}

func NewWebsocketClient(serialization int, url string) (*Client, error) {
	p, err := NewWebsocketPeer(serialization, url, "")
	if err != nil {
		return nil, err
	}
	return newClient(p), nil
}

func newClient(p Peer) *Client {
	c := &Client{
		Peer:           p,
		ReceiveTimeout: 5 * time.Second,
		// roles:          roles,
		listeners:    make(map[ID]chan Message),
		events:       make(map[ID]EventHandler),
		procedures:   make(map[ID]MethodHandler),
		requestCount: 0,
	}
	return c
}

func (c *Client) JoinRealm(realm URI, roles int, details map[string]interface{}) (map[string]interface{}, error) {
	if details == nil {
		details = map[string]interface{}{}
	}
	details["roles"] = createRolesMap(roles)
	if err := c.Send(&Hello{Realm: realm, Details: details}); err != nil {
		c.Peer.Close()
		return nil, err
	}
	if msg, err := GetMessageTimeout(c.Peer, c.ReceiveTimeout); err != nil {
		c.Peer.Close()
		return nil, err
	} else if welcome, ok := msg.(*Welcome); !ok {
		c.Send(abortUnexpectedMsg)
		c.Peer.Close()
		return nil, fmt.Errorf(formatUnexpectedMessage(msg, WELCOME))
	} else {
		go c.Receive()
		return welcome.Details, nil
	}
}

// AuthFunc takes the HELLO details and CHALLENGE details and returns the
// signature string and a details map
type AuthFunc func(map[string]interface{}, map[string]interface{}) (string, map[string]interface{}, error)

func (c *Client) JoinRealmAuth(realm URI, roles int, details map[string]interface{}, auth map[string]AuthFunc) (map[string]interface{}, error) {
	if auth == nil || len(auth) == 0 {
		return nil, fmt.Errorf("no authentication methods provided")
	}
	if details == nil || len(details) == 0 {
		return nil, fmt.Errorf("no details map provided")
	}
	details["roles"] = createRolesMap(roles)
	if err := c.Send(&Hello{Realm: realm, Details: details}); err != nil {
		c.Peer.Close()
		return nil, err
	}
	if msg, err := GetMessageTimeout(c.Peer, c.ReceiveTimeout); err != nil {
		c.Peer.Close()
		return nil, err
	} else if challenge, ok := msg.(*Challenge); !ok {
		c.Send(abortUnexpectedMsg)
		c.Peer.Close()
		return nil, fmt.Errorf(formatUnexpectedMessage(msg, CHALLENGE))
	} else if authFunc, ok := auth[challenge.AuthMethod]; !ok {
		c.Send(abortNoAuthHandler)
		c.Peer.Close()
		return nil, fmt.Errorf("no auth handler for method: %s", challenge.AuthMethod)
	} else if signature, authDetails, err := authFunc(details, challenge.Extra); err != nil {
		c.Send(abortAuthFailure)
		c.Peer.Close()
		return nil, err
	} else if err := c.Send(&Authenticate{Signature: signature, Extra: authDetails}); err != nil {
		c.Peer.Close()
		return nil, err
	}
	if msg, err := GetMessageTimeout(c.Peer, c.ReceiveTimeout); err != nil {
		c.Peer.Close()
		return nil, err
	} else if welcome, ok := msg.(*Welcome); !ok {
		c.Send(abortUnexpectedMsg)
		c.Peer.Close()
		return nil, fmt.Errorf(formatUnexpectedMessage(msg, WELCOME))
	} else {
		go c.Receive()
		return welcome.Details, nil
	}
}

func createRolesMap(roles int) map[string]interface{} {
	rolesMap := make(map[string]interface{})
	if roles&PUBLISHER == PUBLISHER {
		rolesMap["publisher"] = make(map[string]interface{})
	}
	if roles&SUBSCRIBER == SUBSCRIBER {
		rolesMap["subscriber"] = make(map[string]interface{})
	}
	if roles&CALLEE == CALLEE {
		rolesMap["callee"] = make(map[string]interface{})
	}
	if roles&CALLER == CALLER {
		rolesMap["caller"] = make(map[string]interface{})
	}
	return rolesMap
}

func formatUnexpectedMessage(msg Message, expected MessageType) string {
	s := fmt.Sprintf("received unexpected %s message while waiting for %s", msg.MessageType(), expected)
	switch m := msg.(type) {
	case *Abort:
		s += ": " + string(m.Reason)
		s += formatUnknownMap(m.Details)
		return s
	case *Goodbye:
		s += ": " + string(m.Reason)
		s += formatUnknownMap(m.Details)
		return s
	}
	return s
}

func formatUnknownMap(m map[string]interface{}) string {
	s := ""
	for k, v := range m {
		// TODO: reflection to recursively check map
		s += fmt.Sprintf(" %s=%v", k, v)
	}
	return s
}

func (c *Client) LeaveRealm() {
	c.Send(goodbyeClient)
}

func (c *Client) Close() {
	c.Send(goodbyeClient)
	c.Peer.Close()
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

		case *Invocation:
			c.handleInvocation(msg)

		case *Registered:
			c.notifyListener(msg, msg.Request)

		case *Subscribed:
			c.notifyListener(msg, msg.Request)

		case *Result:
			c.notifyListener(msg, msg.Request)

		case *Error:
			c.notifyListener(msg, msg.Request)

		default:
			log.Println("unhandled message:", msg.MessageType(), msg)
		}
	}
	log.Fatal("Receive buffer closed")
}

func (c *Client) notifyListener(msg Message, requestId ID) {
	// pass in the request ID so we don't have to do any type assertion
	if l, ok := c.listeners[requestId]; ok {
		l <- msg
	} else {
		log.Println("no listener for message", msg.MessageType(), requestId)
	}
}

func (c *Client) handleInvocation(msg *Invocation) {
	if fn, ok := c.procedures[msg.Registration]; ok {
		go func() {
			result := fn(msg.Arguments, msg.ArgumentsKw)

			var tosend Message
			tosend = &Yield{
				Request:     msg.Request,
				Options:     make(map[string]interface{}),
				Arguments:   result.args,
				ArgumentsKw: result.kwargs,
			}

			if result.err != "" {
				tosend = &Error{
					Type:        INVOCATION,
					Request:     msg.Request,
					Details:     make(map[string]interface{}),
					Arguments:   result.args,
					ArgumentsKw: result.kwargs,
					Error:       result.err,
				}
			}

			if err := c.Send(tosend); err != nil {
				log.Fatal(err)
			}
		}()
	} else {
		log.Println("no handler registered for registration:", msg.Registration)
	}
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
	sub := &Subscribe{
		Request: id,
		Options: make(map[string]interface{}),
		Topic:   topic,
	}
	if err := c.Send(sub); err != nil {
		return err
	}
	// wait to receive SUBSCRIBED message
	msg, err := c.waitOnListener(id)
	if err != nil {
		return err
	} else if e, ok := msg.(*Error); ok {
		return fmt.Errorf("error subscribing to topic '%v': %v", topic, e.Error)
	} else if subscribed, ok := msg.(*Subscribed); !ok {
		return fmt.Errorf(formatUnexpectedMessage(msg, SUBSCRIBED))
	} else {
		// register the event handler with this subscription
		c.events[subscribed.Subscription] = fn
	}
	return nil
}

type MethodHandler func(args []interface{}, kwargs map[string]interface{}) (result *CallResult)

func (c *Client) Register(procedure URI, fn MethodHandler) error {
	id := c.nextID()
	c.registerListener(id)
	register := &Register{
		Request:   id,
		Options:   make(map[string]interface{}),
		Procedure: procedure,
	}
	if err := c.Send(register); err != nil {
		return err
	}

	// wait to receive REGISTERED message
	msg, err := c.waitOnListener(id)
	if err != nil {
		return err
	} else if e, ok := msg.(*Error); ok {
		return fmt.Errorf("error registering to procedure '%v': %v", procedure, e.Error)
	} else if registered, ok := msg.(*Registered); !ok {
		return fmt.Errorf(formatUnexpectedMessage(msg, REGISTERED))
	} else {
		// register the event handler with this registration
		c.procedures[registered.Registration] = fn
	}
	return nil
}

func (c *Client) Publish(topic URI, args []interface{}, kwargs map[string]interface{}) error {
	return c.Send(&Publish{
		Request:     c.nextID(),
		Options:     make(map[string]interface{}),
		Topic:       topic,
		Arguments:   args,
		ArgumentsKw: kwargs,
	})
}

func (c *Client) Call(procedure URI, args []interface{}, kwargs map[string]interface{}) (Message, error) {
	id := c.nextID()
	c.registerListener(id)

	call := &Call{
		Request:     id,
		Procedure:   procedure,
		Options:     make(map[string]interface{}),
		Arguments:   args,
		ArgumentsKw: kwargs,
	}
	if err := c.Send(call); err != nil {
		return nil, err
	}

	// wait to receive RESULT message
	msg, err := c.waitOnListener(id)
	if err != nil {
		return nil, err
	} else if e, ok := msg.(*Error); ok {
		return nil, fmt.Errorf("error registering to procedure '%v': %v", procedure, e.Error)
	} else if _, ok := msg.(*Result); !ok {
		return nil, fmt.Errorf(formatUnexpectedMessage(msg, RESULT))
	} else {
		return msg, nil
	}
}
