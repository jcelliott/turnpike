package turnpike

// Message is a generic container for a WAMP message.
type Message interface {
	MessageType() MessageType
}

type MessageType int

func (mt MessageType) New() Message {
	switch mt {
	case HELLO:
		return new(Hello)
	case WELCOME:
		return new(Welcome)
	case ABORT:
		return new(Abort)
	case CHALLENGE:
		return new(Challenge)
	case AUTHENTICATE:
		return new(Authenticate)
	case GOODBYE:
		return new(Goodbye)
	case HEARTBEAT:
		return new(Heartbeat)
	case ERROR:
		return new(Error)

	case PUBLISH:
		return new(Publish)
	case PUBLISHED:
		return new(Published)

	case SUBSCRIBE:
		return new(Subscribe)
	case SUBSCRIBED:
		return new(Subscribed)
	case UNSUBSCRIBE:
		return new(Unsubscribe)
	case UNSUBSCRIBED:
		return new(Unsubscribed)
	case EVENT:
		return new(Event)

	case CALL:
		return new(Call)
	case CANCEL:
		return new(Cancel)
	case RESULT:
		return new(Result)

	case REGISTER:
		return new(Register)
	case REGISTERED:
		return new(Registered)
	case UNREGISTER:
		return new(Unregister)
	case UNREGISTERED:
		return new(Unregistered)
	case INVOCATION:
		return new(Invocation)
	case INTERRUPT:
		return new(Interrupt)
	case YIELD:
		return new(Yield)
	default:
		// TODO: allow custom message types?
		return nil
	}
}

func (mt MessageType) String() string {
	switch mt {
	case HELLO:
		return "HELLO"
	case WELCOME:
		return "WELCOME"
	case ABORT:
		return "ABORT"
	case CHALLENGE:
		return "CHALLENGE"
	case AUTHENTICATE:
		return "AUTHENTICATE"
	case GOODBYE:
		return "GOODBYE"
	case HEARTBEAT:
		return "HEARTBEAT"
	case ERROR:
		return "ERROR"

	case PUBLISH:
		return "PUBLISH"
	case PUBLISHED:
		return "PUBLISHED"

	case SUBSCRIBE:
		return "SUBSCRIBE"
	case SUBSCRIBED:
		return "SUBSCRIBED"
	case UNSUBSCRIBE:
		return "UNSUBSCRIBE"
	case UNSUBSCRIBED:
		return "UNSUBSCRIBED"
	case EVENT:
		return "EVENT"

	case CALL:
		return "CALL"
	case CANCEL:
		return "CANCEL"
	case RESULT:
		return "RESULT"

	case REGISTER:
		return "REGISTER"
	case REGISTERED:
		return "REGISTERED"
	case UNREGISTER:
		return "UNREGISTER"
	case UNREGISTERED:
		return "UNREGISTERED"
	case INVOCATION:
		return "INVOCATION"
	case INTERRUPT:
		return "INTERRUPT"
	case YIELD:
		return "YIELD"
	default:
		// TODO: allow custom message types?
		panic("Invalid message type")
	}
}

const (
	HELLO        MessageType = 1
	WELCOME      MessageType = 2
	ABORT        MessageType = 3
	CHALLENGE    MessageType = 4
	AUTHENTICATE MessageType = 5
	GOODBYE      MessageType = 6
	HEARTBEAT    MessageType = 7
	ERROR        MessageType = 8

	PUBLISH   MessageType = 16 //	Tx 	Rx
	PUBLISHED MessageType = 17 //	Rx 	Tx

	SUBSCRIBE    MessageType = 32 //	Rx 	Tx
	SUBSCRIBED   MessageType = 33 //	Tx 	Rx
	UNSUBSCRIBE  MessageType = 34 //	Rx 	Tx
	UNSUBSCRIBED MessageType = 35 //	Tx 	Rx
	EVENT        MessageType = 36 //	Tx 	Rx

	CALL   MessageType = 48 //	Tx 	Rx
	CANCEL MessageType = 49 //	Tx 	Rx
	RESULT MessageType = 50 //	Rx 	Tx

	REGISTER     MessageType = 64 //	Rx 	Tx
	REGISTERED   MessageType = 65 //	Tx 	Rx
	UNREGISTER   MessageType = 66 //	Rx 	Tx
	UNREGISTERED MessageType = 67 //	Tx 	Rx
	INVOCATION   MessageType = 68 //	Tx 	Rx
	INTERRUPT    MessageType = 69 //	Tx 	Rx
	YIELD        MessageType = 70 //	Rx 	Tx
)

