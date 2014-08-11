package turnpike

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

type Router interface {
	Accept(Peer) error
	Close() error
	RegisterRealm(URI, Realm) error
}

// DefaultRouter is a very basic WAMP router.
type DefaultRouter struct {
	*DefaultBroker
	*DefaultDealer

	realms  map[URI]Realm
	clients map[URI][]Session
	closing bool
	lastId  int
}

func NewDefaultRouter() *DefaultRouter {
	return &DefaultRouter{
		DefaultBroker: NewDefaultBroker(),
		DefaultDealer: NewDefaultDealer(),
		realms:        make(map[URI]Realm),
		clients:       make(map[URI][]Session),
	}
}

func (r *DefaultRouter) Close() error {
	r.closing = true
	for _, clients := range r.clients {
		for _, client := range clients {
			client.kill <- WAMP_ERROR_SYSTEM_SHUTDOWN
		}
	}
	return nil
}

func (r *DefaultRouter) RegisterRealm(uri URI, realm Realm) error {
	if _, ok := r.realms[uri]; ok {
		return realmExists(uri)
	}
	r.realms[uri] = realm
	return nil
}

func (r *DefaultRouter) broker(realm URI) Broker {
	if br := r.realms[realm].Broker(); br != nil {
		return br
	}
	return r
}

func (r *DefaultRouter) dealer(realm URI) Dealer {
	if d := r.realms[realm].Dealer(); d != nil {
		return d
	}
	return r
}

func (r *DefaultRouter) handleSession(sess Session, realm URI) {
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

		log.Println(sess.Id, msg.MessageType(), msg)

		switch v := msg.(type) {
		case *Goodbye:
			sess.Send(&Goodbye{Reason: WAMP_ERROR_GOODBYE_AND_OUT})
			return

		// Broker messages
		case *Publish:
			r.broker(realm).Publish(sess.Peer, v)
		case *Subscribe:
			r.broker(realm).Subscribe(sess.Peer, v)
		case *Unsubscribe:
			r.broker(realm).Unsubscribe(sess.Peer, v)

		// Dealer messages
		case *Register:
			r.dealer(realm).Register(sess.Peer, v)
		case *Unregister:
			r.dealer(realm).Unregister(sess.Peer, v)
		case *Call:
			r.dealer(realm).Call(sess.Peer, v)
		case *Yield:
			r.dealer(realm).Yield(sess.Peer, v)

		default:
			log.Println("Unhandled message:", v.MessageType())
		}
	}
}

func (r *DefaultRouter) Accept(client Peer) error {
	if r.closing {
		client.Send(&Abort{Reason: WAMP_ERROR_SYSTEM_SHUTDOWN})
		client.Close()
		return fmt.Errorf("Router is closing, no new connections are allowed")
	}

	c := client.Receive()

	select {
	case <-time.After(5 * time.Second):
		client.Close()
		return fmt.Errorf("Timeout on receiving messages")
	case msg, open := <-c:
		if !open {
			client.Close()
			return fmt.Errorf("No messages received")
		}
		if hello, ok := msg.(*Hello); !ok {
			if err := client.Send(&Abort{Reason: WAMP_ERROR_NOT_AUTHORIZED}); err != nil {
				return err
			}
			return client.Close()
		} else if _, ok := r.realms[hello.Realm]; !ok {
			// TODO: handle invalid realm more gracefully
			if err := client.Send(&Abort{Reason: WAMP_ERROR_NO_SUCH_REALM}); err != nil {
				return err
			}
			return client.Close()
		} else {
			id := NewID()

			// TODO: challenge
			if err := client.Send(&Welcome{Id: id}); err != nil {
				return err
			}
			log.Println("Established session:", id)

			sess := Session{Peer: client, Id: id, kill: make(chan URI, 1)}
			r.clients[hello.Realm] = append(r.clients[hello.Realm], sess)
			go r.handleSession(sess, hello.Realm)
		}
	}

	return nil
}
