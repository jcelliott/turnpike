package wampv2

type Callee interface {
	ErrorHandler
	// Acknowledge that the endpoint was succesfully registered
	SendRegistered(*Registered)
	// Acknowledge that the endpoint was succesfully unregistered
	SendUnregistered(*Unregistered)
	// Dealer requests fulfillment of a procedure call
	SendInvocation(*Invocation)
}

type Caller interface {
	ErrorHandler
	// Dealer sends the returned result from the procedure call
	SendResult(*Result)
}

type Dealer interface {
	// Register a procedure on an endpoint
	Register(Callee, *Register)
	// Unregister a procedure on an endpoint
	Unregister(Callee, *Unregister)
	// Call a procedure on an endpoint
	Call(Caller, *Call)
	// Return the result of a procedure call
	Yield(Callee, *Yield)
}

type RemoteProcedure struct {
	Endpoint  Callee
	Procedure URI
}

type DefaultDealer struct {
	// map registration IDs to procedures
	procedures map[ID]RemoteProcedure
	// map procedure URIs to registration IDs
	// TODO: this will eventually need to be `map[URI][]ID` to support
	// multiple callees for the same procedure
	registrations map[URI]ID
	// keep track of call IDs so we can send the response to the caller
	calls map[ID]Caller
	// link the invocation ID to the call ID
	invocations map[ID]ID
}

func NewDefaultDealer() *DefaultDealer {
	return &DefaultDealer{
		procedures:    make(map[ID]RemoteProcedure),
		registrations: make(map[URI]ID),
		calls:         make(map[ID]Caller),
		invocations:   make(map[ID]ID),
	}
}

func (d *DefaultDealer) Register(callee Callee, msg *Register) {
	if _, ok := d.registrations[msg.Procedure]; ok {
		callee.SendError(&Error{
			Type:    msg.MessageType(),
			Request: msg.Request,
			Error:   WAMP_ERROR_PROCEDURE_ALREADY_EXISTS,
		})
		return
	}
	reg := NewID()
	d.procedures[reg] = RemoteProcedure{callee, msg.Procedure}
	d.registrations[msg.Procedure] = reg
	callee.SendRegistered(&Registered{
		Request:      msg.Request,
		Registration: reg,
	})
}

func (d *DefaultDealer) Unregister(callee Callee, msg *Unregister) {
	if procedure, ok := d.procedures[msg.Registration]; !ok {
		// the registration doesn't exist
		callee.SendError(&Error{
			Type:    msg.MessageType(),
			Request: msg.Request,
			Error:   WAMP_ERROR_NO_SUCH_REGISTRATION,
		})
	} else {
		delete(d.registrations, procedure.Procedure)
		delete(d.procedures, msg.Registration)
		callee.SendUnregistered(&Unregistered{
			Request: msg.Request,
		})
	}
}

func (d *DefaultDealer) Call(caller Caller, msg *Call) {
	if reg, ok := d.registrations[msg.Procedure]; !ok {
		caller.SendError(&Error{
			Type:    msg.MessageType(),
			Request: msg.Request,
			Error:   WAMP_ERROR_NO_SUCH_PROCEDURE,
		})
	} else {
		if rproc, ok := d.procedures[reg]; !ok {
			// found a registration id, but doesn't match any remote procedure
			caller.SendError(&Error{
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
			rproc.Endpoint.SendInvocation(&Invocation{
				Request:      invocationID,
				Registration: reg,
				Arguments:    msg.Arguments,
				ArgumentsKw:  msg.ArgumentsKw,
			})
		}
	}
}

func (d *DefaultDealer) Yield(callee Callee, msg *Yield) {
	if callID, ok := d.invocations[msg.Request]; !ok {
		callee.SendError(&Error{
			Type:    msg.MessageType(),
			Request: msg.Request,
			// TODO: what should this error be?
			Error: URI("wamp.error.no_such_invocation"),
		})
	} else {
		if caller, ok := d.calls[callID]; !ok {
			// found the invocation id, but doesn't match any call id
			callee.SendError(&Error{
				Type:    msg.MessageType(),
				Request: msg.Request,
				// TODO: what should this error be?
				Error: URI("wamp.error.no_such_call"),
			})
		} else {
			// return the result to the caller
			caller.SendResult(&Result{
				Request:     callID,
				Arguments:   msg.Arguments,
				ArgumentsKw: msg.ArgumentsKw,
			})
		}
	}
}
