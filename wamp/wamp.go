// Package wamp implements the Websocket Application Messaging Protocol
package wamp

import (
	"encoding/json"
	// "fmt"
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

type WAMPSyntaxError struct {
	Msg string
}

func (e *WAMPSyntaxError) Error() string {
	return "wamp: %s" + e.Msg
}

// EventMsg returns a json encoded WAMP 'EVENT' message as a byte slice
// event can be nil, a simple json type, or a complex json type
func EventMsg(topicURI string, event interface{}) ([]byte, error) {
	if _, err := url.ParseRequestURI(topicURI); err != nil {
		return nil, &WAMPSyntaxError{"invalid URI: %s" + topicURI}
	}

	var data []interface{}
	data = append(data, TYPE_ID_EVENT, topicURI, event)

	return json.Marshal(data)
}

// PublishMsg returns a json encoded WAMP 'PUBLISH' message as a byte slice
// arguments must be given in one of the following formats:
// [ topicURI, event ]
// [ topicURI, event, excludeMe ]
// [ topicURI, event, exclude, eligible ]
// event can be nil, a simple json type, or a complex json type
func PublishMsg(topicURI string, event interface{}, opts ...interface{}) ([]byte, error) {
	if _, err := url.ParseRequestURI(topicURI); err != nil {
		return nil, &WAMPSyntaxError{"invalid URI: %s" + topicURI}
	}

	var data []interface{}
	data = append(data, TYPE_ID_PUBLISH, topicURI, event)

	switch {
	case len(opts) == 0:
		// [ TYPE_ID_PUBLISH, topicURI, event ]
		return json.Marshal(data)
	case len(opts) == 1:
		// [ TYPE_ID_PUBLISH, topicURI, event, excludeMe ]
		data = append(data, opts[0])
		return json.Marshal(data)
	case len(opts) == 2:
		// [ TYPE_ID_PUBLISH, topicURI, event, exclude, eligible ]
		data = append(data, opts[0], opts[1])
		return json.Marshal(data)
	}

	return nil, &WAMPSyntaxError{"invalid arguments to PublishMsg"}
}

// // EventMsg represents a WAMP 'EVENT' message
// type EventMsg struct {
// 	TopicURI string
// 	Event    interface{}
// }
// 
// func (msg EventMsg) MarshalJSON() ([]byte, error) {
// 	if _, err := url.Parse(msg.TopicURI); err != nil {
// 		return nil, &WAMPSyntaxError{"invalid URI: %s" + msg.TopicURI}
// 	}
// 
// 	var data []interface{}
// 	data = append(data, TYPE_ID_EVENT, msg.TopicURI, msg.Event)
// 
// 	return json.Marshal(data)
// }
// 
// // PublishMsg represents a WAMP 'PUBLISH' message
// type PublishMsg struct {
// 	TopicURI     string
// 	Event        interface{}
// 	ExcludeMe    bool
// 	ExcludeList  []string
// 	EligibleList []string
// }
// 
// func (msg PublishMsg) MarshalJSON() ([]byte, error) {
// 	if _, err := url.Parse(msg.TopicURI); err != nil {
// 		return nil, &WAMPSyntaxError{"invalid URI: %s" + msg.TopicURI}
// 	}
// 
// 	if msg.ExcludeMe {
// 		if msg.ExcludeList != nil || msg.EligibleList != nil {
// 			return nil, &WAMPSyntaxError("cannot specify 'excludeMe' and exclude/eligibe list together")
// 		}
// 	}
// 	var data []interface{}
// 	data = append(data, TYPE_ID_PUBLISH, msg.TopicURI)
// 
// 	return nil, nil
// }
