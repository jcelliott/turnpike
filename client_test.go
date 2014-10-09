package turnpike

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type TestPeer struct {
	messages      chan Message
	sent_messages []Message
}

func (t *TestPeer) Send(msg Message) error {
	t.sent_messages = append(t.sent_messages, msg)

	switch msg := msg.(type) {

	case *Register:
		// Only allow methods named "mymethod" to be registered.
		if msg.Procedure == "mymethod" {
			args := make([]interface{}, 0, 0)
			args = append(args, 1234)

			t.messages <- &Registered{
				Request:      msg.Request,
				Registration: 4567,
			}
		} else {
			t.messages <- &Error{
				Type:        REGISTER,
				Request:     msg.Request,
				Details:     msg.Options,
				Error:       "invalid method",
				Arguments:   make([]interface{}, 0, 0),
				ArgumentsKw: make(map[string]interface{}),
			}
		}

	case *Yield:
		// Transform the yield into a result, and send it back to the client.
		t.messages <- &Result{
			Request:     msg.Request,
			Details:     msg.Options,
			Arguments:   msg.Arguments,
			ArgumentsKw: msg.ArgumentsKw,
		}

	case *Call:
		// testmethod: A method called by the client test (fake)
		// mymethod: A method called by the server test (does real work)
		if msg.Procedure == "testmethod" {
			args := make([]interface{}, 0, 0)
			args = append(args, 1234)

			t.messages <- &Result{
				Request:     msg.Request,
				Details:     msg.Options,
				Arguments:   args,
				ArgumentsKw: make(map[string]interface{}),
			}
		} else if msg.Procedure == "mymethod" {
			t.messages <- &Invocation{
				Request:      msg.Request,
				Registration: 4567, // Must match the registered message above.
				Details:      msg.Options,
				Arguments:    msg.Arguments,
				ArgumentsKw:  msg.ArgumentsKw,
			}
		} else {
			t.messages <- &Error{
				Type:        CALL,
				Request:     msg.Request,
				Details:     msg.Options,
				Error:       "unknown method",
				Arguments:   make([]interface{}, 0, 0),
				ArgumentsKw: make(map[string]interface{}),
			}
		}
	}

	return nil
}

func (t *TestPeer) Close() error {
	return nil
}

func (t *TestPeer) Receive() <-chan Message {
	return t.messages
}

func TestRemoteCall(t *testing.T) {
	Convey("Given a client connected to a peer", t, func() {
		testPeer := &TestPeer{
			messages:      make(chan Message),
			sent_messages: make([]Message, 0, 1),
		}

		client, err := NewClientInternal(testPeer, "testrealm", ALL)
		Convey("The client registers a method", func() {
			So(err, ShouldEqual, nil)

			err := client.Register("mymethod", func(args []interface{}, kwargs map[string]interface{}) *CallResult {
				return ValueResult(args[0].(int) * 2)
			})

			Convey("And expects no error", func() {
				So(err, ShouldEqual, nil)
			})

			other_client, err := NewClientInternal(testPeer, "testrealm", ALL)
			Convey("We create another client", func() {
				So(err, ShouldEqual, nil)

				Convey("That calls the first client's remote method", func() {
					call_args := make([]interface{}, 0, 0)
					call_args = append(call_args, 5100)
					result, err := other_client.Call("mymethod", call_args, make(map[string]interface{}))
					Convey("And succeeds at multiplying the number by 2", func() {
						So(err, ShouldEqual, nil)
						So(result.(*Result).Arguments[0], ShouldEqual, 10200)
					})
				})
			})
		})

		Convey("The client registers an invalid method", func() {
			So(err, ShouldEqual, nil)

			err := client.Register("invalidmethod", func(args []interface{}, kwargs map[string]interface{}) *CallResult {
				return nil
			})

			Convey("And expects an error", func() {
				So(err, ShouldNotBeNil)
			})
		})
	})
}

func TestClientCall(t *testing.T) {
	Convey("Given a client connected to a peer", t, func() {
		testPeer := &TestPeer{
			messages:      make(chan Message),
			sent_messages: make([]Message, 0, 1),
		}

		client, err := NewClientInternal(testPeer, "testrealm", ALL)
		Convey("The client calls a valid method", func() {
			So(err, ShouldEqual, nil)

			result, err := client.Call("testmethod", make([]interface{}, 0, 0), make(map[string]interface{}))
			Convey("And expects a result", func() {
				So(err, ShouldEqual, nil)
				So(result.(*Result).Arguments[0], ShouldEqual, 1234)
			})
		})

		Convey("The client calls an invalid method", func() {
			So(err, ShouldEqual, nil)

			_, err := client.Call("invalidmethod", make([]interface{}, 0, 0), make(map[string]interface{}))
			Convey("And expects an error", func() {
				So(err, ShouldNotBeNil)
			})
		})
	})
}

