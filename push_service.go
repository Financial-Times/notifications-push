package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	stdlog "log"
	"net/http"
	"regexp"
	"time"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/kafka-client-go/kafka"
	queueConsumer "github.com/Financial-Times/notifications-push/v5/consumer"
	"github.com/Financial-Times/notifications-push/v5/dispatch"
	"github.com/Financial-Times/notifications-push/v5/resources"
	"github.com/Financial-Times/service-status-go/httphandlers"
	"github.com/gorilla/mux"
	"github.com/samuel/go-zookeeper/zk"
	"github.com/wvanbergen/kazoo-go"
)

type notificationSystem interface {
	Start()
	Stop()
}

func startService(srv *http.Server, n notificationSystem, consumer kafka.Consumer, msgHandler queueConsumer.MessageQueueHandler, log *logger.UPPLogger) func(time.Duration) {
	go n.Start()

	consumer.StartListening(msgHandler.HandleMessage)

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
		consumer.Shutdown()
		n.Stop()
	}
}

func initRouter(r *mux.Router,
	s *resources.SubHandler,
	resource string,
	d *dispatch.Dispatcher,
	h *resources.HistoryHandler,
	hc *resources.HealthCheck,
	log *logger.UPPLogger) {
	r.HandleFunc("/"+resource+"/notifications-push", s.HandleSubscription).Methods("GET")

	r.HandleFunc("/__health", hc.Health())
	r.HandleFunc(httphandlers.GTGPath, httphandlers.NewGoodToGoHandler(hc.GTG))
	r.HandleFunc(httphandlers.BuildInfoPath, httphandlers.BuildInfoHandler)
	r.HandleFunc(httphandlers.PingPath, httphandlers.PingHandler)

	r.HandleFunc("/__stats", resources.Stats(d, log)).Methods("GET")
	r.HandleFunc("/__history", h.History).Methods("GET")
}

type supervisedConsumer struct {
	c     kafka.Consumer
	errCh chan error
}

func (s *supervisedConsumer) StartListening(messageHandler func(message kafka.FTMessage) error) {
	s.c.StartListening(messageHandler)
}

func (s *supervisedConsumer) Shutdown() {
	close(s.errCh)
	s.c.Shutdown()
}

func (s *supervisedConsumer) ConnectivityCheck() error {
	return s.c.ConnectivityCheck()
}

func createSupervisedConsumer(log *logger.UPPLogger, address string, groupID string, topics []string) (*supervisedConsumer, error) {
	errCh := make(chan error, 2)
	var fatalErrs = []error{kazoo.ErrPartitionNotClaimed, zk.ErrNoServer}
	fatalErrHandler := func(err error, serviceName string) {
		log.WithError(err).Fatalf("Exiting %s due to fatal error", serviceName)
	}

	supervisor := newServiceSupervisor(serviceName, errCh, fatalErrs, fatalErrHandler)
	go supervisor.Supervise()

	consumerConfig := kafka.DefaultConsumerConfig()
	consumerConfig.Zookeeper.Logger = stdlog.New(ioutil.Discard, "", 0)
	c, err := kafka.NewConsumer(kafka.Config{
		ZookeeperConnectionString: address,
		ConsumerGroup:             groupID,
		Topics:                    topics,
		ConsumerGroupConfig:       consumerConfig,
		Err:                       errCh,
	})
	if err != nil {
		return nil, err
	}
	return &supervisedConsumer{c: c, errCh: errCh}, nil
}

func createDispatcher(cacheDelay int, historySize int, log *logger.UPPLogger) (*dispatch.Dispatcher, dispatch.History) {
	history := dispatch.NewHistory(historySize)
	dispatcher := dispatch.NewDispatcher(time.Duration(cacheDelay)*time.Second, history, log)
	return dispatcher, history
}

type msgHandlerCfg struct {
	Resource        string
	BaseURL         string
	ContentURI      string
	ContentTypes    []string
	MetadataHeaders []string
	E2ETestUUIDs    []string
}

func createMessageHandler(config msgHandlerCfg, dispatcher *dispatch.Dispatcher, log *logger.UPPLogger) (*queueConsumer.MessageQueueRouter, error) {
	mapper := queueConsumer.NotificationMapper{
		Resource:   config.Resource,
		APIBaseURL: config.BaseURL,
	}
	whitelistR, err := regexp.Compile(config.ContentURI)
	if err != nil {
		return nil, fmt.Errorf("content whitelist regex MUST compile: %w", err)
	}
	ctWhitelist := queueConsumer.NewSet()
	for _, value := range config.ContentTypes {
		ctWhitelist.Add(value)
	}

	contentHandler := queueConsumer.NewContentQueueHandler(whitelistR, ctWhitelist, config.E2ETestUUIDs, mapper, dispatcher, log)
	metadataHandler := queueConsumer.NewMetadataQueueHandler(config.MetadataHeaders, mapper, dispatcher, log)
	handler := queueConsumer.NewMessageQueueHandler(contentHandler, metadataHandler)
	return handler, nil
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
