package turnpike

// Authorizer is the interface implemented by an object that can determine
// whether a particular request is authorized or not.
//
// Authorize takes the session and the message (request), and returns true if
// the request is authorized, otherwise false.
type Authorizer interface {
	Authorize(session Session, msg Message) (bool, error)
}

// DefaultAuthorizer always returns authorized.
type defaultAuthorizer struct {
}

// NewDefaultAuthorizer returns the default authorizer struct
func NewDefaultAuthorizer() Authorizer {
	return &defaultAuthorizer{}
}

func (da *defaultAuthorizer) Authorize(session Session, msg Message) (bool, error) {
	return true, nil
}
