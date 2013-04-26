// Package wamp implements the Websocket Application Messaging Protocol
package wamp

import (
	"encoding/json"
	"net/url"
)

const (
	TYPE_ID_WELCOME = iota
	TYPE_ID_PREFIX
	TYPE_ID_CALL
	TYPE_ID_CALLRESULT
	TYPE_ID_CALLERROR
	TYPE_ID_SUBSCRIBE
	TYPE_ID_UNSUBSCRIBE
	TYPE_ID_PUBLISH
	TYPE_ID_EVENT
)

const PROTOCOL_VERSION = 1

var (
	// this should be changed for another server using this WAMP protocol implementation
	ServerIdent = "turnpike-0.1.0"
)

// A WAMPError is returned when attempting to create a message that does not follow the WAMP
// protocol
type WAMPError struct {
	Msg string
}

var (
	ErrInvalidURI          = &WAMPError{"invalid URI"}
	ErrInvalidNumArgs      = &WAMPError{"invalid number of arguments in message"}
	ErrUnsupportedProtocol = &WAMPError{"unsupported protocol"}
)

func (e *WAMPError) Error() string {
	return "wamp: " + e.Msg
}

// TODO: function to determine what type of message it is

// WELCOME
type WelcomeMsg struct {
	SessionId       string
	ProtocolVersion int
	ServerIdent     string
}

func (msg *WelcomeMsg) UnmarshalJSON(jsonData []byte) error {
	var data []interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		return err
	}
	if len(data) != 4 {
		return ErrInvalidNumArgs
	}
	var ok bool
	if msg.SessionId, ok = data[1].(string); !ok {
		return &WAMPError{"invalid session ID"}
	}
	if protocolVersion, ok := data[2].(float64); ok {
		msg.ProtocolVersion = int(protocolVersion)
	} else {
		return ErrUnsupportedProtocol
	}
	if msg.ServerIdent, ok = data[3].(string); !ok {
		return &WAMPError{"invalid server identity"}
	}
	return nil
}

// Welcome returns a json encoded WAMP 'WELCOME' message as a byte slice
// sessionId a randomly generated string provided by the server
func Welcome(sessionId string) ([]byte, error) {
	return createWAMPMessage(TYPE_ID_WELCOME, sessionId, PROTOCOL_VERSION, ServerIdent)
}

// PREFIX
type PrefixMsg struct {
	Prefix string
	URI    string
}

func (msg *PrefixMsg) UnmarshalJSON(jsonData []byte) error {
	var data []interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		return err
	}
	if len(data) != 3 {
		return ErrInvalidNumArgs
	}
	var ok bool
	if msg.Prefix, ok = data[1].(string); !ok {
		return &WAMPError{"invalid prefix"}
	}
	if msg.URI, ok = data[2].(string); !ok {
		return &WAMPError{"invalid URI"}
	}
	return nil
}

// Prefix returns a json encoded WAMP 'PREFIX' message as a byte slice
func Prefix(prefix, URI string) ([]byte, error) {
	if _, err := url.ParseRequestURI(URI); err != nil {
		return nil, &WAMPError{"invalid URI: %s" + URI}
	}
	return createWAMPMessage(TYPE_ID_PREFIX, prefix, URI)
}

// CALL
type CallMsg struct {
	CallID   string
	ProcURI  string
	CallArgs []interface{}
}

func (msg *CallMsg) UnmarshalJSON(jsonData []byte) error {
	var data []interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		return err
	}
	if len(data) < 3 {
		return ErrInvalidNumArgs
	}
	var ok bool
	if msg.CallID, ok = data[1].(string); !ok {
		return &WAMPError{"invalid callID"}
	}
	if msg.ProcURI, ok = data[2].(string); !ok {
		return &WAMPError{"invalid procURI"}
	}
	if len(data) > 3 {
		msg.CallArgs = data[3:]
	}
	return nil
}

// Call returns a json encoded WAMP 'CALL' message as a byte slice
// callID must be a randomly generated string, procURI is the URI of the remote
// procedure to be called, followed by zero or more call arguments
func Call(callID, procURI string, args ...interface{}) ([]byte, error) {
	if _, err := url.ParseRequestURI(procURI); err != nil {
		return nil, &WAMPError{"invalid URI: %s" + procURI}
	}
	var data []interface{}
	data = append(data, TYPE_ID_CALL, callID, procURI)
	data = append(data, args...)
	return json.Marshal(data)
}

// CALLRESULT
type CallResultMsg struct {
	CallID string
	Result interface{}
}

func (msg *CallResultMsg) UnmarshalJSON(jsonData []byte) error {
	var data []interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		return err
	}
	if len(data) != 3 {
		return ErrInvalidNumArgs
	}
	var ok bool
	if msg.CallID, ok = data[1].(string); !ok {
		return &WAMPError{"invalid callID"}
	}
	msg.Result = data[2]

	return nil
}

// CallResult returns a json encoded WAMP 'CALLRESULT' message as a byte slice
// callID is the randomly generated string provided by the client
func CallResult(callID string, result interface{}) ([]byte, error) {
	return createWAMPMessage(TYPE_ID_CALLRESULT, callID, result)
}

// CALLERROR
type CallErrorMsg struct {
	CallID       string
	ErrorURI     string
	ErrorDesc    string
	ErrorDetails interface{}
}

