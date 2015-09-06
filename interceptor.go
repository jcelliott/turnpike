package turnpike

// Interceptor is the interface implemented by an object that intercepts
// messages in the router to modify them somehow.
//
// Intercept takes the session ID, (a pointer to) the message, and the WELCOME
// details for that session and (possibly) modifies the message.
type Interceptor interface {
	Intercept(id ID, msg *Message, details map[string]interface{})
}

// DefaultInterceptor does nothing :)
type defaultInterceptor struct {
}

// NewDefaultInterceptor returns the default interceptor, which does nothing.
func NewDefaultInterceptor() Interceptor {
	return &defaultInterceptor{}
}

func (di *defaultInterceptor) Intercept(id ID, msg *Message, details map[string]interface{}) {
}
