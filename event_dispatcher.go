package main

import (
	"fmt"
	"regexp"
	"time"
)

type eventDispatcher struct {
	incoming         chan string
	subscribers      map[chan string]subscriber
	addSubscriber    chan subscriberEvent
	removeSubscriber chan subscriberEvent
}

func newDispatcher() *eventDispatcher {
	incoming := make(chan string)
	subscribers := make(map[chan string]subscriber)
	addSubscriber := make(chan subscriberEvent)
	removeSubscriber := make(chan subscriberEvent)
	return &eventDispatcher{incoming, subscribers, addSubscriber, removeSubscriber}
}

type notification struct {
	APIURL string `json:"apiUrl"`
	ID     string `json:"id"`
	Type   string `json:"type"`
}

type notificationUPP struct {
	notification
	LastModified     string `json:"lastModified"`
	PublishReference string `json:"publishReference"`
}

type subscriberEvent struct {
	ch         chan string
	subscriber subscriber
}

type subscriber struct {
	Addr  string
	Since time.Time
}

var whitelist = regexp.MustCompile("^http://(methode-article|wordpress-article)-transformer-(pr|iw)-uk-.*\\.svc\\.ft\\.com(:\\d{2,5})?/(content)/[\\w-]+.*$")

func (d eventDispatcher) distributeEvents() {
	heartbeat := time.NewTimer(heartbeatPeriod * time.Second)
	for {
		select {
		case msg := <-d.incoming:
			for subCh, sub := range d.subscribers {
				select {
				case subCh <- msg:
				default:
					warnLogger.Printf("Subscriber [%v] lagging behind.", sub)
				}
			}
			resetTimer(heartbeat)
		case <-heartbeat.C:
			for subCh, sub := range d.subscribers {
				select {
				case subCh <- heartbeatMsg:
				default:
					warnLogger.Printf("Subscriber [%v] lagging behind when sending heartbeat.", sub)
				}
			}
			resetTimer(heartbeat)
		case s := <-d.addSubscriber:
			infoLogger.Printf("New subscriber [%s].", s.subscriber.Addr)
			d.subscribers[s.ch] = s.subscriber
			s.ch <- heartbeatMsg
		case s := <-d.removeSubscriber:
			delete(d.subscribers, s.ch)
			infoLogger.Printf("Subscriber left [%s].", s.subscriber)
		}
	}
}

const heartbeatMsg = "[]"
const heartbeatPeriod = 30

func resetTimer(timer *time.Timer) {
	timer.Reset(heartbeatPeriod * time.Second)
}

func (s subscriber) String() string {
	return fmt.Sprintf("Addr=[%s]. Since=[%s]. Connection duration=[%s].", s.Addr, s.Since.Format(time.StampMilli), time.Since(s.Since))
}

func (s subscriber) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`{ "addr" : "%s", "since" : "%s", "connectionDuration": "%s" }`, s.Addr, s.Since.Format(time.StampMilli), time.Since(s.Since))), nil
}
