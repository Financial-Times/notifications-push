package main

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	cli "github.com/jawher/mow.cli"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/notifications-push/v5/resources"
)

const (
	heartbeatPeriod = 30 * time.Second
	appDescription  = "Proactively notifies subscribers about new publishes/modifications."
)

func main() {
	serviceName := os.Getenv("APP_NAME")
	if serviceName == "" {
		serviceName = "notifications-push"
	}
	app := cli.App(serviceName, appDescription)
	resource := app.String(cli.StringOpt{
		Name:   "notifications_resource",
		Value:  "",
		Desc:   "The resource of which notifications are produced (e.g., content or lists)",
		EnvVar: "NOTIFICATIONS_RESOURCE",
	})

	consumerAddress := app.String(cli.StringOpt{
		Name:   "consumer_addr",
		Value:  "",
		Desc:   "Comma separated kafka hosts for message consuming.",
		EnvVar: "KAFKA_ADDRESS",
	})

	consumerLagTolerance := app.Int(cli.IntOpt{
		Name:   "consumer_lag_tolerance",
		Value:  120,
		Desc:   "Kafka lag tolerance",
		EnvVar: "KAFKA_LAG_TOLERANCE",
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
	apiKeyPoliciesEndpoint := app.String(cli.StringOpt{
		Name:   "api_key_policies_endpoint",
		Value:  "t800/policy",
		Desc:   "The API Gateway ApiKey policies endpoint",
		EnvVar: "API_KEY_POLICIES_ENDPOINT",
	})
	apiGatewayHealthcheckEndpoint := app.String(cli.StringOpt{
		Name:   "api_healthcheck_endpoint",
		Value:  "/t800-healthcheck",
		Desc:   "The API Gateway healthcheck endpoint",
		EnvVar: "API_HEALTHCHECK_ENDPOINT",
	})
	kafkaTopic := app.String(cli.StringOpt{
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
	contentURIAllowList := app.String(cli.StringOpt{
		Name:   "contentURIAllowList",
		Value:  "",
		Desc:   `The contentURI allowlist for incoming notifications - i.e. ^http://.*-transformer-(pr|iw)-uk-.*\.svc\.ft\.com(:\d{2,5})?/content/[\w-]+.*$`,
		EnvVar: "CONTENT_URI_ALLOWLIST",
	})
	contentTypeAllowlist := app.Strings(cli.StringsOpt{
		Name:   "content_type_allowlist",
		Value:  []string{},
		Desc:   `Comma-separated list of allowlisted ContentTypes for incoming notifications - i.e. application/vnd.ft-upp-article+json,application/vnd.ft-upp-audio+json`,
		EnvVar: "CONTENT_TYPE_ALLOWLIST",
	})
	logLevel := app.String(cli.StringOpt{
		Name:   "logLevel",
		Value:  "INFO",
		Desc:   "Logging level (DEBUG, INFO, WARN, ERROR)",
		EnvVar: "LOG_LEVEL",
	})

	allowedAllContentType := app.Strings(cli.StringsOpt{
		Name:   "allowed_all_contentType",
		Value:  []string{},
		Desc:   `Comma-separated list of ContentTypes that compose ALL (contentType) - i.e. Article,`,
		EnvVar: "ALLOWED_ALL_CONTENT_TYPE",
	})

	supportedSubscriptionType := app.Strings(cli.StringsOpt{
		Name:   "supported_subscription_type",
		Value:  []string{},
		Desc:   `Comma-separated list of supported subscription types`,
		EnvVar: "SUPPORTED_SUBSCRIPTION_TYPE",
	})

	defaultSubscriptionType := app.String(cli.StringOpt{
		Name:   "default_subscription_type",
		Value:  "Article",
		Desc:   `The default subscription type to serve when no arguments are passed.`,
		EnvVar: "DEFAULT_SUBSCRIPTION_TYPE",
	})

	e2eTestUUIDs := app.Strings(cli.StringsOpt{
		Name:   "e2e_test_ids",
		Value:  []string{},
		Desc:   `Comma-separated list of allowed UUIDs used for end-to-end tests.`,
		EnvVar: "E2E_TEST_IDS",
	})

	shouldMonitor := app.Bool(cli.BoolOpt{
		Name:   "shouldMonitor",
		Value:  true,
		Desc:   "Specifies if the messages be monitored by PAM and NPM.",
		EnvVar: "MONITOR_NOTIFICATIONS",
	})

	updateEventType := app.String(cli.StringOpt{
		Name:   "updateEventType",
		Value:  "http://www.ft.com/thing/ThingChangeType/UPDATE",
		Desc:   "Specifies the update event type.",
		EnvVar: "UPDATE_EVENT_TYPE",
	})

	apiURLResource := app.String(cli.StringOpt{
		Name:   "apiURLResource",
		Value:  "",
		Desc:   "Specifies the resource in the apiUrl in the notification.",
		EnvVar: "API_URL_RESOURCE",
	})

	includeScoop := app.Bool(cli.BoolOpt{
		Name:   "includeScoop",
		Value:  false,
		Desc:   "Should the standout scoop field be included in the response.",
		EnvVar: "INCLUDE_SCOOP",
	})

	log := logger.NewUPPLogger(serviceName, *logLevel)

	app.Action = func() {
		log.WithFields(map[string]interface{}{
			"KAFKA_TOPIC":   *kafkaTopic,
			"GROUP_ID":      *consumerGroupID,
			"KAFKA_ADDRESS": *consumerAddress,
			"LAG_TOLERANCE": *consumerLagTolerance,
			"E2E_TEST_IDS":  *e2eTestUUIDs,
		}).Infof("[Startup] notifications-push is starting ")

		kafkaConsumer := createConsumer(log, *consumerAddress, *consumerGroupID, *kafkaTopic, *consumerLagTolerance)

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

		router := mux.NewRouter()
		srv := &http.Server{
			Addr:    ":" + strconv.Itoa(*port),
			Handler: router,
		}

		baseURL, err := url.Parse(*apiBaseURL)
		if err != nil {
			log.WithError(err).Fatal("cannot parse api_base_url")
		}

		healthCheckEndpoint, err := url.Parse(*apiGatewayHealthcheckEndpoint)
		if err != nil {
			log.WithError(err).Fatal("cannot parse api_healthcheck_endpoint")
		}

		healthCheckEndpoint = baseURL.ResolveReference(healthCheckEndpoint)
		hc := resources.NewHealthCheck(kafkaConsumer, healthCheckEndpoint.String(), requestStatusCode, serviceName, log)

		dispatcher, history := createDispatcher(*delay, *historySize, log)

		msgConfig := msgHandlerCfg{
			BaseURL:              *apiBaseURL,
			ContentURIAllowList:  *contentURIAllowList,
			ContentTypeAllowList: *contentTypeAllowlist,
			E2ETestUUIDs:         *e2eTestUUIDs,
			ShouldMonitor:        *shouldMonitor,
			UpdateEventType:      *updateEventType,
			APIUrlResource:       *apiURLResource,
			IncludeScoop:         *includeScoop,
		}

		queueHandler, err := createMessageHandler(msgConfig, dispatcher, log)
		if err != nil {
			log.WithError(err).Fatal("could not start notification consumer")
		}

		keyValidateURL, err := url.Parse(*apiKeyValidationEndpoint)
		if err != nil {
			log.WithError(err).Fatal("cannot parse api_key_validation_endpoint")
		}
		keyValidateURL = baseURL.ResolveReference(keyValidateURL)

		var keyPoliciesURL *url.URL
		var policyCheckAllowed bool
		if apiKeyPoliciesEndpoint != nil && *apiKeyPoliciesEndpoint != "" {
			keyPoliciesURL, err = url.Parse(*apiKeyPoliciesEndpoint)
			if err != nil {
				log.WithError(err).Fatal("cannot parse api_key_policies_endpoint")
			}
			keyPoliciesURL = baseURL.ResolveReference(keyPoliciesURL)
			policyCheckAllowed = true
		}

		keyProcessor := resources.NewKeyProcessor(keyValidateURL.String(), keyPoliciesURL.String(), httpClient, log)
		subHandler := resources.NewSubHandler(dispatcher, keyProcessor, srv, heartbeatPeriod,
			log, *allowedAllContentType, *supportedSubscriptionType, *defaultSubscriptionType, policyCheckAllowed)
		if err != nil {
			log.WithError(err).Fatal("Could not create request handler")
		}

		initRouter(router, subHandler, *resource, dispatcher, history, hc, log)

		shutdown := startService(srv, dispatcher, kafkaConsumer, queueHandler, log)

		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch

		shutdown(time.Second * 30)
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