// URIs are dot-separated identifiers, where each component *should* only contain letters, numbers or underscores.
//
// See the documentation for specifics: https://github.com/wamp-proto/wamp-proto/blob/master/rfc/text/basic/bp_identifiers.md#uris-uris
type URI string

// An ID is a unique, non-negative number. Different uses may have additional restrictions.
type ID uint64

// [HELLO, Realm|uri, Details|dict]
type Hello struct {
	Realm   URI
	Details map[string]interface{}
}

func (msg *Hello) MessageType() MessageType {
	return HELLO
}

// [WELCOME, Session|id, Details|dict]
type Welcome struct {
	Id      ID
	Details map[string]interface{}
}

func (msg *Welcome) MessageType() MessageType {
	return WELCOME
}

// [ABORT, Details|dict, Reason|uri]
type Abort struct {
	Details map[string]interface{}
	Reason  URI
}

func (msg *Abort) MessageType() MessageType {
	return ABORT
}

// [CHALLENGE, AuthMethod|string, Extra|dict]
type Challenge struct {
	AuthMethod string
	Extra      map[string]interface{}
}

func (msg *Challenge) MessageType() MessageType {
	return CHALLENGE
}

// [AUTHENTICATE, Signature|string, Extra|dict]
type Authenticate struct {
	Signature string
	Extra     map[string]interface{}
}

func (msg *Authenticate) MessageType() MessageType {
	return AUTHENTICATE
}

// [GOODBYE, Details|dict, Reason|uri]
type Goodbye struct {
	Details map[string]interface{}
	Reason  URI
}

func (msg *Goodbye) MessageType() MessageType {
	return GOODBYE
}

// [HEARTBEAT, IncomingSeq|integer, OutgoingSeq|integer
// [HEARTBEAT, IncomingSeq|integer, OutgoingSeq|integer, Discard|string]
type Heartbeat struct {
	IncomingSeq uint
	OutgoingSeq uint
	Discard     string
}

func (msg *Heartbeat) MessageType() MessageType {
	return HEARTBEAT
}

// [ERROR, REQUEST.Type|int, REQUEST.Request|id, Details|dict, Error|uri]
// [ERROR, REQUEST.Type|int, REQUEST.Request|id, Details|dict, Error|uri, Arguments|list]
// [ERROR, REQUEST.Type|int, REQUEST.Request|id, Details|dict, Error|uri, Arguments|list, ArgumentsKw|dict]
type Error struct {
	Type        MessageType
	Request     ID
	Details     map[string]interface{}
	Error       URI
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
}

func (msg *Error) MessageType() MessageType {
	return ERROR
}

// [PUBLISH, Request|id, Options|dict, Topic|uri]
// [PUBLISH, Request|id, Options|dict, Topic|uri, Arguments|list]
// [PUBLISH, Request|id, Options|dict, Topic|uri, Arguments|list, ArgumentsKw|dict]
type Publish struct {
	Request     ID
	Options     map[string]interface{}
	Topic       URI
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
}

func (msg *Publish) MessageType() MessageType {
	return PUBLISH
}

// [PUBLISHED, PUBLISH.Request|id, Publication|id]
type Published struct {
	Request     ID
	Publication ID
}

func (msg *Published) MessageType() MessageType {
	return PUBLISHED
}

// [SUBSCRIBE, Request|id, Options|dict, Topic|uri]
type Subscribe struct {
	Request ID
	Options map[string]interface{}
	Topic   URI
}

func (msg *Subscribe) MessageType() MessageType {
	return SUBSCRIBE
}

// [SUBSCRIBED, SUBSCRIBE.Request|id, Subscription|id]
type Subscribed struct {
	Request      ID
	Subscription ID
}

func (msg *Subscribed) MessageType() MessageType {
	return SUBSCRIBED
}

// [UNSUBSCRIBE, Request|id, SUBSCRIBED.Subscription|id]
type Unsubscribe struct {
	Request      ID
	Subscription ID
}

func (msg *Unsubscribe) MessageType() MessageType {
	return UNSUBSCRIBE
}

