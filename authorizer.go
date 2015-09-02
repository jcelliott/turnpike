package turnpike

type Authorizer interface {
	IsAuthorized(msg Message, details map[string]interface{}) (bool, error)
}

// DefaultAuthorizer always returns authorized.
type defaultAuthorizer struct {
}

func NewDefaultAuthorizer() *defaultAuthorizer {
	return &defaultAuthorizer{}
}

func (da *defaultAuthorizer) IsAuthorized(msg Message, details map[string]interface{}) (bool, error) {
	return true, nil
}
