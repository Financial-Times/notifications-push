package dispatch

import (
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/notifications-push/v5/access"
)

const (
	RFC3339Millis = "2006-01-02T15:04:05.000Z07:00"
)

// NewDispatcher creates and returns a new Dispatcher
// Delay argument configures minimum delay between send notifications
// History is a system that collects a list of all notifications send by Dispatcher
func NewDispatcher(delay time.Duration, history History, opaAgent access.Agent, log *logger.UPPLogger) *Dispatcher {
	return &Dispatcher{
		delay:       delay,
		inbound:     make(chan NotificationModel),
		subscribers: map[NotificationConsumer]struct{}{},
		lock:        &sync.RWMutex{},
		history:     history,
		opaAgent:    opaAgent,
		stopChan:    make(chan bool),
		log:         log,
	}
}

type Dispatcher struct {
	delay       time.Duration
	inbound     chan NotificationModel
	subscribers map[NotificationConsumer]struct{}
	lock        *sync.RWMutex
	history     History
	opaAgent    access.Agent
	stopChan    chan bool
	log         *logger.UPPLogger
}

func (d *Dispatcher) Start() {
	for {
		select {
		case notification := <-d.inbound:
			d.forwardToSubscribers(notification)
			d.history.Push(notification)
		case <-d.stopChan:
			return
		}
	}
}

func (d *Dispatcher) Stop() {
	d.stopChan <- true
}

func (d *Dispatcher) Send(n NotificationModel) {
	d.log.WithTransactionID(n.PublishReference).Infof("Received notification. Waiting configured delay (%v).", d.delay)
	go func() {
		time.Sleep(d.delay)
		n.NotificationDate = time.Now().Format(RFC3339Millis)
		d.inbound <- n
	}()
}

func (d *Dispatcher) Subscribers() []Subscriber {
	d.lock.RLock()
	defer d.lock.RUnlock()

	var subs []Subscriber
	for sub := range d.subscribers {
		subs = append(subs, sub)
	}
	return subs
}

func (d *Dispatcher) Subscribe(address string, subTypes []string, monitoring bool, options *access.NotificationSubscriptionOptions) (Subscriber, error) {
	var s NotificationConsumer
	var err error
	if monitoring {
		s, err = NewMonitorSubscriber(address, subTypes, options)
	} else {
		s, err = NewStandardSubscriber(address, subTypes, options)
	}

	if err != nil {
		return nil, err
	}

	d.addSubscriber(s)

	return s, nil
}

func (d *Dispatcher) Unsubscribe(subscriber Subscriber) {
	d.lock.Lock()
	defer d.lock.Unlock()

	s := subscriber.(NotificationConsumer)

	delete(d.subscribers, s)

	logWithSubscriber(d.log, s).Info("Unregistered subscriber")
}

func (d *Dispatcher) addSubscriber(s NotificationConsumer) {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.subscribers[s] = struct{}{}
	logWithSubscriber(d.log, s).Info("Registered new subscriber")
}

func logWithSubscriber(log *logger.UPPLogger, s Subscriber) *logger.LogEntry {
	return log.WithFields(map[string]interface{}{
		"subscriberId":        s.ID(),
		"subscriberAddress":   s.Address(),
		"subscriberType":      reflect.TypeOf(s).Elem().Name(),
		"subscriberSince":     s.Since().Format(time.RFC3339),
		"acceptedContentType": s.SubTypes(),
	})
}

func (d *Dispatcher) forwardToSubscribers(notification NotificationModel) {
	d.lock.RLock()
	defer d.lock.RUnlock()

	var sent, failed, skipped int
	defer func() {
		entry := d.log.
			WithTransactionID(notification.PublishReference).
			WithFields(map[string]interface{}{
				"resource": notification.APIURL,
				"sent":     sent,
				"failed":   failed,
				"skipped":  skipped,
			})
		if len(d.subscribers) == 0 || sent > 0 || len(d.subscribers) == skipped {
			entry.WithMonitoringEvent("NotificationsPush", notification.PublishReference, notification.SubscriptionType).
				Info("Processed subscribers.")
		} else {
			entry.Error("Processed subscribers. Failed to send notifications")
		}
	}()

	publication := ""
	if notification.Publication != nil {
		pu, pubErr := notification.Publication.OnlyOneOrPink()
		if pubErr != nil {
			d.log.
				WithTransactionID(notification.PublishReference).
				WithField("resource", notification.APIURL).
				WithError(pubErr).
				Warn("Failed to evaluate notification")
			return
		}
		publication = pu
	}
	evaluationResult, err := d.opaAgent.EvaluateContentPolicy(map[string]interface{}{
		"EditorialDesk": notification.EditorialDesk,
		"Publication":   publication,
	})
	if err != nil {
		d.log.
			WithTransactionID(notification.PublishReference).
			WithField("resource", notification.APIURL).
			WithError(err).
			Warn("Failed to evaluate OPA notifications-push policy")
		return
	}
	hasAccess := evaluationResult.Allow

	isRelatedContent := notification.Type == RelatedContentType
	for sub := range d.subscribers {
		entry := logWithSubscriber(d.log, sub).
			WithTransactionID(notification.PublishReference).
			WithField("resource", notification.APIURL)

		if notification.IsE2ETest {
			if _, isStandard := sub.(*StandardSubscriber); isStandard {
				skipped++
				entry.Info("Test notification. Skipping standard subscriber.")
				continue
			}
		} else {
			if !matchesSubType(notification, sub) {
				skipped++
				entry.Info("Skipping subscriber due to subscription type mismatch.")
				continue
			}
			if !hasAccess {
				skipped++
				entry.Info("Skipping subscriber due to ", strings.Join(evaluationResult.Reasons[:], ", "))
				continue
			}
			if isRelatedContent && !sub.Options().ReceiveInternalUnstable {
				skipped++
				entry.Info("Skipping subscriber due to RELATEDCONTENТ notification, without policy InternalUnstable.")
				continue
			}
		}
		nr := CreateNotificationResponse(notification, sub.Options())
		if err = sub.Send(nr); err != nil {
			failed++
			entry.WithError(err).Warn("Failed forwarding to subscriber.")
		} else {
			sent++
			entry.Info("Forwarding to subscriber.")
		}
	}
}

// matchesSubType matches subscriber's ContentType with the incoming contentType notification.
func matchesSubType(n NotificationModel, s Subscriber) bool {
	subTypes := make(map[string]bool)
	for _, subType := range s.SubTypes() {
		subTypes[strings.ToLower(subType)] = true
	}

	notificationType := strings.ToLower(n.SubscriptionType)

	if n.Type == ContentDeleteType && notificationType == "" {
		return true
	}

	return subTypes[notificationType]
}
