package turnpike

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestRegister(t *testing.T) {
	Convey("Registering a procedure", t, func() {
		dealer := NewDefaultDealer().(*defaultDealer)
		callee := getTestSession()
		testProcedure := URI("turnpike.test.endpoint")
		msg := &Register{Request: 123, Procedure: testProcedure}
		dealer.Register(callee, msg)

		Convey("The callee should have received a REGISTERED message", func() {
			reg := getTestMessage(callee).(*Registered).Registration
			So(reg, ShouldNotEqual, 0)
		})

		Convey("The dealer should have the endpoint registered", func() {
			reg := getTestMessage(callee).(*Registered).Registration
			reg2, ok := dealer.registrations[testProcedure]
			So(ok, ShouldBeTrue)
			So(reg, ShouldEqual, reg2)
			proc, ok := dealer.procedures[reg]
			So(ok, ShouldBeTrue)
			So(proc.Procedure, ShouldEqual, testProcedure)
		})

		Convey("The same procedure cannot be registered more than once", func() {
			msg := &Register{Request: 321, Procedure: testProcedure}
			dealer.Register(callee, msg)
			err := getTestMessage(callee).(*Error)
			So(err.Error, ShouldEqual, ErrProcedureAlreadyExists)
			So(err.Details, ShouldNotBeNil)
		})
	})
}

func TestUnregister(t *testing.T) {
	dealer := NewDefaultDealer().(*defaultDealer)
	callee := getTestSession()
	testProcedure := URI("turnpike.test.endpoint")
	msg := &Register{Request: 123, Procedure: testProcedure}
	dealer.Register(callee, msg)
	reg := getTestMessage(callee).(*Registered).Registration

	Convey("Unregistering a procedure", t, func() {
		msg := &Unregister{Request: 124, Registration: reg}
		dealer.Unregister(callee, msg)

		Convey("The callee should have received an UNREGISTERED message", func() {
			unreg := getTestMessage(callee).(*Unregistered).Request
			So(unreg, ShouldNotEqual, 0)
		})

		Convey("The dealer should no longer have the endpoint registered", func() {
			_, ok := dealer.registrations[testProcedure]
			So(ok, ShouldBeFalse)
			_, ok = dealer.procedures[reg]
			So(ok, ShouldBeFalse)
		})
	})
}

func TestCall(t *testing.T) {
	Convey("With a procedure registered", t, func() {
		dealer := NewDefaultDealer().(*defaultDealer)
		callee := getTestSession()
		testProcedure := URI("turnpike.test.endpoint")
		msg := &Register{Request: 123, Procedure: testProcedure}
		dealer.Register(callee, msg)
		caller := getTestSession()

		Convey("Calling an invalid procedure", func() {
			msg := &Call{Request: 124, Procedure: URI("turnpike.test.bad")}
			dealer.Call(caller, msg)

			Convey("The caller should have received an ERROR message", func() {
				err := getTestMessage(caller).(*Error)
				So(err.Error, ShouldEqual, ErrNoSuchProcedure)
				So(err.Details, ShouldNotBeNil)
			})
		})

		Convey("Calling a valid procedure", func() {
			msg := &Call{Request: 125, Procedure: testProcedure}
			dealer.Call(caller, msg)

			Convey("The callee should have received an INVOCATION message", func() {
				So(getTestMessage(callee).MessageType(), ShouldEqual, INVOCATION)
				inv := getTestMessage(callee).(*Invocation)

				Convey("And the callee responds with a YIELD message", func() {
					msg := &Yield{Request: inv.Request}
					dealer.Yield(callee, msg)

					Convey("The caller should have received a RESULT message", func() {
						So(getTestMessage(caller).MessageType(), ShouldEqual, RESULT)
						So(getTestMessage(caller).(*Result).Request, ShouldEqual, 125)
					})
				})

				Convey("And the callee responds with an ERROR message", func() {
					msg := &Error{Request: inv.Request}
					dealer.Error(callee, msg)

					Convey("The caller should have received an ERROR message", func() {
						So(getTestMessage(caller).MessageType(), ShouldEqual, ERROR)
						So(getTestMessage(caller).(*Error).Request, ShouldEqual, 125)
					})
				})
			})
		})
	})
}
