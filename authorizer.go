package turnpike

// Authorizer is the interface implemented by an object that can determine
// whether a particular request is authorized or not.
//
// Authorize takes the session ID, the message (request), and the WELCOME
// details for that session and returns true if the request is authorized,
// otherwise false.
type Authorizer interface {
	Authorize(id ID, msg Message, details map[string]interface{}) (bool, error)
}

// DefaultAuthorizer always returns authorized.
type defaultAuthorizer struct {
}

// NewDefaultAuthorizer returns the default authorizer struct
func NewDefaultAuthorizer() Authorizer {
	return &defaultAuthorizer{}
}

func (da *defaultAuthorizer) Authorize(id ID, msg Message, details map[string]interface{}) (bool, error) {
	return true, nil
}
