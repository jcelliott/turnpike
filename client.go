package turnpike

import (
	"fmt"
	"github.com/mitchellh/mapstructure"
	"reflect"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
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
	serviceMap   map[string]*service
	requestCount uint
}

type service struct {
	name       string
	procedures map[string]reflect.Method
	topics     map[string]reflect.Method
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

	if c.ReceiveDone != nil {
		c.ReceiveDone <- true
	}
}

func (c *Client) notifyListener(msg Message, requestID ID) {
	// pass in the request ID so we don't have to do any type assertion
	if l, ok := c.listeners[requestID]; ok {
		l <- msg
	} else {
		log.Println("no listener for message", msg.MessageType(), requestID)
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
	wait, ok := c.listeners[id]
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

// RegisterService registers in the dealer the set of methods of the
// receiver value that satisfy the following conditions:
//	- exported method of exported type
//	- at least one return value, of type error
// It returns an error if the receiver is not an exported type or has
// no suitable methods.
// The client accesses each method using a string of the form "type.method",
// where type is the receiver's concrete type.
func (c *Client) RegisterService(rcvr interface{}) error {
	return c.registerService(rcvr, "", false)
}

// RegisterServiceName is like RegisterService but uses the provided name for the type
// instead of the receiver's concrete type.
func (c *Client) RegisterServiceName(name string, rcvr interface{}) error {
	return c.registerService(rcvr, name, true)
}

var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

// Is this an exported - upper case - name?
func isExported(name string) bool {
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune)
}

func (c *Client) registerService(rcvr interface{}, name string, useName bool) error {
	if c.serviceMap == nil {
		c.serviceMap = make(map[string]*service)
	}
	typ := reflect.TypeOf(rcvr)
	val := reflect.ValueOf(rcvr)
	sname := reflect.Indirect(val).Type().Name()
	if name != "" {
		sname = name
	}
	if !isExported(sname) && !useName {
		return fmt.Errorf("type %s is not exported", sname)
	}
	if _, present := c.serviceMap[sname]; present {
		return fmt.Errorf("service %s is already defined", sname)
	}

	procedures := make(map[string]reflect.Method)
	topics := make(map[string]reflect.Method)
	for m := 0; m < typ.NumMethod(); m++ {
		method := typ.Method(m)
		mtype := method.Type
		mname := method.Name
		// Method must be exported.
		if method.PkgPath != "" {
			continue
		}

		// Method needs at least one out to be a procedure
		numOut := mtype.NumOut()
		if numOut == 0 {
			// A method with zero outs must start with On to be a event listener
			if strings.HasPrefix(mname, "On") {
				topics[mname] = method
			}
			continue
		}

		// Method last out must be error
		if errType := mtype.Out(numOut - 1); errType != typeOfError {
			continue
		}

		procedures[mname] = method
	}

	// Register methods as procedures with Dealer
	for mname, value := range procedures {
		procedure := value
		namespace := sname + "." + mname
		f := func(args []interface{}, kwargs map[string]interface{}, details map[string]interface{}) (callResult *CallResult) {
			methodArgs := procedure.Type.NumIn() - 1
			if methodArgs != len(args) {
				err := fmt.Errorf("method %s has %d inputs, was called with %d arguments as %s procedure ", procedure.Name, methodArgs, len(args), namespace)
				return &CallResult{
					Args: []interface{}{err.Error()},
					Err:  ErrInvalidArgument,
				}
			}
			values := make([]reflect.Value, len(args))
			var err error
			for i, arg := range args {
				in := procedure.Type.In(i + 1)
				values[i], err = decodeArgument(in, arg)
				if err != nil {
					return &CallResult{
						Args: []interface{}{err.Error()},
						Err:  ErrInvalidArgument,
					}
				}
			}
			values = append([]reflect.Value{val}, values...)
			function := procedure.Func
			returnValues := function.Call(values)

			result := make([]interface{}, len(returnValues))
			for i := range returnValues {
				result[i] = returnValues[i].Interface()
			}
			return &CallResult{
				Args: result,
			}
		}
		if err := c.Register(namespace, f, map[string]interface{}{}); err != nil {
			return err
		}
	}

	// Subscribe methods to topics with the broker
	for mname, value := range topics {
		topic := value
		namespace := sname + "." + mname
		f := func(args []interface{}, kwargs map[string]interface{}) {
			methodArgs := topic.Type.NumIn() - 1
			if methodArgs != len(args) {
				log.Printf("event %s has %d inputs, was published with %d arguments", namespace, methodArgs, len(args))
				return
			}
			values := make([]reflect.Value, len(args))
			var err error
			for i, arg := range args {
				in := topic.Type.In(i + 1)
				values[i], err = decodeArgument(in, arg)
				if err != nil {
					log.Println(err)
					return
				}
			}
			values = append([]reflect.Value{val}, values...)
			topic.Func.Call(values)
		}
		if err := c.Subscribe(namespace, f); err != nil {
			return err
		}
	}
	c.serviceMap[sname] = &service{
		procedures: procedures,
		topics:     topics,
		name:       sname,
	}
	return nil
}

func decodeArgument(target reflect.Type, arg interface{}) (reflect.Value, error) {
	if arg == nil {
		return reflect.Zero(target), nil
	}
	switch target.Kind() {
	case reflect.Ptr:
		return decodeArgument(target.Elem(), arg)
	case reflect.Struct:
		val := reflect.New(target)
		err := mapstructure.Decode(arg, val.Interface())
		return val, err
	case reflect.Slice:
		varg := reflect.ValueOf(arg)
		val := reflect.MakeSlice(target, varg.Len(), varg.Len())
		for i := 0; i < varg.Len(); i++ {
			v := varg.Index(i)
			e := val.Index(i)
			kind := target.Elem()
			vargEle, err := decodeArgument(kind, v.Interface())
			if err != nil {
				return val, err
			}
			e.Set(vargEle)
		}
		return val, nil
	// Special case for ints, numbers are decoded as float64
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		targ := reflect.TypeOf(arg)
		if targ.Kind() == reflect.Float64 {
			return reflect.ValueOf(arg).Convert(target), nil
		}
	}
	return reflect.ValueOf(arg), nil
}

// UnregisterService unsubscribes the service event listeners and unregisters the service procedures.
func (c *Client) UnregisterService(name string) error {
	service, present := c.serviceMap[name]
	if !present {
		return fmt.Errorf("service %s is not defined", name)
	}
	errs := newErrCol()
	for procedureName := range service.procedures {
		err := c.Unregister(service.name + "." + procedureName)
		errs.IfErrAppend(err)
	}
	for topicName := range service.topics {
		err := c.Unsubscribe(service.name + "." + topicName)
		errs.IfErrAppend(err)
	}
	c.serviceMap[name] = nil
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// CallService invokes the service method 'namespace' with arguments 'args'.
// The return values are saved in 'replies'.
func (c *Client) CallService(namespace string, args []interface{}, replies ...interface{}) error {
	for i := 0; i < len(replies); i++ {
		if replies[i] == nil {
			continue
		}
		kind := reflect.ValueOf(replies[i]).Kind()
		if kind != reflect.Ptr {
			return fmt.Errorf("reply[%d] type %s is not a pointer", i, kind)
		}
	}

	res, err := c.Call(namespace, args, nil)
	if err != nil {
		return err
	}
	returnValues := res.Arguments
	if len(returnValues)-1 < len(replies) {
		return fmt.Errorf("expected %d return values, got %d", len(returnValues)-1, len(replies))
	}
	if len(returnValues) == 0 {
		return fmt.Errorf("expected at least one return value of type string, nil or error")
	}

	for i := 0; i < len(replies); i++ {
		if replies[i] == nil {
			continue
		}
		vReplies := reflect.ValueOf(replies[i])
		tReplies := vReplies.Type()
		val, err := decodeArgument(tReplies, returnValues[i])
		if err != nil {
			return err
		}
		vReplies.Elem().Set(val)
	}
	switch e := returnValues[len(returnValues)-1].(type) {
	case string:
		return fmt.Errorf("%s", e)
	case nil:
		return nil
	case error:
		return e
	default:
		return fmt.Errorf("expected the last return value to be of type string, nil or error got %T", e)
	}
}