func (msg *CallErrorMsg) UnmarshalJSON(jsonData []byte) error {
	var data []interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		return err
	}
	if len(data) < 4 || len(data) > 5 {
		return ErrInvalidNumArgs
	}
	var ok bool
	if msg.CallID, ok = data[1].(string); !ok {
		return &WAMPError{"invalid callID"}
	}
	if msg.ErrorURI, ok = data[2].(string); !ok {
		return &WAMPError{"invalid errorURI"}
	}
	if msg.ErrorDesc, ok = data[3].(string); !ok {
		return &WAMPError{"invalid error description"}
	}
	if len(data) == 5 {
		msg.ErrorDetails = data[4]
	}
	return nil
}

// CallError returns a json encoded WAMP 'CALLERROR' message as a byte slice
// callID is the randomly generated string provided by the client, errorURI is
// a URI identifying the error, errorDesc is a human-readable description of the
// error (for developers), errorDetails, if present, is a non-nil object
func CallError(callID, errorURI, errorDesc string, errorDetails ...interface{}) ([]byte, error) {
	if _, err := url.ParseRequestURI(errorURI); err != nil {
		return nil, &WAMPError{"invalid URI: %s" + errorURI}
	}
	var data []interface{}
	data = append(data, TYPE_ID_CALLERROR, callID, errorURI, errorDesc)
	data = append(data, errorDetails...)
	return json.Marshal(data)
}

// SUBSCRIBE
type SubscribeMsg struct {
	TopicURI string
}

func (msg *SubscribeMsg) UnmarshalJSON(jsonData []byte) error {
	var data []interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		return err
	}
	if len(data) != 2 {
		return ErrInvalidNumArgs
	}
	var ok bool
	if msg.TopicURI, ok = data[1].(string); !ok {
		return &WAMPError{"invalid topicURI"}
	}
	return nil
}

// Subscribe returns a json encoded WAMP 'SUBSCRIBE' message as a byte slice
// topicURI is the topic that the client wants to subscribe to
func Subscribe(topicURI string) ([]byte, error) {
	return createWAMPMessagePubSub(TYPE_ID_SUBSCRIBE, topicURI)
}

// UNSUBSCRIBE
type UnsubscribeMsg struct {
	TopicURI string
}

func (msg *UnsubscribeMsg) UnmarshalJSON(jsonData []byte) error {
	var data []interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		return err
	}
	if len(data) != 2 {
		return ErrInvalidNumArgs
	}
	var ok bool
	if msg.TopicURI, ok = data[1].(string); !ok {
		return &WAMPError{"invalid topicURI"}
	}
	return nil
}

// Unsubscribe returns a json encoded WAMP 'UNSUBSCRIBE' message as a byte slice
// topicURI is the topic that the client wants to unsubscribe from
func Unsubscribe(topicURI string) ([]byte, error) {
	return createWAMPMessagePubSub(TYPE_ID_UNSUBSCRIBE, topicURI)
}

// PUBLISH
type PublishMsg struct {
	TopicURI     string
	Event        interface{}
	ExcludeMe    bool
	ExcludeList  []interface{}
	EligibleList []interface{}
}

func (msg *PublishMsg) UnmarshalJSON(jsonData []byte) error {
	var data []interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		return err
	}
	if len(data) < 3 || len(data) > 5 {
		return ErrInvalidNumArgs
	}
	var ok bool
	if msg.TopicURI, ok = data[1].(string); !ok {
		return &WAMPError{"invalid topicURI"}
	}
	msg.Event = data[2]
	if len(data) > 3 {
		if msg.ExcludeMe, ok = data[3].(bool); !ok {
			if msg.ExcludeList, ok = data[3].([]interface{}); !ok {
				return &WAMPError{"invalid exclude argument"}
			}
			if len(data) == 5 {
				if msg.EligibleList, ok = data[4].([]interface{}); !ok {
					return &WAMPError{"invalid eligible list"}
				}
			}
		}
	}
	return nil
}

// Publish returns a json encoded WAMP 'PUBLISH' message as a byte slice
// arguments must be given in one of the following formats:
// [ topicURI, event ]
// [ topicURI, event, excludeMe ]
// [ topicURI, event, exclude ]
// [ topicURI, event, exclude, eligible ]
// event can be nil, a simple json type, or a complex json type
func Publish(topicURI string, event interface{}, opts ...interface{}) ([]byte, error) {
	var data []interface{}
	data = append(data, TYPE_ID_PUBLISH, topicURI, event)
	data = append(data, opts...)
	return createWAMPMessagePubSub(data...)
}

// EVENT
type EventMsg struct {
	TopicURI string
	Event    interface{}
}

func (msg *EventMsg) UnmarshalJSON(jsonData []byte) error {
	var data []interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		return err
	}
	if len(data) != 3 {
		return ErrInvalidNumArgs
	}
	var ok bool
	if msg.TopicURI, ok = data[1].(string); !ok {
		return &WAMPError{"invalid topicURI"}
	}
	msg.Event = data[2]

	return nil
}

// Event returns a json encoded WAMP 'EVENT' message as a byte slice
// event can be nil, a simple json type, or a complex json type
func Event(topicURI string, event interface{}) ([]byte, error) {
	return createWAMPMessagePubSub(TYPE_ID_EVENT, topicURI, event)
}

// createWAMPMessagePubSub checks that the second argument (topicURI) is a valid
// URI and then passes the request on to createWAMPMessage
func createWAMPMessagePubSub(args ...interface{}) ([]byte, error) {
	if _, err := url.ParseRequestURI(args[1].(string)); err != nil {
		return nil, &WAMPError{"invalid URI: %s" + args[1].(string)}
	}
	return createWAMPMessage(args...)
}

// createWAMPMessage returns a JSON encoded list from all the arguments passed to it
func createWAMPMessage(args ...interface{}) ([]byte, error) {
	var data []interface{}
	data = append(data, args...)
	return json.Marshal(data)
}
