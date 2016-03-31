package turnpike

import (
	"sync"
)

// Broker is the interface implemented by an object that handles routing EVENTS
// from Publishers to Subscribers.
type Broker interface {
	// Publishes a message to all Subscribers.
	Publish(*Session, *Publish)
	// Subscribes to messages on a URI.
	Subscribe(*Session, *Subscribe)
	// Unsubscribes from messages on a URI.
	Unsubscribe(*Session, *Unsubscribe)
}

// A super simple broker that matches URIs to Subscribers.
type defaultBroker struct {
	routes        map[URI]map[ID]Sender
	subscriptions map[ID]URI

	sync.RWMutex
}

// NewDefaultBroker initializes and returns a simple broker that matches URIs to
// Subscribers.
func NewDefaultBroker() Broker {
	return &defaultBroker{
		routes:        make(map[URI]map[ID]Sender),
		subscriptions: make(map[ID]URI),
	}
}

// Publish sends a message to all subscribed clients except for the sender.
//
// If msg.Options["acknowledge"] == true, the publisher receives a Published event
// after the message has been sent to all subscribers.
func (br *defaultBroker) Publish(sess *Session, msg *Publish) {
	br.RLock()
	defer br.RUnlock()

	pub := sess.Peer
	pubID := sess.NextRequestId()
	evtTemplate := Event{
		Publication: pubID,
		Arguments:   msg.Arguments,
		ArgumentsKw: msg.ArgumentsKw,
		Details:     make(map[string]interface{}),
	}

	excludePublisher := true
	if exclude, ok := msg.Options["exclude_me"].(bool); ok {
		excludePublisher = exclude
	}

	for id, sub := range br.routes[msg.Topic] {
		// shallow-copy the template
		event := evtTemplate
		event.Subscription = id
		// don't send event to publisher
		if sub != pub || !excludePublisher {
			sub.Send(&event)
		}
	}

	// only send published message if acknowledge is present and set to true
	if doPub, _ := msg.Options["acknowledge"].(bool); doPub {
		pub.Send(&Published{Request: msg.Request, Publication: pubID})
	}
}

// Subscribe subscribes the client to the given topic.
func (br *defaultBroker) Subscribe(sess *Session, msg *Subscribe) {
	br.Lock()
	defer br.Unlock()

	if _, ok := br.routes[msg.Topic]; !ok {
		br.routes[msg.Topic] = make(map[ID]Sender)
	}
	id := sess.NextRequestId()
	br.routes[msg.Topic][id] = sess.Peer

	br.subscriptions[id] = msg.Topic

	sess.Peer.Send(&Subscribed{Request: msg.Request, Subscription: id})
}

func (br *defaultBroker) Unsubscribe(sess *Session, msg *Unsubscribe) {
	br.Lock()
	defer br.Unlock()

	topic, ok := br.subscriptions[msg.Subscription]
	if !ok {
		err := &Error{
			Type:    msg.MessageType(),
			Request: msg.Request,
			Error:   ErrNoSuchSubscription,
		}
		sess.Peer.Send(err)
		log.Printf("Error unsubscribing: no such subscription %v", msg.Subscription)
		return
	}
	delete(br.subscriptions, msg.Subscription)

	if r, ok := br.routes[topic]; !ok {
		log.Printf("Error unsubscribing: unable to find routes for %s topic", topic)
	} else if _, ok := r[msg.Subscription]; !ok {
		log.Printf("Error unsubscribing: %s route does not exist for %v subscription", topic, msg.Subscription)
	} else {
		delete(r, msg.Subscription)
		if len(r) == 0 {
			delete(br.routes, topic)
		}
	}
	sess.Peer.Send(&Unsubscribed{Request: msg.Request})
}
