package turnpike

import (
	"fmt"
)

type CRAuthenticator interface {
	// accept HELLO details and returns a challenge map (which will be sent in a CHALLENGE message)
	Challenge(details map[string]interface{}) (map[string]interface{}, error)
	// accept a challenge map (same as was generated in Challenge) and a signature string, and
	// authenticates the signature string against the challenge. Returns a details map and error.
	Authenticate(challenge map[string]interface{}, signature string) (map[string]interface{}, error)
}

type Authenticator interface {
	// Authenticate TODO:
	Authenticate(details map[string]interface{}) (map[string]interface{}, error)
}

type BasicTicketAuthenticator struct {
	ticket string
}

func (t *BasicTicketAuthenticator) Challenge(details map[string]interface{}) (map[string]interface{}, error) {
	return make(map[string]interface{}), nil
}

func (t *BasicTicketAuthenticator) Authenticate(challenge map[string]interface{}, signature string) (map[string]interface{}, error) {
	if signature != t.ticket {
		return nil, fmt.Errorf("Invalid ticket")
	}
	return nil, nil
}

func NewBasicTicketAuthenticator(ticket string) *BasicTicketAuthenticator {
	return &BasicTicketAuthenticator{ticket: ticket}
}
