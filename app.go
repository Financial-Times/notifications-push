package main

import (
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"regexp"
	"strconv"
	"time"

	log "github.com/Financial-Times/go-logger"
	"github.com/Financial-Times/kafka-client-go/kafka"
	queueConsumer "github.com/Financial-Times/notifications-push/consumer"
	"github.com/Financial-Times/notifications-push/dispatch"
	"github.com/Financial-Times/notifications-push/resources"
	"github.com/Financial-Times/service-status-go/httphandlers"
	"github.com/gorilla/mux"
	cli "github.com/jawher/mow.cli"
	"github.com/samuel/go-zookeeper/zk"
	"github.com/wvanbergen/kazoo-go"
)

const (
	heartbeatPeriod = 30 * time.Second
	serviceName     = "notifications-push"
	appDescription  = "Proactively notifies subscribers about new publishes/modifications."
)

func main() {
	app := cli.App(serviceName, appDescription)
	resource := app.String(cli.StringOpt{
		Name:   "notifications_resource",
		Value:  "",
		Desc:   "The resource of which notifications are produced (e.g., content or lists)",
		EnvVar: "NOTIFICATIONS_RESOURCE",
	})

	consumerAddrs := app.String(cli.StringOpt{
		Name:   "consumer_addr",
		Value:  "",
		Desc:   "Comma separated kafka hosts for message consuming.",
		EnvVar: "KAFKA_ADDRS",
	})

	consumerGroupID := app.String(cli.StringOpt{
		Name:   "consumer_group_id",
		Value:  "",
		Desc:   "Kafka qroup id used for message consuming.",
		EnvVar: "GROUP_ID",
	})
	apiBaseURL := app.String(cli.StringOpt{
		Name:   "api_base_url",
		Value:  "http://api.ft.com",
		Desc:   "The API base URL where resources are accessible",
		EnvVar: "API_BASE_URL",
	})
	apiKeyValidationEndpoint := app.String(cli.StringOpt{
		Name:   "api_key_validation_endpoint",
		Value:  "t800/a",
		Desc:   "The API Gateway ApiKey validation endpoint",
		EnvVar: "API_KEY_VALIDATION_ENDPOINT",
	})
	topic := app.String(cli.StringOpt{
		Name:   "topic",
		Value:  "",
		Desc:   "Kafka topic to read from.",
		EnvVar: "TOPIC",
	})
	port := app.Int(cli.IntOpt{
		Name:   "port",
		Value:  8080,
		Desc:   "application port",
		EnvVar: "PORT",
	})
	historySize := app.Int(cli.IntOpt{
		Name:   "notification_history_size",
		Value:  200,
		Desc:   "the number of recent notifications to be saved and returned on the /__history endpoint",
		EnvVar: "NOTIFICATION_HISTORY_SIZE",
	})
	delay := app.Int(cli.IntOpt{
		Name:   "notifications_delay",
		Value:  30,
		Desc:   "The time to delay each notification before forwarding to any subscribers (in seconds).",
		EnvVar: "NOTIFICATIONS_DELAY",
	})
	contentURIWhitelist := app.String(cli.StringOpt{
		Name:   "content_uri_whitelist",
		Desc:   `The contentURI whitelist for incoming notifications - i.e. ^http://.*-transformer-(pr|iw)-uk-.*\.svc\.ft\.com(:\d{2,5})?/content/[\w-]+.*$`,
		EnvVar: "CONTENT_URI_WHITELIST",
	})
	contentTypeWhitelist := app.Strings(cli.StringsOpt{
		Name:   "content_type_whitelist",
		Value:  []string{},
		Desc:   `Comma-separated list of whitelisted ContentTypes for incoming notifications - i.e. application/vnd.ft-upp-article+json,application/vnd.ft-upp-audio+json`,
		EnvVar: "CONTENT_TYPE_WHITELIST",
	})

	logLevel := app.String(cli.StringOpt{
		Name:   "logLevel",
		Value:  "INFO",
		Desc:   "Logging level (DEBUG, INFO, WARN, ERROR)",
		EnvVar: "LOG_LEVEL",
	})

	log.InitLogger(serviceName, *logLevel)
	log.WithFields(map[string]interface{}{
		"KAFKA_TOPIC": *topic,
		"GROUP_ID":    *consumerGroupID,
		"KAFKA_ADDRS": *consumerAddrs,
	}).Infof("[Startup] notifications-push is starting ")

	app.Action = func() {
		errCh := make(chan error, 2)
		defer close(errCh)
		var fatalErrs = []error{kazoo.ErrPartitionNotClaimed, zk.ErrNoServer}
		fatalErrHandler := func(err error, serviceName string) {
			log.WithError(err).Fatalf("Exiting %s due to fatal error", serviceName)
		}

		supervisor := newServiceSupervisor(serviceName, errCh, fatalErrs, fatalErrHandler)
		go supervisor.Supervise()

		consumerConfig := kafka.DefaultConsumerConfig()
		messageConsumer, err := kafka.NewConsumer(kafka.Config{
			ZookeeperConnectionString: *consumerAddrs,
			ConsumerGroup:             *consumerGroupID,
			Topics:                    []string{*topic},
			ConsumerGroupConfig:       consumerConfig,
			Err:                       errCh,
		})
		if err != nil {
			log.WithError(err).Fatal("Cannot create Kafka client")
		}

		httpClient := &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				MaxIdleConnsPerHost:   20,
				TLSHandshakeTimeout:   3 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		}

		history := dispatch.NewHistory(*historySize)
		dispatcher := dispatch.NewDispatcher(time.Duration(*delay)*time.Second, heartbeatPeriod, history)

		mapper := queueConsumer.NotificationMapper{
			Resource:   *resource,
			APIBaseURL: *apiBaseURL,
		}

		whitelistR, err := regexp.Compile(*contentURIWhitelist)
		if err != nil {
			log.WithError(err).Fatal("Whitelist regex MUST compile!")
		}

		apiGatewayKeyValidationURL := fmt.Sprintf("%s/%s", *apiBaseURL, *apiKeyValidationEndpoint)
		go server(":"+strconv.Itoa(*port), *resource, dispatcher, history, messageConsumer, apiGatewayKeyValidationURL, httpClient)

		ctWhitelist := queueConsumer.NewSet()
		for _, value := range *contentTypeWhitelist {
			ctWhitelist.Add(value)
		}
		queueHandler := queueConsumer.NewMessageQueueHandler(whitelistR, ctWhitelist, mapper, dispatcher)
		pushService := newPushService(dispatcher, messageConsumer)
		pushService.start(queueHandler)
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func server(listen string, resource string, dispatcher dispatch.Dispatcher, history dispatch.History, consumer kafka.Consumer, apiGatewayKeyValidationURL string, httpClient *http.Client) {
	notificationsPushPath := "/" + resource + "/notifications-push"

	r := mux.NewRouter()

	r.HandleFunc(notificationsPushPath, resources.Push(dispatcher, apiGatewayKeyValidationURL, httpClient)).Methods("GET")
	r.HandleFunc("/__history", resources.History(history)).Methods("GET")
	r.HandleFunc("/__stats", resources.Stats(dispatcher)).Methods("GET")

	healthCheckHttpClient := &resources.HttpClient{}
	hc := resources.NewHealthCheck(consumer, apiGatewayKeyValidationURL, healthCheckHttpClient)

	r.HandleFunc("/__health", hc.Health())
	r.HandleFunc(httphandlers.GTGPath, httphandlers.NewGoodToGoHandler(hc.GTG))
	r.HandleFunc(httphandlers.BuildInfoPath, httphandlers.BuildInfoHandler)
	r.HandleFunc(httphandlers.PingPath, httphandlers.PingHandler)

	http.Handle("/", r)

	err := http.ListenAndServe(listen, nil)
	log.Fatal(err)
}
