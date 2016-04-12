package turnpike

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestRegister(t *testing.T) {
	Convey("Registering a procedure", t, func() {
		dealer := NewDefaultDealer().(*defaultDealer)
		callee := &TestPeer{}
		testProcedure := URI("turnpike.test.endpoint")
		msg := &Register{Request: 123, Procedure: testProcedure}
		sess := &Session{Peer: callee}
		dealer.Register(sess, msg)

		Convey("The callee should have received a REGISTERED message", func() {
			reg := callee.received.(*Registered).Registration
			So(reg, ShouldNotEqual, 0)
		})

		Convey("The dealer should have the endpoint registered", func() {
			reg := callee.received.(*Registered).Registration
			proc, ok := dealer.procedures[testProcedure]
			So(ok, ShouldBeTrue)
			So(reg, ShouldEqual, proc.Registration)
			procedure, ok := dealer.registrations[reg]
			So(ok, ShouldBeTrue)
			So(procedure, ShouldEqual, testProcedure)
		})

		Convey("The same procedure cannot be registered more than once", func() {
			msg := &Register{Request: 321, Procedure: testProcedure}
			dealer.Register(sess, msg)
			err := callee.received.(*Error)
			So(err.Error, ShouldEqual, ErrProcedureAlreadyExists)
			So(err.Details, ShouldNotBeNil)
		})
	})
}

func TestUnregister(t *testing.T) {
	dealer := NewDefaultDealer().(*defaultDealer)
	callee := &TestPeer{}
	testProcedure := URI("turnpike.test.endpoint")
	msg := &Register{Request: 123, Procedure: testProcedure}
	sess := &Session{Peer: callee}
	dealer.Register(sess, msg)
	reg := callee.received.(*Registered).Registration

	Convey("Unregistering a procedure", t, func() {
		msg := &Unregister{Request: 124, Registration: reg}
		dealer.Unregister(sess, msg)

		Convey("The callee should have received an UNREGISTERED message", func() {
			unreg := callee.received.(*Unregistered).Request
			So(unreg, ShouldNotEqual, 0)
		})

		Convey("The dealer should no longer have the endpoint registered", func() {
			_, ok := dealer.procedures[testProcedure]
			So(ok, ShouldBeFalse)
			_, ok = dealer.registrations[reg]
			So(ok, ShouldBeFalse)
		})
	})
}

func TestCall(t *testing.T) {
	Convey("With a procedure registered", t, func() {
		dealer := NewDefaultDealer().(*defaultDealer)
		callee := &TestPeer{}
		testProcedure := URI("turnpike.test.endpoint")
		msg := &Register{Request: 123, Procedure: testProcedure}
		sess := &Session{Peer: callee}
		dealer.Register(sess, msg)
		caller := &TestPeer{}
		callerSession := &Session{Peer: caller}

		Convey("Calling an invalid procedure", func() {
			msg := &Call{Request: 124, Procedure: URI("turnpike.test.bad")}
			dealer.Call(callerSession, msg)

			Convey("The caller should have received an ERROR message", func() {
				err := caller.getReceived().(*Error)
				So(err.Error, ShouldEqual, ErrNoSuchProcedure)
				So(err.Details, ShouldNotBeNil)
			})
		})

		Convey("Calling a valid procedure", func() {
			msg := &Call{Request: 125, Procedure: testProcedure}
			dealer.Call(callerSession, msg)

			Convey("The callee should have received an INVOCATION message", func() {
				So(callee.received.MessageType(), ShouldEqual, INVOCATION)
				inv := callee.received.(*Invocation)

				Convey("And the callee responds with a YIELD message", func() {
					msg := &Yield{Request: inv.Request}
					dealer.Yield(sess, msg)

					// give it some time to propagate
					time.Sleep(time.Millisecond)

					Convey("The caller should have received a RESULT message", func() {
						So(caller.getReceived(), ShouldNotBeNil)
						So(caller.getReceived().MessageType(), ShouldEqual, RESULT)
						So(caller.getReceived().(*Result).Request, ShouldEqual, 125)
					})
				})

				Convey("And the callee responds with an ERROR message", func() {
					msg := &Error{Request: inv.Request}
					dealer.Error(sess, msg)

					// give it some time to propagate
					time.Sleep(time.Millisecond)

					Convey("The caller should have received an ERROR message", func() {
						So(caller.getReceived(), ShouldNotBeNil)
						So(caller.getReceived().MessageType(), ShouldEqual, ERROR)
						So(caller.getReceived().(*Error).Request, ShouldEqual, 125)
					})
				})
			})
		})
	})
}

func TestRemovePeer(t *testing.T) {
	Convey("With a procedure registered", t, func() {
		dealer := NewDefaultDealer().(*defaultDealer)
		callee := &TestPeer{}
		testProcedure := URI("turnpike.test.endpoint")
		msg := &Register{Request: 123, Procedure: testProcedure}
		sess := &Session{Peer: callee}
		dealer.Register(sess, msg)
		reg := callee.received.(*Registered).Registration
		So(dealer.procedures, ShouldContainKey, testProcedure)
		So(dealer.registrations, ShouldContainKey, reg)

		Convey("Calling RemoveSession should remove the registration", func() {
			dealer.RemoveSession(sess)
			So(dealer.registrations, ShouldNotContainKey, testProcedure)
			So(dealer.procedures, ShouldNotContainKey, reg)

			Convey("And registering the endpoint again should succeed", func() {
				msg.Request = 124
				dealer.Register(sess, msg)
				So(callee.received.MessageType(), ShouldEqual, REGISTERED)
			})
		})
	})
}
