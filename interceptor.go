package turnpike

type Interceptor interface {
	Intercept(id ID, msg *Message, details map[string]interface{})
}

// DefaultInterceptor does nothing :)
type defaultInterceptor struct {
}

func NewDefaultInterceptor() *defaultInterceptor {
	return &defaultInterceptor{}
}

func (di *defaultInterceptor) Intercept(id ID, msg *Message, details map[string]interface{}) {
}
