package main

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/Financial-Times/message-queue-gonsumer/consumer"
)

type eventDispatcher struct {
	incoming         chan string
	subscribers      map[chan string]subscriber
	addSubscriber    chan subscriberEvent
	removeSubscriber chan subscriberEvent
}

func newEvents() *eventDispatcher {
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

type subscriberEvent struct {
	ch         chan string
	subscriber subscriber
}

type subscriber struct {
	Addr  string
	Since time.Time
}

var whitelist = regexp.MustCompile("^http://(methode-article|wordpress-article)-transformer-(pr|iw)-uk-.*\\.svc\\.ft\\.com(:\\d{2,5})?/(content)/[\\w-]+.*$")

func (d eventDispatcher) receiveEvents(msg consumer.Message) {
	if strings.HasPrefix(msg.Headers["X-Request-Id"], "SYNTH") {
		return
	}
	var cmsPubEvent cmsPublicationEvent
	err := json.Unmarshal([]byte(msg.Body), &cmsPubEvent)
	if err != nil {
		warnLogger.Printf("Skipping cmsPublicationEvent [%v]: [%v].", msg.Body, err)
		return
	}
	if !whitelist.MatchString(cmsPubEvent.ContentURI) {
		infoLogger.Printf("Skipping msg with contentUri [%v]", cmsPubEvent.ContentURI)
		return
	}

	//infoLogger.Printf("CmsPublicationEvent with tid [%v] is valid.", msg.Headers["X-Request-Id"])
	n := buildNotification(cmsPubEvent)
	if n == nil {
		warnLogger.Printf("Skipping. Cannot build notification for msg: [%#v]", cmsPubEvent)
		return
	}
	bytes, err := json.Marshal([]*notification{n})
	if err != nil {
		warnLogger.Printf("Skipping notification [%#v]: [%v]", n, err)
		return
	}

	go func() {
		//wait 30sec for the content to be ingested before notifying the clients
		time.Sleep(30 * time.Second)

		d.incoming <- string(bytes[:])
	}()
}

func buildNotification(cmsPubEvent cmsPublicationEvent) *notification {
	if cmsPubEvent.UUID == "" {
		return nil
	}

	eventType := "UPDATE"
	if len(cmsPubEvent.Payload) == 0 {
		eventType = "DELETE"
	}
	return &notification{
		Type:   "http://www.ft.com/thing/ThingChangeType/" + eventType,
		ID:     "http://www.ft.com/thing/" + cmsPubEvent.UUID,
		APIURL: "http://api.ft.com/content/" + cmsPubEvent.UUID,
	}
}

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
			log.Printf("New subscriber [%s].", s.subscriber.Addr)
			d.subscribers[s.ch] = s.subscriber
			s.ch <- heartbeatMsg
		case s := <-d.removeSubscriber:
			delete(d.subscribers, s.ch)
			log.Printf("Subscriber [%s] left.", s.subscriber)
		}
	}
}

const heartbeatMsg = "[]"
const heartbeatPeriod = 30

func resetTimer(timer *time.Timer) {
	timer.Reset(heartbeatPeriod * time.Second)
}

func (s subscriber) String() string {
	return fmt.Sprintf("Addr: [%s]. Since: [%s]. Connection duration: [%s].", s.Addr, s.Since.Format(time.StampMilli), time.Since(s.Since))
}

func (s subscriber) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`{ "addr" : "%s", "since" : "%s", "connectionDuration": "%s" }`, s.Addr, s.Since.Format(time.StampMilli), time.Since(s.Since))), nil
}
