package turnpike

type Interceptor interface {
	Modify(msg *Message, details map[string]interface{})
}

// DefaultInterceptor does nothing :)
type defaultInterceptor struct {
}

func NewDefaultInterceptor() *defaultInterceptor {
	return &defaultInterceptor{}
}

func (di *defaultInterceptor) Modify(msg *Message, details map[string]interface{}) {
}
