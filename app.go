package main

import (
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strconv"
	"strings"

	queueConsumer "github.com/Financial-Times/message-queue-gonsumer/consumer"
	"github.com/jawher/mow.cli"
)

const logPattern = log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile | log.LUTC

var infoLogger *log.Logger
var warnLogger *log.Logger
var errorLogger *log.Logger

type notificationsApp struct {
	eventDispatcher     *eventDispatcher
	consumerConfig      *queueConsumer.QueueConfig
	notificationBuilder notificationBuilder
	notificationsCache  queue
}

func main() {
	app := cli.App("notifications-push", "Proactively notifies subscribers about new content publishes/modifications.")
	consumerAddrs := app.String(cli.StringOpt{
		Name:   "consumer_proxy_addr",
		Value:  "",
		Desc:   "Comma separated kafka proxy hosts for message consuming.",
		EnvVar: "QUEUE_PROXY_ADDRS",
	})
	consumerHost := app.String(cli.StringOpt{
		Name:   "consumer_host_header",
		Value:  "",
		Desc:   "Host header for consumer proxy.",
		EnvVar: "QUEUE_HOST",
	})
	consumerGroupID := app.String(cli.StringOpt{
		Name:   "consumer_group_id",
		Value:  "",
		Desc:   "Kafka qroup id used for message consuming.",
		EnvVar: "GROUP_ID",
	})
	consumerAutoCommitEnable := app.Bool(cli.BoolOpt{
		Name:   "consumer_autocommit_enable",
		Value:  true,
		Desc:   "Enable autocommit for small messages.",
		EnvVar: "CONSUMER_AUTOCOMMIT_ENABLE",
	})
	consumerAuthorizationKey := app.String(cli.StringOpt{
		Name:   "consumer_authorization_key",
		Value:  "",
		Desc:   "The authorization key required to UCS access.",
		EnvVar: "AUTHORIZATION_KEY",
	})
	apiBaseURL := app.String(cli.StringOpt{
		Name:   "api_base_url",
		Value:  "http://api.ft.com",
		Desc:   "The API base URL where the content is accessible",
		EnvVar: "API_BASE_URL",
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
	nCap := app.Int(cli.IntOpt{
		Name:   "notifications_capacity",
		Value:  200,
		Desc:   "the nr of recent notifications to be saved and returned on the /notifications endpoint",
		EnvVar: "NOTIFICATIONS_CAPACITY",
	})
	app.Action = func() {
		dispatcher := newDispatcher()
		go dispatcher.distributeEvents()

		consumerConfig := queueConsumer.QueueConfig{}
		consumerConfig.Addrs = strings.Split(*consumerAddrs, ",")
		consumerConfig.Group = *consumerGroupID
		consumerConfig.Topic = *topic
		consumerConfig.Queue = *consumerHost
		consumerConfig.AuthorizationKey = *consumerAuthorizationKey
		consumerConfig.AutoCommitEnable = *consumerAutoCommitEnable

		infoLogger.Printf("Config: [\n\tconsumerAddrs: [%v]\n\tconsumerGroupID: [%v]\n\ttopic: [%v]\n\tconsumerAutoCommitEnable: [%v]\n\tapiBaseURL: [%v]\n\tnotifications_capacity: [%v]\n]", *consumerAddrs, *consumerGroupID, *topic, *consumerAutoCommitEnable, *apiBaseURL, *nCap)

		notificationsCache := newCircularBuffer(*nCap)
		h := handler{dispatcher, notificationsCache}
		hc := &healthcheck{client: http.Client{}, consumerConf: consumerConfig}
		http.HandleFunc("/content/notifications-push", h.notificationsPush)
		http.HandleFunc("/content/notifications", h.notifications)
		http.HandleFunc("/stats", h.stats)
		http.HandleFunc("/__health", hc.healthcheck())
		http.HandleFunc("/__gtg", hc.gtg)
		go func() {
			err := http.ListenAndServe(":"+strconv.Itoa(*port), nil)
			errorLogger.Println(err)
		}()

		app := notificationsApp{dispatcher, &consumerConfig, notificationBuilder{*apiBaseURL}, notificationsCache}
		app.consumeMessages()
	}
	if err := app.Run(os.Args); err != nil {
		errorLogger.Fatal(err)
	}
}

func init() {
	initLogs(os.Stdout, os.Stdout, os.Stderr)
}

func initLogs(infoHandle io.Writer, warnHandle io.Writer, errorHandle io.Writer) {
	infoLogger = log.New(infoHandle, "INFO  - ", logPattern)
	warnLogger = log.New(warnHandle, "WARN  - ", logPattern)
	errorLogger = log.New(errorHandle, "ERROR - ", logPattern)
}
