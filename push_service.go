package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/kafka-client-go/v4"
	"github.com/Financial-Times/notifications-push/v5/access"
	queueConsumer "github.com/Financial-Times/notifications-push/v5/consumer"
	"github.com/Financial-Times/notifications-push/v5/dispatch"
	"github.com/Financial-Times/notifications-push/v5/resources"
	"github.com/Financial-Times/service-status-go/httphandlers"
	"github.com/gorilla/mux"
)

type notificationSystem interface {
	Start()
	Stop()
}

func startService(srv *http.Server, n notificationSystem, consumer *kafka.Consumer, msgHandler queueConsumer.MessageQueueHandler, log *logger.UPPLogger) func(time.Duration) {
	go n.Start()

	go consumer.Start(msgHandler.HandleMessage)

	go func() {
		err := srv.ListenAndServe()
		if err != http.ErrServerClosed {
			log.WithError(err).Error("http server")
		}
	}()

	return func(timeout time.Duration) {
		log.Info("Termination started. Quitting message consumer and notification dispatcher function.")
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		_ = srv.Shutdown(ctx)
		err := consumer.Close()
		if err != nil {
			log.WithError(err).Error("Failed to close kafka consumer")
		}
		n.Stop()
	}
}

func initRouter(r *mux.Router,
	s *resources.SubHandler,
	resource string,
	d *dispatch.Dispatcher,
	h dispatch.History,
	hc *resources.HealthCheck,
	log *logger.UPPLogger) {
	r.HandleFunc("/"+resource+"/notifications-push", s.HandleSubscription).Methods("GET")

	r.HandleFunc("/__health", hc.Health())
	r.HandleFunc(httphandlers.GTGPath, httphandlers.NewGoodToGoHandler(hc.GTG))
	r.HandleFunc(httphandlers.BuildInfoPath, httphandlers.BuildInfoHandler)
	r.HandleFunc(httphandlers.PingPath, httphandlers.PingHandler)

	r.HandleFunc("/__stats", resources.Stats(d, log)).Methods("GET")
	r.HandleFunc("/__history", resources.History(h, log)).Methods("GET")
}

func createConsumer(log *logger.UPPLogger, address, groupID string, topic string, lagTolerance int) (*kafka.Consumer, error) {
	consumerConfig := kafka.ConsumerConfig{
		BrokersConnectionString: address,
		ConsumerGroup:           groupID,
		OffsetFetchInterval:     2 * time.Minute,
		Options:                 kafka.DefaultConsumerOptions(),
	}

	kafkaTopic := []*kafka.Topic{
		kafka.NewTopic(topic, kafka.WithLagTolerance(int64(lagTolerance))),
	}

	return kafka.NewConsumer(consumerConfig, kafkaTopic, log)
}

func createDispatcher(cacheDelay int, historySize int, evaluator *access.Evaluator, log *logger.UPPLogger) (*dispatch.Dispatcher, dispatch.History) {
	history := dispatch.NewHistory(historySize)
	dispatcher := dispatch.NewDispatcher(time.Duration(cacheDelay)*time.Second, history, evaluator, log)
	return dispatcher, history
}

type msgHandlerCfg struct {
	BaseURL              string
	ContentURIAllowList  string
	ContentTypeAllowList []string
	E2ETestUUIDs         []string
	ShouldMonitor        bool
	UpdateEventType      string
	APIUrlResource       string
	IncludeScoop         bool
}

func createMessageHandler(config msgHandlerCfg, dispatcher *dispatch.Dispatcher, log *logger.UPPLogger) (*queueConsumer.QueueHandler, error) {
	mapper := queueConsumer.NotificationMapper{
		APIBaseURL:      config.BaseURL,
		UpdateEventType: config.UpdateEventType,
		APIUrlResource:  config.APIUrlResource,
		IncludeScoop:    config.IncludeScoop,
	}
	allowListR, err := regexp.Compile(config.ContentURIAllowList)
	if err != nil {
		return nil, fmt.Errorf("content allowlist regex MUST compile: %w", err)
	}
	ctAllowList := queueConsumer.NewSet()
	for _, value := range config.ContentTypeAllowList {
		ctAllowList.Add(value)
	}

	return queueConsumer.NewQueueHandler(allowListR, ctAllowList, config.E2ETestUUIDs, config.ShouldMonitor, mapper, dispatcher, log), nil
}

func requestStatusCode(ctx context.Context, url string) (int, error) {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, url, bytes.NewReader([]byte("")))
	if err != nil {
		return 0, fmt.Errorf("error creating request: %w", err)
	}
	client := &http.Client{Timeout: time.Second * 15}
	res, err := client.Do(r)
	if err != nil {
		return 0, fmt.Errorf("error making http request:%w", err)
	}
	defer res.Body.Close()

	return res.StatusCode, nil
}
