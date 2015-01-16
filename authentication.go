package turnpike

import (
	"fmt"
)

// CRAuthenticator describes a type that can handle challenge/response authentication.
type CRAuthenticator interface {
	// accept HELLO details and returns a challenge map (which will be sent in a CHALLENGE message)
	Challenge(details map[string]interface{}) (map[string]interface{}, error)
	// accept a challenge map (same as was generated in Challenge) and a signature string, and
	// authenticates the signature string against the challenge. Returns a details map and error.
	Authenticate(challenge map[string]interface{}, signature string) (map[string]interface{}, error)
}

// Authenticator describes a type that can handle authentication based solely on the HELLO message.
//
// Use CRAuthenticator for more complex authentication schemes.
type Authenticator interface {
	// Authenticate takes the HELLO details and returns a (WELCOME) details map if the
	// authentication is successful, otherwise it returns an error
	Authenticate(details map[string]interface{}) (map[string]interface{}, error)
}

type basicTicketAuthenticator struct {
	tickets map[string]bool
}

func (t *basicTicketAuthenticator) Challenge(details map[string]interface{}) (map[string]interface{}, error) {
	return make(map[string]interface{}), nil
}

func (t *basicTicketAuthenticator) Authenticate(challenge map[string]interface{}, signature string) (map[string]interface{}, error) {
	if !t.tickets[signature] {
		return nil, fmt.Errorf("Invalid ticket")
	}
	return nil, nil
}

// NewBasicTicketAuthenticator creates a basic ticket authenticator from a static set of valid tickets.
//
// This method of ticket-based authentication is insecure, but useful for bootstrapping.
// Do not use this in production.
func NewBasicTicketAuthenticator(tickets ...string) CRAuthenticator {
	authenticator := &basicTicketAuthenticator{tickets: make(map[string]bool)}
	for _, ticket := range tickets {
		authenticator.tickets[ticket] = true
	}
	return authenticator
}