// [UNSUBSCRIBED, UNSUBSCRIBE.Request|id]
type Unsubscribed struct {
	Request ID
}

func (msg *Unsubscribed) MessageType() MessageType {
	return UNSUBSCRIBED
}

// [EVENT, SUBSCRIBED.Subscription|id, PUBLISHED.Publication|id, Details|dict]
// [EVENT, SUBSCRIBED.Subscription|id, PUBLISHED.Publication|id, Details|dict, PUBLISH.Arguments|list]
// [EVENT, SUBSCRIBED.Subscription|id, PUBLISHED.Publication|id, Details|dict, PUBLISH.Arguments|list,
//     PUBLISH.ArgumentsKw|dict]
type Event struct {
	Subscription ID
	Publication  ID
	Details      map[string]interface{}
	Arguments    []interface{}          `wamp:"omitempty"`
	ArgumentsKw  map[string]interface{} `wamp:"omitempty"`
}

func (msg *Event) MessageType() MessageType {
	return EVENT
}

// CallResult represents the result of a CALL.
type CallResult struct {
	Args   []interface{}
	Kwargs map[string]interface{}
	Err    URI
}

// [CALL, Request|id, Options|dict, Procedure|uri]
// [CALL, Request|id, Options|dict, Procedure|uri, Arguments|list]
// [CALL, Request|id, Options|dict, Procedure|uri, Arguments|list, ArgumentsKw|dict]
type Call struct {
	Request     ID
	Options     map[string]interface{}
	Procedure   URI
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
}

func (msg *Call) MessageType() MessageType {
	return CALL
}

// [RESULT, CALL.Request|id, Details|dict]
// [RESULT, CALL.Request|id, Details|dict, YIELD.Arguments|list]
// [RESULT, CALL.Request|id, Details|dict, YIELD.Arguments|list, YIELD.ArgumentsKw|dict]
type Result struct {
	Request     ID
	Details     map[string]interface{}
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
}

func (msg *Result) MessageType() MessageType {
	return RESULT
}

// [REGISTER, Request|id, Options|dict, Procedure|uri]
type Register struct {
	Request   ID
	Options   map[string]interface{}
	Procedure URI
}

func (msg *Register) MessageType() MessageType {
	return REGISTER
}

// [REGISTERED, REGISTER.Request|id, Registration|id]
type Registered struct {
	Request      ID
	Registration ID
}

func (msg *Registered) MessageType() MessageType {
	return REGISTERED
}

// [UNREGISTER, Request|id, REGISTERED.Registration|id]
type Unregister struct {
	Request      ID
	Registration ID
}

func (msg *Unregister) MessageType() MessageType {
	return UNREGISTER
}

// [UNREGISTERED, UNREGISTER.Request|id]
type Unregistered struct {
	Request ID
}

func (msg *Unregistered) MessageType() MessageType {
	return UNREGISTERED
}

// [INVOCATION, Request|id, REGISTERED.Registration|id, Details|dict]
// [INVOCATION, Request|id, REGISTERED.Registration|id, Details|dict, CALL.Arguments|list]
// [INVOCATION, Request|id, REGISTERED.Registration|id, Details|dict, CALL.Arguments|list, CALL.ArgumentsKw|dict]
type Invocation struct {
	Request      ID
	Registration ID
	Details      map[string]interface{}
	Arguments    []interface{}          `wamp:"omitempty"`
	ArgumentsKw  map[string]interface{} `wamp:"omitempty"`
}

func (msg *Invocation) MessageType() MessageType {
	return INVOCATION
}

// [YIELD, INVOCATION.Request|id, Options|dict]
// [YIELD, INVOCATION.Request|id, Options|dict, Arguments|list]
// [YIELD, INVOCATION.Request|id, Options|dict, Arguments|list, ArgumentsKw|dict]
type Yield struct {
	Request     ID
	Options     map[string]interface{}
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
}

func (msg *Yield) MessageType() MessageType {
	return YIELD
}

// [CANCEL, CALL.Request|id, Options|dict]
type Cancel struct {
	Request ID
	Options map[string]interface{}
}

func (msg *Cancel) MessageType() MessageType {
	return CANCEL
}

// [INTERRUPT, INVOCATION.Request|id, Options|dict]
type Interrupt struct {
	Request ID
	Options map[string]interface{}
}

func (msg *Interrupt) MessageType() MessageType {
	return INTERRUPT
}