func TestHello(t *testing.T) {
	Convey("Given an 'ALL' client connected to a peer", t, func() {
		testPeer := &TestPeer{
			messages:      make(chan Message),
			sent_messages: make([]Message, 0, 1),
		}

		_, err := NewClientInternal(testPeer, "testrealm", ALL)
		Convey("The client says HELLO", func() {
			msg := testPeer.sent_messages[0]

			So(err, ShouldEqual, nil)
			So(msg.MessageType(), ShouldEqual, HELLO)

			typed_msg := testPeer.sent_messages[0].(*Hello)
			So(typed_msg.Details["roles"], ShouldNotBeNil)

			roles := typed_msg.Details["roles"].(map[string]interface{})
			So(roles["publisher"], ShouldNotBeNil)
			So(roles["subscriber"], ShouldNotBeNil)
			So(roles["caller"], ShouldNotBeNil)
			So(roles["callee"], ShouldNotBeNil)
		})
	})

	Convey("Given a SUBSCRIBER-only client connected to a peer", t, func() {
		testPeer := &TestPeer{
			messages:      make(chan Message),
			sent_messages: make([]Message, 0, 1),
		}

		_, err := NewClientInternal(testPeer, "testrealm", SUBSCRIBER)
		Convey("The client says HELLO", func() {
			msg := testPeer.sent_messages[0]

			So(err, ShouldEqual, nil)
			So(msg.MessageType(), ShouldEqual, HELLO)

			typed_msg := testPeer.sent_messages[0].(*Hello)
			So(typed_msg.Details["roles"], ShouldNotBeNil)

			roles := typed_msg.Details["roles"].(map[string]interface{})
			So(roles["subscriber"], ShouldNotBeNil)
			So(roles["publisher"], ShouldBeNil)
			So(roles["caller"], ShouldBeNil)
			So(roles["callee"], ShouldBeNil)
		})
	})

	Convey("Given a PUBLISHER-only client connected to a peer", t, func() {
		testPeer := &TestPeer{
			messages:      make(chan Message),
			sent_messages: make([]Message, 0, 1),
		}

		_, err := NewClientInternal(testPeer, "testrealm", PUBLISHER)
		Convey("The client says HELLO", func() {
			msg := testPeer.sent_messages[0]

			So(err, ShouldEqual, nil)
			So(msg.MessageType(), ShouldEqual, HELLO)

			typed_msg := testPeer.sent_messages[0].(*Hello)
			So(typed_msg.Details["roles"], ShouldNotBeNil)

			roles := typed_msg.Details["roles"].(map[string]interface{})
			So(roles["subscriber"], ShouldBeNil)
			So(roles["publisher"], ShouldNotBeNil)
			So(roles["caller"], ShouldBeNil)
			So(roles["callee"], ShouldBeNil)
		})
	})

	Convey("Given a CALLER-only client connected to a peer", t, func() {
		testPeer := &TestPeer{
			messages:      make(chan Message),
			sent_messages: make([]Message, 0, 1),
		}

		_, err := NewClientInternal(testPeer, "testrealm", CALLER)
		Convey("The client says HELLO", func() {
			msg := testPeer.sent_messages[0]

			So(err, ShouldEqual, nil)
			So(msg.MessageType(), ShouldEqual, HELLO)

			typed_msg := testPeer.sent_messages[0].(*Hello)
			So(typed_msg.Details["roles"], ShouldNotBeNil)

			roles := typed_msg.Details["roles"].(map[string]interface{})
			So(roles["subscriber"], ShouldBeNil)
			So(roles["publisher"], ShouldBeNil)
			So(roles["caller"], ShouldNotBeNil)
			So(roles["callee"], ShouldBeNil)
		})
	})

	Convey("Given a CALLEE-only client connected to a peer", t, func() {
		testPeer := &TestPeer{
			messages:      make(chan Message),
			sent_messages: make([]Message, 0, 1),
		}

		_, err := NewClientInternal(testPeer, "testrealm", CALLEE)
		Convey("The client says HELLO", func() {
			msg := testPeer.sent_messages[0]

			So(err, ShouldEqual, nil)
			So(msg.MessageType(), ShouldEqual, HELLO)

			typed_msg := testPeer.sent_messages[0].(*Hello)
			So(typed_msg.Details["roles"], ShouldNotBeNil)

			roles := typed_msg.Details["roles"].(map[string]interface{})
			So(roles["subscriber"], ShouldBeNil)
			So(roles["publisher"], ShouldBeNil)
			So(roles["caller"], ShouldBeNil)
			So(roles["callee"], ShouldNotBeNil)
		})
	})
}
