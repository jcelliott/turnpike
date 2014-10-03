package turnpike

import (
	"fmt"
	"time"
)

var defaultWelcomeDetails = map[string]interface{}{
	"roles": map[string]struct{}{
		"broker": {},
		"dealer": {},
	},
}

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
	if realm.Broker == nil {
		realm.Broker = NewDefaultBroker()
	}
	if realm.Dealer == nil {
		realm.Dealer = NewDefaultDealer()
	}
	r.realms[uri] = realm
	return nil
}

func (r *DefaultRouter) handleSession(sess Session, realmURI URI) {
	defer sess.Close()

	c := sess.Receive()
	// TODO: what happens if the realm is closed?
	realm := r.realms[realmURI]

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

		switch msg := msg.(type) {
		case *Goodbye:
			sess.Send(&Goodbye{Reason: WAMP_ERROR_GOODBYE_AND_OUT})
			return

		// Broker messages
		case *Publish:
			realm.Broker.Publish(sess.Peer, msg)
		case *Subscribe:
			realm.Broker.Subscribe(sess.Peer, msg)
		case *Unsubscribe:
			realm.Broker.Unsubscribe(sess.Peer, msg)

		// Dealer messages
		case *Register:
			realm.Dealer.Register(sess.Peer, msg)
		case *Unregister:
			realm.Dealer.Unregister(sess.Peer, msg)
		case *Call:
			realm.Dealer.Call(sess.Peer, msg)
		case *Yield:
			realm.Dealer.Yield(sess.Peer, msg)

		default:
			log.Println("Unhandled message:", msg.MessageType())
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
		} else if realm, ok := r.realms[hello.Realm]; !ok {
			// TODO: handle invalid realm more gracefully
			if err := client.Send(&Abort{Reason: WAMP_ERROR_NO_SUCH_REALM}); err != nil {
				return err
			}
			return client.Close()
		} else if welcome, err := r.authenticate(client, realm, hello.Details); err != nil {
			abort := &Abort{
				Reason:  WAMP_ERROR_AUTHORIZATION_FAILED,
				Details: map[string]interface{}{"error": err.Error()},
			}
			if err := client.Send(abort); err != nil {
				return err
			}
			return client.Close()
		} else {
			id := NewID()
			welcome.Id = id

			if welcome.Details == nil {
				welcome.Details = make(map[string]interface{})
			}
			// add default details to welcome message
			for k, v := range defaultWelcomeDetails {
				// TODO: check if key already set
				welcome.Details[k] = v
			}
			if err := client.Send(welcome); err != nil {
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

func (r *DefaultRouter) authenticate(client Peer, realm Realm, details map[string]interface{}) (*Welcome, error) {
	msg, err := realm.Authenticate(details)
	if err != nil {
		return nil, err
	}
	// we should never get anything besides WELCOME and CHALLENGE
	if msg.MessageType() == WELCOME {
		return msg.(*Welcome), nil
	} else {
		// Challenge response
		challenge := msg.(*Challenge)
		if err := client.Send(challenge); err != nil {
			return nil, err
		}

		c := client.Receive()
		select {
		case <-time.After(5 * time.Second):
			return nil, fmt.Errorf("Timeout on receiving messages")
		case msg, open := <-c:
			if !open {
				return nil, fmt.Errorf("No messages received")
			}
			if authenticate, ok := msg.(*Authenticate); !ok {
				return nil, fmt.Errorf("unexpected %s message received", msg.MessageType())
			} else {
				return realm.CheckResponse(challenge, authenticate)
			}
		}
	}
}
