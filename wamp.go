// Copyright (c) 2013 Joshua Elliott
// Released under the MIT License
// http://opensource.org/licenses/MIT

package turnpike

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
)

const (
	msgWelcome = iota
	msgPrefix
	msgCall
	msgCallResult
	msgCallError
	msgSubscribe
	msgUnsubscribe
	msgPublish
	msgEvent
)

const wampProtocolVersion = 1

var (
	typeReg = regexp.MustCompile("^\\s*\\[\\s*(\\d+)\\s*,")
)

// A WAMPError is returned when attempting to create a message that does not follow the WAMP
// protocol
type WAMPError struct {
	Msg string
}

var (
	// An ErrInvalidURI describes an invalid URI.
	ErrInvalidURI = &WAMPError{"invalid URI"}
	// An ErrInvalidNumArgs describes invalid number of arguments in a message.
	ErrInvalidNumArgs = &WAMPError{"invalid number of arguments in message"}
	// An ErrUnsupportedProtocol describes an unsupported WAMP protocol in a welcome message.
	ErrUnsupportedProtocol = &WAMPError{"unsupported protocol"}
)

// Error implements the error interface to provide a message.
func (e *WAMPError) Error() string {
	return "wamp: " + e.Msg
}

func parseMessageType(msg string) int {
	match := typeReg.FindStringSubmatch(msg)
	if match == nil {
		return -1
	}
	i, _ := strconv.Atoi(match[1])
	return i
}

func messageTypeString(typ int) string {
	types := []string{"WELCOME", "PREFIX", "CALL", "CALLRESULT", "CALLERROR", "SUBSCRIBE", "UNSUBSCRIBE", "PUBLISH", "EVENT"}
	if typ >= 0 && typ < 9 {
		return types[typ]
	}
	return ""
}

// WELCOME
type welcomeMsg struct {
	SessionId       string
	ProtocolVersion int
	ServerIdent     string
}

func (msg *welcomeMsg) UnmarshalJSON(jsonData []byte) error {
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
// sessionId is a randomly generated string provided by the server, serverIdent
// is a string that identifies the WAMP server
func createWelcome(sessionId, serverIdent string) (string, error) {
	return createWAMPMessage(msgWelcome, sessionId, wampProtocolVersion, serverIdent)
}

// PREFIX
type prefixMsg struct {
	Prefix string
	URI    string
}

func (msg *prefixMsg) UnmarshalJSON(jsonData []byte) error {
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
func createPrefix(prefix, URI string) (string, error) {
	if _, err := url.ParseRequestURI(URI); err != nil {
		return "", &WAMPError{"invalid URI: " + URI}
	}
	return createWAMPMessage(msgPrefix, prefix, URI)
}

// CALL
type callMsg struct {
	CallID   string
	ProcURI  string
	CallArgs []interface{}
}

func (msg *callMsg) UnmarshalJSON(jsonData []byte) error {
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
func createCall(callID, procURI string, args ...interface{}) (string, error) {
	if _, err := url.ParseRequestURI(procURI); err != nil {
		return "", &WAMPError{"invalid URI: " + procURI}
	}
	var data []interface{}
	data = append(data, msgCall, callID, procURI)
	data = append(data, args...)
	b, err := json.Marshal(data)
	return string(b), err
}

// CALLRESULT
type callResultMsg struct {
	CallID string
	Result interface{}
}

func (msg *callResultMsg) UnmarshalJSON(jsonData []byte) error {
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
func createCallResult(callID string, result interface{}) (string, error) {
	return createWAMPMessage(msgCallResult, callID, result)
}

// CALLERROR
type callErrorMsg struct {
	CallID       string
	ErrorURI     string
	ErrorDesc    string
	ErrorDetails interface{}
}

func (msg *callErrorMsg) UnmarshalJSON(jsonData []byte) error {
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
func createCallError(callID, errorURI, errorDesc string, errorDetails ...interface{}) (string, error) {
	if _, err := url.ParseRequestURI(errorURI); err != nil {
		return "", &WAMPError{"invalid URI: " + errorURI}
	}
	var data []interface{}
	data = append(data, msgCallError, callID, errorURI, errorDesc)
	data = append(data, errorDetails...)
	b, err := json.Marshal(data)
	return string(b), err
}

// SUBSCRIBE
type subscribeMsg struct {
	TopicURI string
}

func (msg *subscribeMsg) UnmarshalJSON(jsonData []byte) error {
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
func createSubscribe(topicURI string) (string, error) {
	return createWAMPMessagePubSub(msgSubscribe, topicURI)
}

// UNSUBSCRIBE
type unsubscribeMsg struct {
	TopicURI string
}

func (msg *unsubscribeMsg) UnmarshalJSON(jsonData []byte) error {
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
func createUnsubscribe(topicURI string) (string, error) {
	return createWAMPMessagePubSub(msgUnsubscribe, topicURI)
}

// PUBLISH
type publishMsg struct {
	TopicURI     string
	Event        interface{}
	ExcludeMe    bool
	ExcludeList  []string
	EligibleList []string
}

func (msg *publishMsg) UnmarshalJSON(jsonData []byte) error {
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
			var arr []interface{}
			if arr, ok = data[3].([]interface{}); !ok && data[3] != nil {
				return &WAMPError{"invalid exclude argument"}
			}
			for _, v := range arr {
				if val, ok := v.(string); !ok {
					return &WAMPError{"invalid exclude list"}
				} else {
					msg.ExcludeList = append(msg.ExcludeList, val)
				}
			}
			if len(data) == 5 {
				if arr, ok = data[4].([]interface{}); !ok && data[3] != nil {
					return &WAMPError{"invalid eligable list"}
				}
				for _, v := range arr {
					if val, ok := v.(string); !ok {
						return &WAMPError{"invalid eligable list"}
					} else {
						msg.EligibleList = append(msg.EligibleList, val)
					}
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
func createPublish(topicURI string, event interface{}, opts ...interface{}) (string, error) {
	var data []interface{}
	data = append(data, msgPublish, topicURI, event)
	data = append(data, opts...)
	return createWAMPMessagePubSub(data...)
}

// EVENT
type eventMsg struct {
	TopicURI string
	Event    interface{}
}

func (msg *eventMsg) UnmarshalJSON(jsonData []byte) error {
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
func createEvent(topicURI string, event interface{}) (string, error) {
	return createWAMPMessagePubSub(msgEvent, topicURI, event)
}

// createWAMPMessagePubSub checks that the second argument (topicURI) is a valid
// URI and then passes the request on to createWAMPMessage
func createWAMPMessagePubSub(args ...interface{}) (string, error) {
	if _, err := url.ParseRequestURI(args[1].(string)); err != nil {
		return "", &WAMPError{"invalid URI: " + args[1].(string)}
	}
	return createWAMPMessage(args...)
}

// createWAMPMessage returns a JSON encoded list from all the arguments passed to it
func createWAMPMessage(args ...interface{}) (string, error) {
	var data []interface{}
	data = append(data, args...)
	b, err := json.Marshal(data)
	return string(b), err
}
