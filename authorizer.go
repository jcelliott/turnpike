package turnpike

type Authorizer interface {
	Authorize(id ID, msg Message, details map[string]interface{}) (bool, error)
}

// DefaultAuthorizer always returns authorized.
type defaultAuthorizer struct {
}

func NewDefaultAuthorizer() *defaultAuthorizer {
	return &defaultAuthorizer{}
}

func (da *defaultAuthorizer) Authorize(id ID, msg Message, details map[string]interface{}) (bool, error) {
	return true, nil
}
