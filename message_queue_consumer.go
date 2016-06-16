package main

import (
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	queueConsumer "github.com/Financial-Times/message-queue-gonsumer/consumer"
)

func (app notificationsApp) consumeMessages() {
	consumer := queueConsumer.NewConsumer(*app.consumerConfig, app.receiveEvents, http.Client{})

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		infoLogger.Println("Started consuming.")
		consumer.Start()
		infoLogger.Println("Finished consuming.")
		wg.Done()
	}()

	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	infoLogger.Println("Termination signal received. Quitting consumeMessages function.")
	consumer.Stop()
	wg.Wait()
}

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
		app.notifications.enqueue(uppN)
	}()
}
