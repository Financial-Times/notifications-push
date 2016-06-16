package main

import (
	"encoding/json"
	"strings"
	"time"

	queueConsumer "github.com/Financial-Times/message-queue-gonsumer/consumer"
)

func (app notificationsApp) receiveEvents(msg queueConsumer.Message) {
	tid := msg.Headers["X-Request-Id"]
	if strings.HasPrefix(tid, "SYNTH") {
		return
	}
	infoLogger.Printf("Received event: tid=[%v].", tid)
	var cmsPubEvent cmsPublicationEvent
	err := json.Unmarshal([]byte(msg.Body), &cmsPubEvent)
	if err != nil {
		warnLogger.Printf("Skipping event: tid=[%v], msg=[%v]: [%v].", tid, msg.Body, err)
		return
	}
	uuid := cmsPubEvent.UUID
	if !whitelist.MatchString(cmsPubEvent.ContentURI) {
		infoLogger.Printf("Skipping event: tid=[%v]. Invalid contentUri=[%v]", tid, cmsPubEvent.ContentURI)
		return
	}

	n := app.notificationBuilder.buildNotification(cmsPubEvent)
	if n == nil {
		warnLogger.Printf("Skipping event: tid=[%v]. Cannot build notification for msg=[%#v]", tid, cmsPubEvent)
		return
	}
	bytes, err := json.Marshal([]*notification{n})
	if err != nil {
		warnLogger.Printf("Skipping event: tid=[%v]. Notification [%#v]: [%v]", tid, n, err)
		return
	}

	go func() {
		//wait 30sec for the content to be ingested before notifying the clients
		time.Sleep(30 * time.Second)
		infoLogger.Printf("Notifying clients about tid=[%v] uuid=[%v].", tid, uuid)
		app.eventDispatcher.incoming <- string(bytes[:])

		uppN := buildUPPNotification(n, tid, msg.Headers["Message-Timestamp"])
		infoLogger.Println(uppN)

	}()
}

func (nb notificationBuilder) buildNotification(cmsPubEvent cmsPublicationEvent) *notification {
	if cmsPubEvent.UUID == "" {
		return nil
	}

	empty := false
	switch v := cmsPubEvent.Payload.(type) {
	case nil:
		empty = true
	case string:
		if len(v) == 0 {
			empty = true
		}
	case map[string]interface{}:
		if len(v) == 0 {
			empty = true
		}
	}
	eventType := "UPDATE"
	if empty {
		eventType = "DELETE"
	}
	return &notification{
		Type:   "http://www.ft.com/thing/ThingChangeType/" + eventType,
		ID:     "http://www.ft.com/thing/" + cmsPubEvent.UUID,
		APIURL: nb.APIBaseURL + "/content/" + cmsPubEvent.UUID,
	}
}

func buildUPPNotification(n *notification, tid, lastModified string) *notificationUPP {
	return &notificationUPP{
		*n,
		tid,
		lastModified,
	}
}
