package turnpike

// Interceptor is the interface implemented by an object that intercepts
// messages in the router to modify them somehow.
//
// Intercept takes the session and (a pointer to) the message, and (possibly)
// modifies the message.
type Interceptor interface {
	Intercept(session Session, msg *Message)
}

// DefaultInterceptor does nothing :)
type defaultInterceptor struct {
}

// NewDefaultInterceptor returns the default interceptor, which does nothing.
func NewDefaultInterceptor() Interceptor {
	return &defaultInterceptor{}
}

func (di *defaultInterceptor) Intercept(session Session, msg *Message) {
}
