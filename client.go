package turnpike

import (
	"crypto/tls"
	"fmt"
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
	acts         chan func()
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
		ReceiveTimeout: 10 * time.Second,
		listeners:      make(map[ID]chan Message),
		events:         make(map[ID]*eventDesc),
		procedures:     make(map[ID]*procedureDesc),
		acts:           make(chan func()),
		requestCount:   0,
	}
	go c.run()
	return c
}

func (c *Client) run() {
	for {
		if act, ok := <-c.acts; ok {
			act()
		} else {
			return
		}
	}
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
		close(c.acts)
		return nil, err
	}
	if msg, err := GetMessageTimeout(c.Peer, c.ReceiveTimeout); err != nil {
		c.Peer.Close()
		close(c.acts)
		return nil, err
	} else if welcome, ok := msg.(*Welcome); !ok {
		c.Send(abortUnexpectedMsg)
		c.Peer.Close()
		close(c.acts)
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
		close(c.acts)
		return nil, err
	}
	if msg, err := GetMessageTimeout(c.Peer, c.ReceiveTimeout); err != nil {
		c.Peer.Close()
		close(c.acts)
		return nil, err
	} else if challenge, ok := msg.(*Challenge); !ok {
		c.Send(abortUnexpectedMsg)
		c.Peer.Close()
		close(c.acts)
		return nil, fmt.Errorf(formatUnexpectedMessage(msg, CHALLENGE))
	} else if authFunc, ok := c.Auth[challenge.AuthMethod]; !ok {
		c.Send(abortNoAuthHandler)
		c.Peer.Close()
		close(c.acts)
		return nil, fmt.Errorf("no auth handler for method: %s", challenge.AuthMethod)
	} else if signature, authDetails, err := authFunc(details, challenge.Extra); err != nil {
		c.Send(abortAuthFailure)
		c.Peer.Close()
		close(c.acts)
		return nil, err
	} else if err := c.Send(&Authenticate{Signature: signature, Extra: authDetails}); err != nil {
		c.Peer.Close()
		close(c.acts)
		return nil, err
	}
	if msg, err := GetMessageTimeout(c.Peer, c.ReceiveTimeout); err != nil {
		c.Peer.Close()
		close(c.acts)
		return nil, err
	} else if welcome, ok := msg.(*Welcome); !ok {
		c.Send(abortUnexpectedMsg)
		c.Peer.Close()
		close(c.acts)
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
			c.handleEvent(msg)

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

	close(c.acts)
	log.Println("client closed")

	if c.ReceiveDone != nil {
		c.ReceiveDone <- true
	}
}

func (c *Client) handleEvent(msg *Event) {
	sync := make(chan struct{})
	c.acts <- func() {
		if event, ok := c.events[msg.Subscription]; ok {
			go event.handler(msg.Arguments, msg.ArgumentsKw)
		} else {
			log.Println("no handler registered for subscription:", msg.Subscription)
		}
		sync <- struct{}{}
	}
	<-sync
}

func (c *Client) notifyListener(msg Message, requestID ID) {
	// pass in the request ID so we don't have to do any type assertion
	var (
		sync = make(chan struct{})
		l    chan Message
		ok   bool
	)
	c.acts <- func() {
		l, ok = c.listeners[requestID]
		sync <- struct{}{}
	}
	<-sync
	if ok {
		l <- msg
	} else {
		log.Println("no listener for message", msg.MessageType(), requestID)
	}
}

func (c *Client) handleInvocation(msg *Invocation) {
	sync := make(chan struct{})
	c.acts <- func() {
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
		sync <- struct{}{}
	}
	<-sync
}

func (c *Client) registerListener(id ID) {
	log.Println("register listener:", id)
	wait := make(chan Message, 1)
	sync := make(chan struct{})
	c.acts <- func() {
		c.listeners[id] = wait
		sync <- struct{}{}
	}
	<-sync
}

