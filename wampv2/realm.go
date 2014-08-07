package wampv2

// Realm is the interface all WAMP realms must implement.
type Realm interface {
	// Broker returns a custom broker for this realm.
	// If this is nil, the default broker will be used.
	Broker() Broker
}

type DefaultRealm struct {
}

func NewDefaultRealm() *DefaultRealm {
	return &DefaultRealm{}
}

func (realm *DefaultRealm) Broker() Broker {
	return nil
}
