package wampv2

import "fmt"
import "time"

type realmExists string

func (e realmExists) Error() string {
	return "Realm exists: " + string(e)
}

type unexpectedMessage struct {
	rec MessageType
	exp MessageType
}

func (e unexpectedMessage) Error() string {
	return fmt.Sprintf("Unexpected message: %s; expected %s", e.rec, e.exp)
}

type client struct {
	kill chan<- URI
}

// Router is a very basic WAMP router.
type Router struct {
	*BasicBroker

	realms  map[URI]Realm
	clients map[URI][]client
	closing bool
}

func NewRouter() *Router {
	return &Router{
		BasicBroker: NewBasicBroker(),
		realms:      make(map[URI]Realm),
		clients:     make(map[URI][]client),
	}
}

func (r *Router) Close() error {
	r.closing = true
	for _, clients := range r.clients {
		for _, client := range clients {
			client.kill <- WAMP_ERROR_SYSTEM_SHUTDOWN
		}
	}
	return nil
}

func (r *Router) RegisterRealm(uri URI, realm Realm) error {
	if _, ok := r.realms[uri]; ok {
		return realmExists(uri)
	}
	r.realms[uri] = realm
	return nil
}

func (r *Router) Broker(realm URI) Broker {
	br := r.realms[realm].Broker()
	if br == nil {
		br = r
	}
	return br
}

func (r *Router) handleEP(ep Endpoint, realm URI, kill <-chan URI) {
	defer ep.Close()

	c := ep.Receive()

	for {
		var msg Message
		var open bool
		select {
		case msg, open = <-c:
			if !open {
				return
			}
		case reason := <-kill:
			ep.Send(&Goodbye{Reason: reason})
			// TODO: wait for client Goodbye?
			return
		}

		switch v := msg.(type) {
		case *Goodbye:
			ep.Send(&Goodbye{Reason: WAMP_ERROR_GOODBYE_AND_OUT})
			return

		// Broker messages
		case *Publish:
			if pub, ok := ep.(Publisher); ok {
				r.Broker(realm).Publish(pub, v)
			} else {
				err := &Error{
					Type:    v.MessageType(),
					Request: v.Request,
					Error:   WAMP_ERROR_NOT_AUTHORIZED,
				}
				ep.Send(err)
			}
		case *Subscribe:
			if sub, ok := ep.(Subscriber); ok {
				r.Broker(realm).Subscribe(sub, v)
			} else {
				err := &Error{
					Type:    v.MessageType(),
					Request: v.Request,
					Error:   WAMP_ERROR_NOT_AUTHORIZED,
				}
				ep.Send(err)
			}
		case *Unsubscribe:
			if sub, ok := ep.(Subscriber); ok {
				r.Broker(realm).Unsubscribe(sub, v)
			} else {
				err := &Error{
					Type:    v.MessageType(),
					Request: v.Request,
					Error:   WAMP_ERROR_NOT_AUTHORIZED,
				}
				ep.Send(err)
			}

		default:
			fmt.Println("Unhandled message:", v.MessageType())
		}
	}
}

func (r *Router) Accept(ep Endpoint) error {
	if r.closing {
		ep.Send(&Abort{Reason: WAMP_ERROR_SYSTEM_SHUTDOWN})
		ep.Close()
		return fmt.Errorf("Router is closing, no new connections are allowed")
	}

	c := ep.Receive()

	select {
	case <-time.After(5*time.Second):
		ep.Close()
		return fmt.Errorf("Timeout on receiving messages")
	case msg, open := <-c:
		if !open {
			ep.Close()
			return fmt.Errorf("No messages received")
		}
		if hello, ok := msg.(*Hello); !ok {
			if err := ep.Send(&Abort{Reason: WAMP_ERROR_NOT_AUTHORIZED}); err != nil {
				return err
			}
			return ep.Close()
		} else if _, ok := r.realms[hello.Realm]; !ok {
			// TODO: handle invalid realm more gracefully
			if err := ep.Send(&Abort{Reason: WAMP_ERROR_NO_SUCH_REALM}); err != nil {
				return err
			}
			return ep.Close()
		} else {
			// TODO: challenge
			if err := ep.Send(&Welcome{Id: NewID()}); err != nil {
				return err
			}

			kill := make(chan URI, 1)
			r.clients[hello.Realm] = append(r.clients[hello.Realm], client{kill: kill})
			go r.handleEP(ep, hello.Realm, kill)
		}
	}

	return nil
}