func (c *Client) waitOnListener(id ID) (msg Message, err error) {
	log.Println("wait on listener:", id)
	var (
		sync = make(chan struct{})
		wait chan Message
		ok   bool
	)
	c.acts <- func() {
		wait, ok = c.listeners[id]
		sync <- struct{}{}
	}
	<-sync
	if !ok {
		return nil, fmt.Errorf("unknown listener ID: %v", id)
	}
	select {
	case msg = <-wait:
	case <-time.After(c.ReceiveTimeout):
		err = fmt.Errorf("timeout while waiting for message")
	}
	c.acts <- func() {
		delete(c.listeners, id)
	}
	return
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
	err := c.Send(sub)
	if err != nil {
		return err
	}
	// wait to receive SUBSCRIBED message
	var msg Message
	if msg, err = c.waitOnListener(id); err != nil {
		return err
	} else if e, ok := msg.(*Error); ok {
		return fmt.Errorf("error subscribing to topic '%v': %v", topic, e.Error)
	} else if subscribed, ok := msg.(*Subscribed); !ok {
		return fmt.Errorf(formatUnexpectedMessage(msg, SUBSCRIBED))
	} else {
		// register the event handler with this subscription
		sync := make(chan struct{})
		c.acts <- func() {
			c.events[subscribed.Subscription] = &eventDesc{topic, fn}
			sync <- struct{}{}
		}
		<-sync
	}
	return nil
}

// Unsubscribe removes the registered EventHandler from the topic.
func (c *Client) Unsubscribe(topic string) error {
	var (
		sync           = make(chan struct{})
		subscriptionID ID
		found          bool
		msg            Message
		err            error
	)
	c.acts <- func() {
		for id, desc := range c.events {
			if desc.topic == topic {
				subscriptionID = id
				found = true
				break
			}
		}
		sync <- struct{}{}
	}
	<-sync
	if !found {
		return fmt.Errorf("Event %s is not registered with this client.", topic)
	}

	id := NewID()
	c.registerListener(id)
	sub := &Unsubscribe{
		Request:      id,
		Subscription: subscriptionID,
	}
	err = c.Send(sub)
	if err != nil {
		return err
	}
	// wait to receive UNSUBSCRIBED message
	if msg, err = c.waitOnListener(id); err != nil {
		return err
	} else if e, ok := msg.(*Error); ok {
		return fmt.Errorf("error unsubscribing to topic '%v': %v", topic, e.Error)
	} else if _, ok := msg.(*Unsubscribed); !ok {
		return fmt.Errorf(formatUnexpectedMessage(msg, UNSUBSCRIBED))
	}
	c.acts <- func() {
		delete(c.events, subscriptionID)
		sync <- struct{}{}
	}
	<-sync
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
	err := c.Send(register)
	if err != nil {
		return err
	}

	// wait to receive REGISTERED message
	var msg Message
	if msg, err = c.waitOnListener(id); err != nil {
		return err
	} else if e, ok := msg.(*Error); ok {
		return fmt.Errorf("error registering procedure '%v': %v", procedure, e.Error)
	} else if registered, ok := msg.(*Registered); !ok {
		return fmt.Errorf(formatUnexpectedMessage(msg, REGISTERED))
	} else {
		// register the event handler with this registration
		sync := make(chan struct{})
		c.acts <- func() {
			c.procedures[registered.Registration] = &procedureDesc{procedure, fn}
			sync <- struct{}{}
		}
		<-sync
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
		sync        = make(chan struct{})
		procedureID ID
		found       bool
		msg         Message
		err         error
	)
	c.acts <- func() {
		for id, p := range c.procedures {
			if p.name == procedure {
				procedureID = id
				found = true
				break
			}
		}
		sync <- struct{}{}
	}
	<-sync
	if !found {
		return fmt.Errorf("Procedure %s is not registered with this client.", procedure)
	}
	id := NewID()
	c.registerListener(id)
	unregister := &Unregister{
		Request:      id,
		Registration: procedureID,
	}
	if err = c.Send(unregister); err != nil {
		return err
	}

	// wait to receive UNREGISTERED message
	if msg, err = c.waitOnListener(id); err != nil {
		return err
	} else if e, ok := msg.(*Error); ok {
		return fmt.Errorf("error unregister to procedure '%v': %v", procedure, e.Error)
	} else if _, ok := msg.(*Unregistered); !ok {
		return fmt.Errorf(formatUnexpectedMessage(msg, UNREGISTERED))
	}
	// register the event handler with this unregistration
	c.acts <- func() {
		delete(c.procedures, procedureID)
		sync <- struct{}{}
	}
	<-sync
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
	err := c.Send(call)
	if err != nil {
		return nil, err
	}

	// wait to receive RESULT message
	var msg Message
	if msg, err = c.waitOnListener(id); err != nil {
		return nil, err
	} else if e, ok := msg.(*Error); ok {
		return nil, fmt.Errorf("error calling procedure '%v': %v", procedure, e.Error)
	} else if result, ok := msg.(*Result); !ok {
		return nil, fmt.Errorf(formatUnexpectedMessage(msg, RESULT))
	} else {
		return result, nil
	}
}
