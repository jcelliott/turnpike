package turnpike

import (
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type testPeer struct {
	messages     chan Message
	sentMessages []Message
}

func (t *testPeer) Send(msg Message) error {
	t.sentMessages = append(t.sentMessages, msg)

	switch msg := msg.(type) {

	case *Hello:
		if _, ok := msg.Details["authmethods"]; !ok {
			t.messages <- &Welcome{
				Id:      NewID(),
				Details: make(map[string]interface{}),
			}
		} else {
			t.messages <- &Challenge{
				AuthMethod: "testauth",
				Extra:      map[string]interface{}{"challenge": "password"},
			}
		}

	case *Authenticate:
		if msg.Signature == "passwordpassword" {
			t.messages <- &Welcome{
				Id:      NewID(),
				Details: make(map[string]interface{}),
			}
		} else {
			t.messages <- &Abort{
				Reason: URI("turnpike.error.invalid_auth_signature"),
			}
		}

	case *Register:
		// Only allow methods named "mymethod" to be registered.
		if msg.Procedure == "mymethod" {
			args := make([]interface{}, 0)
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
				Error:       WAMP_ERROR_INVALID_URI,
				Arguments:   make([]interface{}, 0),
				ArgumentsKw: make(map[string]interface{}),
			}
		}

	case *Unregister:
		// Only allow methods name "mymethod" (4567) to be unregistered
		if msg.Registration == 4567 {
			args := make([]interface{}, 0)
			args = append(args, 1234)

			t.messages <- &Unregistered{
				Request: msg.Request,
			}
		} else {
			t.messages <- &Error{
				Type:        UNREGISTER,
				Request:     msg.Request,
				Error:       WAMP_ERROR_INVALID_URI,
				Arguments:   make([]interface{}, 0),
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
			args := make([]interface{}, 0)
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
				Arguments:   make([]interface{}, 0),
				ArgumentsKw: make(map[string]interface{}),
			}
		}

	case *Error:
		// forward error messages
		t.messages <- msg
	}

	return nil
}

func (t *testPeer) Close() error {
	return nil
}

func (t *testPeer) Receive() <-chan Message {
	return t.messages
}

func newTestPeer() *testPeer {
	return &testPeer{
		messages: make(chan Message, 2),
	}
}

func connectedTestClients() (*Client, *Client) {
	peer := newTestPeer()
	return newTestClient(peer), newTestClient(peer)
}

func newTestClient(p Peer) *Client {
	client := NewClient(p)
	_, err := client.JoinRealm("test.realm", ALLROLES, nil)
	So(err, ShouldBeNil)
	return client
}

func TestJoinRealm(t *testing.T) {
	Convey("Given a server accepting client connections", t, func() {
		server := newTestPeer()

		Convey("A client should be able to succesfully join a realm", func() {
			client := NewClient(server)
			_, err := client.JoinRealm("test.realm", ALLROLES, nil)
			So(err, ShouldBeNil)
		})
	})
}

func testAuthFunc(d map[string]interface{}, c map[string]interface{}) (string, map[string]interface{}, error) {
	key := c["challenge"].(string)
	if key == "fail" {
		return "", map[string]interface{}{}, fmt.Errorf("authentication failed")
	}
	signature := key + key // it's super effective!
	return signature, map[string]interface{}{}, nil
}

func TestJoinRealmCRA(t *testing.T) {
	Convey("Given a server accepting client connections", t, func() {
		server := newTestPeer()

		Convey("A client should be able to successfully authenticate and join a realm", func() {
			details := map[string]interface{}{"authmethods": []string{"testauth"}}
			auth := map[string]AuthFunc{"testauth": testAuthFunc}
			client := NewClient(server)
			_, err := client.JoinRealmCRA("test.realm", ALLROLES, details, auth)
			So(err, ShouldBeNil)
		})
	})
}

func TestRemoteCall(t *testing.T) {
	Convey("Given two clients connected to the same server", t, func() {
		callee, caller := connectedTestClients()

		Convey("The callee registers an invalid method", func() {
			handler := func(args []interface{}, kwargs map[string]interface{}) *CallResult {
				return nil
			}
			err := callee.Register("invalidmethod", handler)

			Convey("And expects an error", func() {
				So(err, ShouldNotBeNil)
			})
		})

		Convey("The callee unregisters an invalid method", func() {
			err := callee.Unregister("invalidmethod")
			Convey("And expects an error", func() {
				So(err, ShouldNotBeNil)
			})
		})

		Convey("The callee registers a valid method", func() {
			handler := func(args []interface{}, kwargs map[string]interface{}) *CallResult {
				return &CallResult{Args: []interface{}{args[0].(int) * 2}}
			}
			methodName := "mymethod"
			err := callee.Register(methodName, handler)

			Convey("And expects no error", func() {
				So(err, ShouldBeNil)

				Convey("The caller calls the callee's remote method", func() {
					callArgs := []interface{}{5100}
					result, err := caller.Call("mymethod", callArgs, make(map[string]interface{}))

					Convey("And succeeds at multiplying the number by 2", func() {
						So(err, ShouldBeNil)
						So(result.Arguments[0], ShouldEqual, 10200)
					})
				})
			})
			Convey("And unregisters the method", func() {
				err := callee.Unregister(methodName)
				Convey("And expects no error", func() {
					So(err, ShouldBeNil)
				})

				Convey("Calling the unregistered procedure", func() {
					callArgs := []interface{}{5100}
					result, err := caller.Call(methodName, callArgs, make(map[string]interface{}))

					Convey("Should result in an error", func() {
						So(err, ShouldNotBeNil)
						So(result, ShouldBeNil)
					})
				})
			})
		})
	})
}

func TestClientCall(t *testing.T) {
	Convey("Given a client connected to a server", t, func() {
		server := newTestPeer()
		client := newTestClient(server)

		Convey("The client calls a valid method", func() {
			result, err := client.Call("testmethod", []interface{}{}, map[string]interface{}{})

			Convey("And expects a result", func() {
				So(err, ShouldBeNil)
				So(result.Arguments[0], ShouldEqual, 1234)
			})
		})

		Convey("The client calls an invalid method", func() {
			_, err := client.Call("invalidmethod", []interface{}{}, map[string]interface{}{})
			Convey("And expects an error", func() {
				So(err, ShouldNotBeNil)
			})
		})
	})
}
