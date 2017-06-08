package turnpike

import (
	"crypto/tls"
	"fmt"
	"sync"
	"time"
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
		Reason:  ErrCloseRealm,
	}
)

// A Client routes messages to/from a WAMP router.
type Client struct {
	Peer
	// ReceiveTimeout is the amount of time that the client will block waiting for a response from the router.
	ReceiveTimeout time.Duration
	// Auth is a map of WAMP authmethods to functions that will handle each auth type
	Auth map[string]AuthFunc
	// ReceiveDone is notified when the client's connection to the router is lost.
	ReceiveDone  chan bool
	listeners    map[ID]chan Message
	events       map[ID]*eventDesc
	procedures   map[ID]*procedureDesc
	requestCount uint

	lock sync.RWMutex
}

type procedureDesc struct {
	name    string
	handler MethodHandler
}

type eventDesc struct {
	topic   string
	handler EventHandler
}

// NewWebsocketClient creates a new websocket client connected to the specified
// `url` and using the specified `serialization`.
func NewWebsocketClient(serialization Serialization, url string, tlscfg *tls.Config) (*Client, error) {
	p, err := NewWebsocketPeer(serialization, url, "", tlscfg)
	if err != nil {
		return nil, err
	}
	return NewClient(p), nil
}

// NewClient takes a connected Peer and returns a new Client
func NewClient(p Peer) *Client {
	c := &Client{
		Peer:           p,
		ReceiveTimeout: 30 * time.Second,
		listeners:      make(map[ID]chan Message),
		events:         make(map[ID]*eventDesc),
		procedures:     make(map[ID]*procedureDesc),
		requestCount:   0,
	}
	return c
}

// JoinRealm joins a WAMP realm, but does not handle challenge/response authentication.
func (c *Client) JoinRealm(realm string, details map[string]interface{}) (map[string]interface{}, error) {
	if details == nil {
		details = map[string]interface{}{}
	}
	details["roles"] = clientRoles()
	if c.Auth != nil && len(c.Auth) > 0 {
		return c.joinRealmCRA(realm, details)
	}
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

// joinRealmCRA joins a WAMP realm and handles challenge/response authentication.
func (c *Client) joinRealmCRA(realm string, details map[string]interface{}) (map[string]interface{}, error) {
	authmethods := []interface{}{}
	for m := range c.Auth {
		authmethods = append(authmethods, m)
	}
	details["authmethods"] = authmethods
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
	} else if authFunc, ok := c.Auth[challenge.AuthMethod]; !ok {
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

func clientRoles() map[string]map[string]interface{} {
	return map[string]map[string]interface{}{
		"publisher":  make(map[string]interface{}),
		"subscriber": make(map[string]interface{}),
		"callee":     make(map[string]interface{}),
		"caller":     make(map[string]interface{}),
	}
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
			c.lock.RLock()
			if event, ok := c.events[msg.Subscription]; ok {
				go event.handler(msg.Arguments, msg.ArgumentsKw)
			} else {
				log.Println("no handler registered for subscription:", msg.Subscription)
			}
			c.lock.RUnlock()

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

	if c.ReceiveDone != nil {
		c.ReceiveDone <- true
	}
}

func (c *Client) notifyListener(msg Message, requestID ID) {
	// pass in the request ID so we don't have to do any type assertion
	c.lock.RLock()
	defer c.lock.RUnlock()
	if l, ok := c.listeners[requestID]; ok {
		l <- msg
	} else {
		log.Println("no listener for message", msg.MessageType(), requestID)
	}
}

func (c *Client) handleInvocation(msg *Invocation) {
	c.lock.RLock()
	if proc, ok := c.procedures[msg.Registration]; ok {
		c.lock.RUnlock()
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
		c.lock.RUnlock()
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
	c.lock.Lock()
	defer c.lock.Unlock()
	c.listeners[id] = wait
}

func (c *Client) unregisterListener(id ID) {
	c.lock.Lock()
	defer c.lock.Unlock()

	log.Println("unregister listener:", id)
	delete(c.listeners, id)
}

func (c *Client) waitOnListener(id ID) (msg Message, err error) {
	log.Println("wait on listener:", id)
	c.lock.RLock()
	wait, ok := c.listeners[id]
	c.lock.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown listener ID: %v", id)
	}
	select {
	case msg = <-wait:
		return
	case <-time.After(c.ReceiveTimeout):
		err = fmt.Errorf("timeout while waiting for message")
		return
	}
}

// EventHandler handles a publish event.
type EventHandler func(args []interface{}, kwargs map[string]interface{})

// Subscribe registers the EventHandler to be called for every message in the provided topic.
func (c *Client) Subscribe(topic string, fn EventHandler) error {
	id := NewID()
	c.registerListener(id)
	// TODO: figure out where to clean this up
	// defer c.unregisterListener(id)

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
		c.lock.Lock()
		defer c.lock.Unlock()
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
	c.lock.RLock()
	for id, desc := range c.events {
		if desc.topic == topic {
			subscriptionID = id
			found = true
		}
	}
	c.lock.RUnlock()
	if !found {
		return fmt.Errorf("Event %s is not registered with this client.", topic)
	}

	id := NewID()
	c.registerListener(id)
	defer c.unregisterListener(id)

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

	c.lock.Lock()
	defer c.lock.Unlock()
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
	// TODO: figure out where to clean this up
	// defer c.unregisterListener(id)

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
		c.lock.Lock()
		defer c.lock.Unlock()
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
	c.lock.RLock()
	for id, p := range c.procedures {
		if p.name == procedure {
			procedureID = id
			found = true
		}
	}
	c.lock.RUnlock()
	if !found {
		return fmt.Errorf("Procedure %s is not registered with this client.", procedure)
	}
	id := NewID()
	c.registerListener(id)
	defer c.unregisterListener(id)

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
	c.lock.Lock()
	defer c.lock.Unlock()
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

type RPCError struct {
	ErrorMessage *Error
	Procedure    string
}

func (rpc RPCError) Error() string {
	return fmt.Sprintf("error calling procedure '%v': %v: %v: %v", rpc.Procedure, rpc.ErrorMessage.Error, rpc.ErrorMessage.Arguments, rpc.ErrorMessage.ArgumentsKw)
}

// Call calls a procedure given a URI.
func (c *Client) Call(procedure string, args []interface{}, kwargs map[string]interface{}) (*Result, error) {
	id := NewID()
	c.registerListener(id)
	defer c.unregisterListener(id)

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
		return nil, RPCError{e, procedure}
	} else if result, ok := msg.(*Result); !ok {
		return nil, fmt.Errorf(formatUnexpectedMessage(msg, RESULT))
	} else {
		return result, nil
	}
}
