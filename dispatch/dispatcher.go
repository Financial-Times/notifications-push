package dispatch

import (
	"reflect"
	"sync"
	"time"

	log "github.com/Financial-Times/go-logger"
)

const (
	HeartbeatMsg  = "[]"
	rfc3339Millis = "2006-01-02T15:04:05.000Z07:00"
)

// Dispatcher forwards a new notification onto subscribers.
type Dispatcher interface {
	Start()
	Stop()
	Send(notification ...Notification)
	Subscribers() []Subscriber
	Registrar
}

// Registrar (aka Registrator :smirk:) is the interface for a component that
// manages subscriber registration
type Registrar interface {
	Register(subscriber Subscriber)
	Close(subscriber Subscriber)
}

// NewDispatcher creates and returns a new dispatcher
func NewDispatcher(delay time.Duration, heartbeatPeriod time.Duration, history History) Dispatcher {
	return &dispatcher{
		delay:           delay,
		heartbeatPeriod: heartbeatPeriod,
		inbound:         make(chan Notification),
		subscribers:     map[Subscriber]struct{}{},
		lock:            &sync.RWMutex{},
		history:         history,
		stopChan:        make(chan bool),
	}
}

type dispatcher struct {
	delay           time.Duration
	heartbeatPeriod time.Duration
	inbound         chan Notification
	subscribers     map[Subscriber]struct{}
	lock            *sync.RWMutex
	history         History
	stopChan        chan bool
}

func (d *dispatcher) Start() {
	heartbeat := time.NewTimer(d.heartbeatPeriod)

	for {
		select {
		case notification := <-d.inbound:
			d.forwardToSubscribers(notification)
		case <-heartbeat.C:
			d.heartbeat()
		case <-d.stopChan:
			heartbeat.Stop()
			return
		}

		heartbeat.Reset(d.heartbeatPeriod)
	}
}

func (d *dispatcher) forwardToSubscribers(notification Notification) {
	d.lock.RLock()
	defer d.lock.RUnlock()

	var sent, failed, skipped int
	defer func() {
		if len(d.subscribers) == 0 || sent > 0 || len(d.subscribers) == skipped {
			log.WithMonitoringEvent("NotificationsPush", notification.PublishReference, notification.ContentType).
				WithFields(map[string]interface{}{"resource": notification.APIURL, "sent": sent, "failed": failed, "skipped": skipped}).
				Info("Processed subscribers.")
		} else {
			log.WithFields(map[string]interface{}{"transaction_id": notification.PublishReference, "resource": notification.APIURL, "sent": sent, "failed": failed, "skipped": skipped}).
				Error("Processed subscribers. Failed to send notifications")
		}
	}()

	for sub := range d.subscribers {
		entry := log.WithField("transaction_id", notification.PublishReference).
			WithField("resource", notification.APIURL).
			WithField("subscriberId", sub.Id()).
			WithField("subscriberAddress", sub.Address()).
			WithField("subscriberSince", sub.Since().Format(time.RFC3339))

		if !sub.matchesContentType(notification) {
			skipped++
			entry.Info("Skipping subscriber.")
			continue
		}

		err := sub.send(notification)
		if err != nil {
			failed++
			entry.WithError(err).Warn("Failed forwarding to subscriber.")
		} else {
			sent++
			entry.Info("Forwarding to subscriber.")
		}
	}

	d.history.Push(notification)
}

func (d *dispatcher) heartbeat() {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for sub := range d.subscribers {
		sub.writeOnMsgChannel(HeartbeatMsg)
	}
}

func (d *dispatcher) Stop() {
	d.stopChan <- true
}

func (d *dispatcher) Send(notifications ...Notification) {
	log.WithField("batchSize", len(notifications)).Infof("Received notifications batch. Waiting configured delay (%v).", d.delay)
	go func() {
		d.delayForCache()
		for _, n := range notifications {
			n.NotificationDate = time.Now().Format(rfc3339Millis)
			d.inbound <- n
		}
	}()
}

func (d *dispatcher) delayForCache() {
	time.Sleep(d.delay)
}

func (d *dispatcher) Register(subscriber Subscriber) {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.subscribers[subscriber] = struct{}{}

	log.WithField("subscriberId", subscriber.Id()).
		WithField("subscriber", subscriber.Address()).
		WithField("subscriberType", reflect.TypeOf(subscriber).Elem().Name()).
		WithField("acceptedContentType", subscriber.AcceptedContentType()).
		Info("Registered new subscriber")

	subscriber.writeOnMsgChannel(HeartbeatMsg)
}

func (d *dispatcher) Subscribers() []Subscriber {
	d.lock.RLock()
	defer d.lock.RUnlock()

	var subs []Subscriber
	for sub := range d.subscribers {
		subs = append(subs, sub)
	}
	return subs
}

func (d *dispatcher) Close(subscriber Subscriber) {
	d.lock.Lock()
	defer d.lock.Unlock()

	delete(d.subscribers, subscriber)
	log.WithField("subscriberId", subscriber.Id()).
		WithField("subscriber", subscriber.Address()).
		WithField("subscriberType", reflect.TypeOf(subscriber).Elem().Name()).
		Info("Unregistered subscriber")
}
