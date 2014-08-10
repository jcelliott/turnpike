package turnpike

import "math/rand"

const (
	// Peer is not authorized to access the given resource. This might be triggered by a session trying to join a realm, a publish, subscribe, register or call.
	WAMP_ERROR_NOT_AUTHORIZED = URI("wamp.error.not_authorized")

	// Peer wanted to join a non-existing realm (and the Router did not allow to auto-create the realm).
	WAMP_ERROR_NO_SUCH_REALM = URI("wamp.error.no_such_realm")

	// The Peer is shutting down completely - used as a GOODBYE (or ABORT) reason.
	WAMP_ERROR_SYSTEM_SHUTDOWN = URI("wamp.error.system_shutdown")

	// The Peer want to leave the realm - used as a GOODBYE reason.
	WAMP_ERROR_CLOSE_REALM = URI("wamp.error.close_realm")

	// A Peer acknowledges ending of a session - used as a GOOBYE reply reason.
	WAMP_ERROR_GOODBYE_AND_OUT = URI("wamp.error.goodbye_and_out")

	// A Dealer could not perform a call, since the procedure called does not exist.
	WAMP_ERROR_NO_SUCH_PROCEDURE = URI("wamp.error.no_such_procedure")

	// A Broker could not perform a unsubscribe, since the given subscription is not active.
	WAMP_ERROR_NO_SUCH_SUBSCRIPTION = URI("wamp.error.no_such_subscription")

	// A Dealer could not perform a unregister, since the given registration is not active.
	WAMP_ERROR_NO_SUCH_REGISTRATION = URI("wamp.error.no_such_registration")

	// A call failed, since the given argument types or values are not acceptable to the called procedure.
	WAMP_ERROR_INVALID_ARGUMENT = URI("wamp.error.invalid_argument")

	// A publish failed, since the given topic is not acceptable to the Broker.
	WAMP_ERROR_INVALID_TOPIC = URI("wamp.error.invalid_topic")

	// A procedure could not be registered, since a procedure with the given URI is already registered (and the Dealer is not able to set up a distributed registration).
	WAMP_ERROR_PROCEDURE_ALREADY_EXISTS = URI("wamp.error.procedure_already_exists")
)

const (
	MAX_ID = 2 << 53
)

func NewID() ID {
	return ID(rand.Intn(MAX_ID))
}
