package turnpike

import (
	"fmt"
	"time"
)

type Role int

const (
	// This client can publish events
	PUBLISHER Role = 1 << iota
	// This client can subscribe to events
	SUBSCRIBER
	// This client can register RPC functions
	CALLEE
	// This client can call RPC functions
	CALLER
	// This client can do all of the above
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

// A Client routes messages to/from a WAMP router.
type Client struct {
	Peer
	// ReceiveTimeout is the amount of time that the client will block waiting for a response from the router.
	ReceiveTimeout time.Duration
	// roles          int
	listeners    map[ID]chan Message
	events       map[ID]*eventDesc
	procedures   map[ID]*procedureDesc
	requestCount uint
}

type procedureDesc struct {
	name    string
	handler MethodHandler
}

type eventDesc struct {
	topic   string
	handler EventHandler
}

// Creates a new websocket client.
func NewWebsocketClient(serialization Serialization, url string) (*Client, error) {
	p, err := NewWebsocketPeer(serialization, url, "")
	if err != nil {
		return nil, err
	}
	return NewClient(p), nil
}

// NewClient takes a connected Peer and returns a new Client
func NewClient(p Peer) *Client {
	c := &Client{
		Peer:           p,
		ReceiveTimeout: 10 * time.Second,
		// roles:          roles,
		listeners:    make(map[ID]chan Message),
		events:       make(map[ID]*eventDesc),
		procedures:   make(map[ID]*procedureDesc),
		requestCount: 0,
	}
	return c
}

// JoinRealm joins a WAMP realm, but does not handle challenge/response authentication.
func (c *Client) JoinRealm(realm string, roles Role, details map[string]interface{}) (map[string]interface{}, error) {
	if details == nil {
		details = map[string]interface{}{}
	}
	details["roles"] = createRolesMap(roles)
	if err := c.Send(&Hello{Realm: URI(realm), Details: details}); err != nil {
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

// JoinRealmCRA joins a WAMP realm and handles challenge/response authentication.
func (c *Client) JoinRealmCRA(realm string, roles Role, details map[string]interface{}, auth map[string]AuthFunc) (map[string]interface{}, error) {
	if auth == nil || len(auth) == 0 {
		return nil, fmt.Errorf("no authentication methods provided")
	}
	if details == nil || len(details) == 0 {
		return nil, fmt.Errorf("no details map provided")
	}
	details["roles"] = createRolesMap(roles)
	if err := c.Send(&Hello{Realm: URI(realm), Details: details}); err != nil {
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

func createRolesMap(roles Role) map[string]interface{} {
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

// LeaveRealm leaves the current realm without closing the connection to the server.
func (c *Client) LeaveRealm() error {
	if err := c.Send(goodbyeClient); err != nil {
		return fmt.Errorf("error leaving realm: %v", err)
	}
	return nil
}

// Close closes the connection to the server.
func (c *Client) Close() error {
	if err := c.LeaveRealm(); err != nil {
		return err
	}
	if err := c.Peer.Close(); err != nil {
		return fmt.Errorf("error closing client connection: %v", err)
	}
	return nil
}

// func (c *Client) nextID() ID {
// 	c.requestCount++
// 	return ID(c.requestCount)
// }

// Receive handles messages from the server until this client disconnects.
//
// This function blocks and is most commonly run in a goroutine.
func (c *Client) Receive() {
	for msg := range c.Peer.Receive() {

		switch msg := msg.(type) {

		case *Event:
			if event, ok := c.events[msg.Subscription]; ok {
				go event.handler(msg.Arguments, msg.ArgumentsKw)
			} else {
				log.Println("no handler registered for subscription:", msg.Subscription)
			}

		case *Invocation:
			c.handleInvocation(msg)

		case *Registered:
			c.notifyListener(msg, msg.Request)

		case *Subscribed:
			c.notifyListener(msg, msg.Request)

		case *Unsubscribed:
			c.notifyListener(msg, msg.Request)

		case *Unregistered:
			c.notifyListener(msg, msg.Request)

		case *Result:
			c.notifyListener(msg, msg.Request)

		case *Error:
			c.notifyListener(msg, msg.Request)

		case *Goodbye:
			log.Println("client received Goodbye message")
			break

		default:
			log.Println("unhandled message:", msg.MessageType(), msg)
		}
	}
	log.Println("client closed")
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
	if proc, ok := c.procedures[msg.Registration]; ok {
		go func() {
			result := proc.handler(msg.Arguments, msg.ArgumentsKw, msg.Details)

			var tosend Message
			tosend = &Yield{
				Request:     msg.Request,
				Options:     make(map[string]interface{}),
				Arguments:   result.Args,
				ArgumentsKw: result.Kwargs,
			}

			if result.Err != "" {
				tosend = &Error{
					Type:        INVOCATION,
					Request:     msg.Request,
					Details:     make(map[string]interface{}),
					Arguments:   result.Args,
					ArgumentsKw: result.Kwargs,
					Error:       result.Err,
				}
			}

			if err := c.Send(tosend); err != nil {
				log.Println("error sending message:", err)
			}
		}()
	} else {
		log.Println("no handler registered for registration:", msg.Registration)
		if err := c.Send(&Error{
			Type:    INVOCATION,
			Request: msg.Request,
			Details: make(map[string]interface{}),
			Error:   URI(fmt.Sprintf("no handler for registration: %v", msg.Registration)),
		}); err != nil {
			log.Println("error sending message:", err)
		}
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

// EventHandler handles a publish event.
type EventHandler func(args []interface{}, kwargs map[string]interface{})

// Subscribe registers the EventHandler to be called for every message in the provided topic.
func (c *Client) Subscribe(topic string, fn EventHandler) error {
	id := NewID()
	c.registerListener(id)
	sub := &Subscribe{
		Request: id,
		Options: make(map[string]interface{}),
		Topic:   URI(topic),
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
		c.events[subscribed.Subscription] = &eventDesc{topic, fn}
	}
	return nil
}

// Unsubscribe removes the registered EventHandler from the topic.
func (c *Client) Unsubscribe(topic string) error {
	var (
		subscriptionID ID
		found          bool
	)
	for id, desc := range c.events {
		if desc.topic == topic {
			subscriptionID = id
			found = true
		}
	}
	if !found {
		return fmt.Errorf("Event %s is not registered with this client.", topic)
	}

	id := NewID()
	c.registerListener(id)
	sub := &Unsubscribe{
		Request:      id,
		Subscription: subscriptionID,
	}
	if err := c.Send(sub); err != nil {
		return err
	}
	// wait to receive UNSUBSCRIBED message
	msg, err := c.waitOnListener(id)
	if err != nil {
		return err
	} else if e, ok := msg.(*Error); ok {
		return fmt.Errorf("error unsubscribing to topic '%v': %v", topic, e.Error)
	} else if _, ok := msg.(*Unsubscribed); !ok {
		return fmt.Errorf(formatUnexpectedMessage(msg, UNSUBSCRIBED))
	}
	delete(c.events, subscriptionID)
	return nil
}

// MethodHandler is an RPC endpoint.
type MethodHandler func(
	args []interface{}, kwargs map[string]interface{}, details map[string]interface{},
) (result *CallResult)

// Register registers a MethodHandler procedure with the router.
func (c *Client) Register(procedure string, fn MethodHandler, options map[string]interface{}) error {
	id := NewID()
	c.registerListener(id)
	register := &Register{
		Request:   id,
		Options:   options,
		Procedure: URI(procedure),
	}
	if err := c.Send(register); err != nil {
		return err
	}

	// wait to receive REGISTERED message
	msg, err := c.waitOnListener(id)
	if err != nil {
		return err
	} else if e, ok := msg.(*Error); ok {
		return fmt.Errorf("error registering procedure '%v': %v", procedure, e.Error)
	} else if registered, ok := msg.(*Registered); !ok {
		return fmt.Errorf(formatUnexpectedMessage(msg, REGISTERED))
	} else {
		// register the event handler with this registration
		c.procedures[registered.Registration] = &procedureDesc{procedure, fn}
	}
	return nil
}

// BasicMethodHandler is an RPC endpoint that doesn't expect the `Details` map
type BasicMethodHandler func(args []interface{}, kwargs map[string]interface{}) (result *CallResult)

// BasicRegister registers a BasicMethodHandler procedure with the router
func (c *Client) BasicRegister(procedure string, fn BasicMethodHandler) error {
	wrap := func(args []interface{}, kwargs map[string]interface{},
		details map[string]interface{}) (result *CallResult) {
		return fn(args, kwargs)
	}
	return c.Register(procedure, wrap, make(map[string]interface{}))
}

// Unregister removes a procedure with the router
func (c *Client) Unregister(procedure string) error {
	var (
		procedureID ID
		found       bool
	)
	for id, p := range c.procedures {
		if p.name == procedure {
			procedureID = id
			found = true
		}
	}
	if !found {
		return fmt.Errorf("Procedure %s is not registered with this client.", procedure)
	}
	id := NewID()
	c.registerListener(id)
	unregister := &Unregister{
		Request:      id,
		Registration: procedureID,
	}
	if err := c.Send(unregister); err != nil {
		return err
	}

	// wait to receive UNREGISTERED message
	msg, err := c.waitOnListener(id)
	if err != nil {
		return err
	} else if e, ok := msg.(*Error); ok {
		return fmt.Errorf("error unregister to procedure '%v': %v", procedure, e.Error)
	} else if _, ok := msg.(*Unregistered); !ok {
		return fmt.Errorf(formatUnexpectedMessage(msg, UNREGISTERED))
	}
	// register the event handler with this unregistration
	delete(c.procedures, procedureID)
	return nil
}

// Publish publishes an EVENT to all subscribed peers.
func (c *Client) Publish(topic string, args []interface{}, kwargs map[string]interface{}) error {
	return c.Send(&Publish{
		Request:     NewID(),
		Options:     make(map[string]interface{}),
		Topic:       URI(topic),
		Arguments:   args,
		ArgumentsKw: kwargs,
	})
}

// Call calls a procedure given a URI.
func (c *Client) Call(procedure string, args []interface{}, kwargs map[string]interface{}) (*Result, error) {
	id := NewID()
	c.registerListener(id)

	call := &Call{
		Request:     id,
		Procedure:   URI(procedure),
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
		return nil, fmt.Errorf("error calling procedure '%v': %v", procedure, e.Error)
	} else if result, ok := msg.(*Result); !ok {
		return nil, fmt.Errorf(formatUnexpectedMessage(msg, RESULT))
	} else {
		return result, nil
	}
}
