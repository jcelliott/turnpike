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

// A WAMPSyntaxError is returned when attempting to create a message that does not follow the WAMP
// protocol
type WAMPSyntaxError struct {
	Msg string
}

func (e *WAMPSyntaxError) Error() string {
	return "wamp: %s" + e.Msg
}

// WelcomeMsg returns a json encoded WAMP 'WELCOME' message as a byte slice
// sessionId a randomly generated string provided by the server
func WelcomeMsg(sessionId string) ([]byte, error) {
	return createWAMPMessage(TYPE_ID_WELCOME, sessionId, PROTOCOL_VERSION, ServerIdent)
}

// PrefixMsg returns a json encoded WAMP 'PREFIX' message as a byte slice
func PrefixMsg(prefix, URI string) ([]byte, error) {
	if _, err := url.ParseRequestURI(URI); err != nil {
		return nil, &WAMPSyntaxError{"invalid URI: %s" + URI}
	}
	return createWAMPMessage(TYPE_ID_PREFIX, prefix, URI)
}

// CallMsg returns a json encoded WAMP 'CALL' message as a byte slice
// callID must be a randomly generated string, procURI is the URI of the remote
// procedure to be called, followed by zero or more call arguments
func CallMsg(callID, procURI string, args ...interface{}) ([]byte, error) {
	if _, err := url.ParseRequestURI(procURI); err != nil {
		return nil, &WAMPSyntaxError{"invalid URI: %s" + procURI}
	}
	var data []interface{}
	data = append(data, TYPE_ID_CALL, callID, procURI)
	data = append(data, args...)
	return json.Marshal(data)
}

// CallResultMsg returns a json encoded WAMP 'CALLRESULT' message as a byte slice
// callID is the randomly generated string provided by the client
func CallResultMsg(callID string, result interface{}) ([]byte, error) {
	return createWAMPMessage(TYPE_ID_CALLRESULT, callID, result)
}

// CallErrorMsg returns a json encoded WAMP 'CALLERROR' message as a byte slice
// callID is the randomly generated string provided by the client, errorURI is
// a URI identifying the error, errorDesc is a human-readable description of the
// error (for developers), errorDetails, if present, is a non-nil object
func CallErrorMsg(callID, errorURI, errorDesc string, errorDetails ...interface{}) ([]byte, error) {
	if _, err := url.ParseRequestURI(errorURI); err != nil {
		return nil, &WAMPSyntaxError{"invalid URI: %s" + errorURI}
	}
	var data []interface{}
	data = append(data, TYPE_ID_CALLERROR, callID, errorURI, errorDesc)
	data = append(data, errorDetails...)
	return json.Marshal(data)
}

// SubscribeMsg returns a json encoded WAMP 'SUBSCRIBE' message as a byte slice
// topicURI is the topic that the client wants to subscribe to
func SubscribeMsg(topicURI string) ([]byte, error) {
	return createWAMPMessagePubSub(TYPE_ID_SUBSCRIBE, topicURI)
}

// UnsubscribeMsg returns a json encoded WAMP 'UNSUBSCRIBE' message as a byte slice
// topicURI is the topic that the client wants to unsubscribe from
func UnsubscribeMsg(topicURI string) ([]byte, error) {
	return createWAMPMessagePubSub(TYPE_ID_UNSUBSCRIBE, topicURI)
}

// PublishMsg returns a json encoded WAMP 'PUBLISH' message as a byte slice
// arguments must be given in one of the following formats:
// [ topicURI, event ]
// [ topicURI, event, excludeMe ]
// [ topicURI, event, exclude ]
// [ topicURI, event, exclude, eligible ]
// event can be nil, a simple json type, or a complex json type
func PublishMsg(topicURI string, event interface{}, opts ...interface{}) ([]byte, error) {
	var data []interface{}
	data = append(data, TYPE_ID_PUBLISH, topicURI, event)
	data = append(data, opts...)
	return createWAMPMessagePubSub(data...)
}

// EventMsg returns a json encoded WAMP 'EVENT' message as a byte slice
// event can be nil, a simple json type, or a complex json type
func EventMsg(topicURI string, event interface{}) ([]byte, error) {
	return createWAMPMessagePubSub(TYPE_ID_EVENT, topicURI, event)
}

// createWAMPMessagePubSub checks that the second argument (topicURI) is a valid
// URI and then passes the request on to createWAMPMessage
func createWAMPMessagePubSub(args ...interface{}) ([]byte, error) {
	if _, err := url.ParseRequestURI(args[1].(string)); err != nil {
		return nil, &WAMPSyntaxError{"invalid URI: %s" + args[1].(string)}
	}
	return createWAMPMessage(args...)
}

// createWAMPMessage returns a JSON encoded list from all the arguments passed to it
func createWAMPMessage(args ...interface{}) ([]byte, error) {
	var data []interface{}
	data = append(data, args...)
	return json.Marshal(data)
}
