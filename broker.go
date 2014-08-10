package turnpike

func spliceSubscribers(subs []Subscriber, i int) []Subscriber {
	if i == len(subs)-1 {
		return subs[:i]
	}
	return append(subs[:i], subs[i+1:]...)
}

type ErrorHandler interface {
	// SendError sends an ERROR message to the client.
	SendError(*Error)
}

type Publisher interface {
	ErrorHandler

	// SendPublished sends acknowledgement that the Event has
	// been successfully published.
	SendPublished(*Published)
}

// A Subscriber can subscribe to messages on a Topic URI.
type Subscriber interface {
	ErrorHandler

	// SendEvent sends a Published Event to the client.
	SendEvent(*Event)
	// SendUnsubscribed sends an acknowledgement that the
	// client has been unsubscribed from messages on the Topic.
	SendUnsubscribed(*Unsubscribed)
	// SendSubscribed sends an acknowledgement that the
	// client has been subscribed to messages on the Topic.
	SendSubscribed(*Subscribed)
}

type Broker interface {
	// Publishes a message to all Subscribers.
	Publish(Publisher, *Publish)
	// Subscribes to messages on a URI.
	Subscribe(Subscriber, *Subscribe)
	// Unsubscribes from messages on a URI.
	Unsubscribe(Subscriber, *Unsubscribe)
}

// A super simple broker that matches URIs to Subscribers.
type DefaultBroker struct {
	routes        map[URI]map[ID]Subscriber
	subscriptions map[ID]URI
}

func NewDefaultBroker() *DefaultBroker {
	return &DefaultBroker{
		routes:        make(map[URI]map[ID]Subscriber),
		subscriptions: make(map[ID]URI),
	}
}

func (br *DefaultBroker) Publish(pub Publisher, msg *Publish) {
	pubId := NewID()
	evtTemplate := Event{
		Publication: pubId,
		Arguments:   msg.Arguments,
		ArgumentsKw: msg.ArgumentsKw,
	}
	for id, sub := range br.routes[msg.Topic] {
		// shallow-copy the template
		event := evtTemplate
		event.Subscription = id
		sub.SendEvent(&event)
		// TODO: publisher should not receive event, even if subscribed.
		// see: https://github.com/tavendo/WAMP/blob/master/spec/basic.md#event-1
	}

	// only send published message if acknowledge is present and set to true
	if doPub, _ := msg.Options["acknowledge"].(bool); doPub {
		pub.SendPublished(&Published{Request: msg.Request, Publication: pubId})
	}
}

func (br *DefaultBroker) Subscribe(sub Subscriber, msg *Subscribe) {
	if _, ok := br.routes[msg.Topic]; !ok {
		br.routes[msg.Topic] = make(map[ID]Subscriber)
	}
	id := NewID()
	br.routes[msg.Topic][id] = sub

	br.subscriptions[id] = msg.Topic

	sub.SendSubscribed(&Subscribed{Request: msg.Request, Subscription: id})
}

func (br *DefaultBroker) Unsubscribe(sub Subscriber, msg *Unsubscribe) {
	topic, ok := br.subscriptions[msg.Subscription]
	if !ok {
		err := &Error{
			Type:    msg.MessageType(),
			Request: msg.Request,
			Error:   WAMP_ERROR_NO_SUCH_SUBSCRIPTION,
		}
		sub.SendError(err)
		return
	}

	if r, ok := br.routes[topic]; !ok {
		err := &Error{
			Type:    msg.MessageType(),
			Request: msg.Request,
			Error:   URI("wamp.error.internal_error"),
		}
		sub.SendError(err)
	} else if _, ok := r[msg.Subscription]; !ok {
		err := &Error{
			Type:    msg.MessageType(),
			Request: msg.Request,
			Error:   URI("wamp.error.internal_error"),
		}
		sub.SendError(err)
	} else {
		delete(r, msg.Subscription)
		if len(r) == 0 {
			delete(br.routes, topic)
		}
		sub.SendUnsubscribed(&Unsubscribed{Request: msg.Request})
	}
}
