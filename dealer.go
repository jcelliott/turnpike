package turnpike

// A Dealer routes and manages RPC calls to callees.
type Dealer interface {
	// Register a procedure on an endpoint
	Register(Sender, *Register)
	// Unregister a procedure on an endpoint
	Unregister(Sender, *Unregister)
	// Call a procedure on an endpoint
	Call(Sender, *Call)
	// Return the result of a procedure call
	Yield(Sender, *Yield)
}

type RemoteProcedure struct {
	Endpoint  Sender
	Procedure URI
}

type defaultDealer struct {
	// map registration IDs to procedures
	procedures map[ID]RemoteProcedure
	// map procedure URIs to registration IDs
	// TODO: this will eventually need to be `map[URI][]ID` to support
	// multiple callees for the same procedure
	registrations map[URI]ID
	// keep track of call IDs so we can send the response to the caller
	calls map[ID]Sender
	// link the invocation ID to the call ID
	invocations map[ID]ID
}

func NewDefaultDealer() Dealer {
	return &defaultDealer{
		procedures:    make(map[ID]RemoteProcedure),
		registrations: make(map[URI]ID),
		calls:         make(map[ID]Sender),
		invocations:   make(map[ID]ID),
	}
}

func (d *defaultDealer) Register(callee Sender, msg *Register) {
	if _, ok := d.registrations[msg.Procedure]; ok {
		callee.Send(&Error{
			Type:    msg.MessageType(),
			Request: msg.Request,
			Error:   WAMP_ERROR_PROCEDURE_ALREADY_EXISTS,
		})
		return
	}
	reg := NewID()
	d.procedures[reg] = RemoteProcedure{callee, msg.Procedure}
	d.registrations[msg.Procedure] = reg
	callee.Send(&Registered{
		Request:      msg.Request,
		Registration: reg,
	})
}

func (d *defaultDealer) Unregister(callee Sender, msg *Unregister) {
	if procedure, ok := d.procedures[msg.Registration]; !ok {
		// the registration doesn't exist
		callee.Send(&Error{
			Type:    msg.MessageType(),
			Request: msg.Request,
			Error:   WAMP_ERROR_NO_SUCH_REGISTRATION,
		})
	} else {
		delete(d.registrations, procedure.Procedure)
		delete(d.procedures, msg.Registration)
		callee.Send(&Unregistered{
			Request: msg.Request,
		})
	}
}

func (d *defaultDealer) Call(caller Sender, msg *Call) {
	if reg, ok := d.registrations[msg.Procedure]; !ok {
		caller.Send(&Error{
			Type:    msg.MessageType(),
			Request: msg.Request,
			Error:   WAMP_ERROR_NO_SUCH_PROCEDURE,
		})
	} else {
		if rproc, ok := d.procedures[reg]; !ok {
			// found a registration id, but doesn't match any remote procedure
			caller.Send(&Error{
				Type:    msg.MessageType(),
				Request: msg.Request,
				// TODO: what should this error be?
				Error: URI("wamp.error.internal_error"),
			})
		} else {
			// everything checks out, make the invocation request
			d.calls[msg.Request] = caller
			invocationID := NewID()
			d.invocations[invocationID] = msg.Request
			rproc.Endpoint.Send(&Invocation{
				Request:      invocationID,
				Registration: reg,
				Arguments:    msg.Arguments,
				ArgumentsKw:  msg.ArgumentsKw,
			})
		}
	}
}

func (d *defaultDealer) Yield(callee Sender, msg *Yield) {
	if callID, ok := d.invocations[msg.Request]; !ok {
		callee.Send(&Error{
			Type:    msg.MessageType(),
			Request: msg.Request,
			// TODO: what should this error be?
			Error: URI("wamp.error.no_such_invocation"),
		})
	} else {
		if caller, ok := d.calls[callID]; !ok {
			// found the invocation id, but doesn't match any call id
			callee.Send(&Error{
				Type:    msg.MessageType(),
				Request: msg.Request,
				// TODO: what should this error be?
				Error: URI("wamp.error.no_such_call"),
			})
		} else {
			delete(d.calls, callID)
			// return the result to the caller
			caller.Send(&Result{
				Request:     callID,
				Arguments:   msg.Arguments,
				ArgumentsKw: msg.ArgumentsKw,
			})
		}
	}
}
