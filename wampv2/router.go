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

type Router interface {
	Accept(Endpoint) error
	Close() error
	RegisterRealm(URI, Realm) error
}

// BasicRouter is a very basic WAMP router.
type BasicRouter struct {
	*BasicBroker

	realms  map[URI]Realm
	clients map[URI][]Session
	closing bool
	lastId  int
}

func NewBasicRouter() *BasicRouter {
	return &BasicRouter{
		BasicBroker: NewBasicBroker(),
		realms:      make(map[URI]Realm),
		clients:     make(map[URI][]Session),
	}
}

func (r *BasicRouter) Close() error {
	r.closing = true
	for _, clients := range r.clients {
		for _, client := range clients {
			client.kill <- WAMP_ERROR_SYSTEM_SHUTDOWN
		}
	}
	return nil
}

func (r *BasicRouter) RegisterRealm(uri URI, realm Realm) error {
	if _, ok := r.realms[uri]; ok {
		return realmExists(uri)
	}
	r.realms[uri] = realm
	return nil
}

func (r *BasicRouter) broker(realm URI) Broker {
	if br := r.realms[realm].Broker(); br != nil {
		return br
	}
	return r
}

func (r *BasicRouter) handleSession(sess Session, realm URI) {
	defer sess.Close()

	c := sess.Receive()

	for {
		var msg Message
		var open bool
		select {
		case msg, open = <-c:
			if !open {
				return
			}
		case reason := <-sess.kill:
			sess.Send(&Goodbye{Reason: reason})
			// TODO: wait for client Goodbye?
			return
		}

		switch v := msg.(type) {
		case *Goodbye:
			sess.Send(&Goodbye{Reason: WAMP_ERROR_GOODBYE_AND_OUT})
			return

		// Broker messages
		case *Publish:
			if pub, ok := sess.Endpoint.(Publisher); ok {
				r.broker(realm).Publish(pub, v)
			} else {
				err := &Error{
					Type:    v.MessageType(),
					Request: v.Request,
					Error:   WAMP_ERROR_NOT_AUTHORIZED,
				}
				sess.Send(err)
			}
		case *Subscribe:
			if sub, ok := sess.Endpoint.(Subscriber); ok {
				r.broker(realm).Subscribe(sub, v)
			} else {
				err := &Error{
					Type:    v.MessageType(),
					Request: v.Request,
					Error:   WAMP_ERROR_NOT_AUTHORIZED,
				}
				sess.Send(err)
			}
		case *Unsubscribe:
			if sub, ok := sess.Endpoint.(Subscriber); ok {
				r.broker(realm).Unsubscribe(sub, v)
			} else {
				err := &Error{
					Type:    v.MessageType(),
					Request: v.Request,
					Error:   WAMP_ERROR_NOT_AUTHORIZED,
				}
				sess.Send(err)
			}

		default:
			fmt.Println("Unhandled message:", v.MessageType())
		}
	}
}

func (r *BasicRouter) Accept(ep Endpoint) error {
	if r.closing {
		ep.Send(&Abort{Reason: WAMP_ERROR_SYSTEM_SHUTDOWN})
		ep.Close()
		return fmt.Errorf("Router is closing, no new connections are allowed")
	}

	c := ep.Receive()

	select {
	case <-time.After(5 * time.Second):
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
			id := NewID()

			// TODO: challenge
			if err := ep.Send(&Welcome{Id: id}); err != nil {
				return err
			}

			sess := Session{Endpoint: ep, Id: id, kill: make(chan URI, 1)}
			r.clients[hello.Realm] = append(r.clients[hello.Realm], sess)
			go r.handleSession(sess, hello.Realm)
		}
	}

	return nil
}
