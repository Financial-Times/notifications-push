package main

import (
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

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
