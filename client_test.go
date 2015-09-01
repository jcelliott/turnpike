package turnpike

import (
	"fmt"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func newTestRouter() *defaultRouter {
	router := NewDefaultRouter()
	router.RegisterRealm(URI("turnpike.test"), Realm{})
	return router.(*defaultRouter)
}

func connectedTestClients() (*Client, *Client) {
	router := newTestRouter()
	peer1 := router.getTestPeer()
	peer2 := router.getTestPeer()
	return newTestClient(peer1), newTestClient(peer2)
}

func newTestClient(p Peer) *Client {
	client := NewClient(p)
	client.ReceiveTimeout = 100 * time.Millisecond
	_, err := client.JoinRealm("turnpike.test", nil)
	So(err, ShouldBeNil)
	return client
}

func TestJoinRealm(t *testing.T) {
	Convey("Given a server accepting client connections", t, func() {
		peer := newTestRouter().getTestPeer()

		Convey("A client should be able to succesfully join a realm", func() {
			client := NewClient(peer)
			_, err := client.JoinRealm("turnpike.test", nil)
			So(err, ShouldBeNil)
		})
	})
}

func testAuthFunc(d map[string]interface{}, c map[string]interface{}) (string, map[string]interface{}, error) {
	return testCRSign(c), map[string]interface{}{}, nil
}

func TestJoinRealmWithAuth(t *testing.T) {
	Convey("Given a server accepting client connections", t, func() {
		router := newTestRouter()
		router.RegisterRealm(URI("turnpike.test.auth"), Realm{
			CRAuthenticators: map[string]CRAuthenticator{"testauth": &testCRAuthenticator{}},
		})

		peer := router.getTestPeer()

		Convey("A client should be able to successfully authenticate and join a realm", func() {
			client := NewClient(peer)
			client.Auth = map[string]AuthFunc{"testauth": testAuthFunc}
			details := map[string]interface{}{"username": "tester"}
			_, err := client.JoinRealm("turnpike.test.auth", details)
			So(err, ShouldBeNil)
		})
	})
}

func TestRemoteCall(t *testing.T) {
	Convey("Given two clients connected to the same server", t, func() {
		callee, caller := connectedTestClients()

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
			err := callee.BasicRegister(methodName, handler)

			Convey("And expects no error", func() {
				So(err, ShouldBeNil)

				Convey("The caller calls the callee's remote method", func() {
					callArgs := []interface{}{5100}
					result, err := caller.Call(methodName, callArgs, make(map[string]interface{}))

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

		// RegisterService registers in the dealer the set of methods of the
		// receiver value that satisfy the following conditions:
		//	- exported method of exported type
		//	- two arguments, both of exported type
		//	- at least one return value, of type error
		// It returns an error if the receiver is not an exported type or has
		// no suitable methods.
		// The client accesses each method using a string of the form "type.method",
		// where type is the receiver's concrete type.
		Convey("The callee registers an invalid service", func() {
			Convey("the type is not a struct", func() {
				s := "invalid service"
				err := callee.RegisterService(s)
				Convey("Should result in an error", func() {
					So(err, ShouldNotBeNil)
				})
			})
			Convey("the type is not exported", func() {
				s := &struct{}{}
				err := callee.RegisterService(s)
				Convey("Should result in an error", func() {
					So(err, ShouldNotBeNil)
				})
			})
			Convey("no method is exported", func() {
				s := &noMethodExportedService{}
				err := callee.RegisterService(s)
				Convey("Should result in an error", func() {
					So(err, ShouldNotBeNil)
				})
			})
			Convey("exported method has no return value", func() {
				s := &noReturnValueService{}
				err := callee.RegisterService(s)
				Convey("Should result in an error", func() {
					So(err, ShouldNotBeNil)
				})
			})
			Convey("exported method has no return value of type error", func() {
				s := &noReturnValueOfTypeErrorService{}
				err := callee.RegisterService(s)
				Convey("Should result in an error", func() {
					So(err, ShouldNotBeNil)
				})
			})
		})

		Convey("The callee registers a valid service", func() {
			s := &ValidService{name: "ValidService"}
			err := callee.RegisterService(s)
			Convey("and expects no error", func() {
				So(err, ShouldBeNil)

				Convey("The caller calls the Ping method of the service", func() {
					var message string
					err := caller.CallService("ValidService.Ping", nil, &message)
					Convey("and expects no error", func() {
						So(err, ShouldBeNil)
					})
					Convey("and expects the message to be 'pong'", func() {
						So(message, ShouldEqual, "pong")
					})
				})

				Convey("The caller calls the Echo method of the service", func() {
					var message string
					err := caller.CallService("ValidService.Echo", []interface{}{"echo"}, &message)
					Convey("and expects no error", func() {
						So(err, ShouldBeNil)
					})
					Convey("and expects the message to be 'echo'", func() {
						So(message, ShouldEqual, "echo")
					})
				})

				Convey("The caller calls the Info method of the service", func() {
					var info *ServiceInfo
					err := caller.CallService("ValidService.Info", nil, &info)
					Convey("and expects no error", func() {
						So(err, ShouldBeNil)
					})
					Convey("and expects the info to be &ServiceInfo{ServiceName:'ValidService'}", func() {
						So(info, ShouldResemble, &ServiceInfo{ServiceName: "ValidService"})
					})
				})

				Convey("The caller calls the SetInfo method of the service", func() {
					info := &ServiceInfo{
						ServiceName: "NewServiceName",
					}
					err := caller.CallService("ValidService.SetInfo", []interface{}{info})
					Convey("and expects no error", func() {
						So(err, ShouldBeNil)
					})
					Convey("and expects the value of s.name to be 'NewServiceName'", func() {
						So(s.name, ShouldEqual, "NewServiceName")
					})
				})

				Convey("The caller calls the Error method of the service", func() {
					err := caller.CallService("ValidService.Error", nil)
					Convey("and expects an error", func() {
						So(err, ShouldNotBeNil)
						So(err.Error(), ShouldEqual, "Error")
					})
				})
			})

			Convey("The callee unregisters the service", func() {
				err := callee.UnregisterService("ValidService")
				Convey("and expects no error", func() {
					So(err, ShouldBeNil)
					Convey("The caller calls the Error method of the service", func() {
						err := caller.CallService("ValidService.Error", nil)
						Convey("and expects an error", func() {
							So(err, ShouldNotBeNil)
						})
					})
				})
			})
		})

		Convey("The callee unregisters a undefined service", func() {
			err := callee.Unregister("ServiceIsNotDefined")
			Convey("and expects an error", func() {
				So(err, ShouldNotBeNil)
			})
		})
	})
}

type noMethodExportedService struct{}

func (s *noMethodExportedService) notExported() error { return nil }

type noReturnValueService struct{}

func (s *noReturnValueService) NoReturnValue() {}

type noReturnValueOfTypeErrorService struct{}

func (s *noReturnValueOfTypeErrorService) NoReturnValue() string { return "" }

type ValidService struct {
	name string
}

func (s *ValidService) Ping() (string, error) {
	return "pong", nil
}

func (s *ValidService) Echo(message string) (string, error) {
	return message, nil
}

func (s *ValidService) Info() (*ServiceInfo, error) {
	return &ServiceInfo{
		ServiceName: s.name,
	}, nil
}

func (s *ValidService) SetInfo(info *ServiceInfo) error {
	s.name = info.ServiceName
	return nil
}

func (s *ValidService) Error() error {
	return fmt.Errorf("%s", "Error")
}

type ServiceInfo struct {
	ServiceName string
}
