package turnpike

// A broker handles routing EVENTS from Publishers to Subscribers.
type Broker interface {
	// Publishes a message to all Subscribers.
	Publish(Session, *Publish)
	// Subscribes to messages on a URI.
	Subscribe(Session, *Subscribe)
	// Unsubscribes from messages on a URI.
	Unsubscribe(Session, *Unsubscribe)
}

// A super simple broker that matches URIs to Subscribers.
type defaultBroker struct {
	routes        map[URI]map[ID]Session
	subscriptions map[ID]URI
}

// NewDefaultBroker initializes and returns a simple broker that matches URIs to Subscribers.
func NewDefaultBroker() Broker {
	return &defaultBroker{
		routes:        make(map[URI]map[ID]Session),
		subscriptions: make(map[ID]URI),
	}
}

// Publish sends a message to all subscribed clients except for the sender.
//
// If msg.Options["acknowledge"] == true, the publisher receives a Published event
// after the message has been sent to all subscribers.
func (br *defaultBroker) Publish(pub Session, msg *Publish) {
	pubId := NewID()
	evtTemplate := Event{
		Publication: pubId,
		Arguments:   msg.Arguments,
		ArgumentsKw: msg.ArgumentsKw,
		Details:     make(map[string]interface{}),
	}
	for id, sub := range br.routes[msg.Topic] {
		// shallow-copy the template
		event := evtTemplate
		event.Subscription = id
		// don't send event to publisher
		if sub.Id != pub.Id {
			sub.Send(&event)
		}
	}

	// only send published message if acknowledge is present and set to true
	if doPub, _ := msg.Options["acknowledge"].(bool); doPub {
		pub.Send(&Published{Request: msg.Request, Publication: pubId})
	}
}

// Subscribe subscribes the client to the given topic.
func (br *defaultBroker) Subscribe(session Session, msg *Subscribe) {
	if _, ok := br.routes[msg.Topic]; !ok {
		br.routes[msg.Topic] = make(map[ID]Session)
	}
	id := NewID()
	br.routes[msg.Topic][id] = session

	br.subscriptions[id] = msg.Topic

	session.Peer.Send(&Subscribed{Request: msg.Request, Subscription: id})
}

func (br *defaultBroker) Unsubscribe(session Session, msg *Unsubscribe) {
	topic, ok := br.subscriptions[msg.Subscription]
	if !ok {
		err := &Error{
			Type:    msg.MessageType(),
			Request: msg.Request,
			Error:   WAMP_ERROR_NO_SUCH_SUBSCRIPTION,
		}
		session.Peer.Send(err)
		return
	}
	delete(br.subscriptions, msg.Subscription)

	if r, ok := br.routes[topic]; !ok {
		err := &Error{
			Type:    msg.MessageType(),
			Request: msg.Request,
			Error:   URI("wamp.error.internal_error"),
		}
		session.Peer.Send(err)
	} else if _, ok := r[msg.Subscription]; !ok {
		err := &Error{
			Type:    msg.MessageType(),
			Request: msg.Request,
			Error:   URI("wamp.error.internal_error"),
		}
		session.Peer.Send(err)
	} else {
		delete(r, msg.Subscription)
		if len(r) == 0 {
			delete(br.routes, topic)
		}
		session.Peer.Send(&Unsubscribed{Request: msg.Request})
	}
}
