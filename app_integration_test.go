package main

import (
	"context"
	"fmt"
	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/kafka-client-go/v4"
	"github.com/Financial-Times/notifications-push/v5/access"
	"github.com/Financial-Times/notifications-push/v5/consumer"
	"github.com/Financial-Times/notifications-push/v5/dispatch"
	"github.com/Financial-Times/notifications-push/v5/mocks"
	"github.com/Financial-Times/notifications-push/v5/resources"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"testing"
	"time"
)

const (
	groupIdEnvVar           = "GROUP_ID"
	kafkaAddressEnvVar      = "KAFKA_ADDRESS"
	kafkaLagToleranceEnvVar = "KAFKA_LAG_TOLERANCE"
	topicEnvVar             = "TOPIC"
	delay                   = time.Millisecond * 200
	historySize             = 50
	uriAllowlist            = `^http://(methode|wordpress-article|content)(-collection|-content-placeholder)?-(mapper|unfolder)(-pr|-iw)?(-uk-.*)?\.svc\.ft\.com(:\d{2,5})?/(content|complementarycontent)/[\w-]+.*$`
	resource                = "content"
	heartbeat               = time.Second * 1
	apiGatewayValidateURL   = "/api-gateway/validate"
	apiGatewayPoliciesURL   = "/api-gateway/policies"
	apiGatewayGTGURL        = "/api-gateway/__gtg"
)

const (
	expectedUpdateNotification  = "data: [{\"apiUrl\":\"test-api/content/3cc23068-e501-11e9-9743-db5a370481bc\",\"id\":\"http://www.ft.com/thing/3cc23068-e501-11e9-9743-db5a370481bc\",\"type\":\"http://www.ft.com/thing/ThingChangeType/UPDATE\",\"title\":\"Lebanon eases dollar flow for importers as crisis grows\",\"standout\":{\"scoop\":false}}]\n\n\n"
	expectedRelatedNotification = "data: [{\"apiUrl\":\"test-api/content/3cc23068-e501-11e9-9743-db5a370481bc\",\"id\":\"http://www.ft.com/thing/3cc23068-e501-11e9-9743-db5a370481bc\",\"type\":\"http://www.ft.com/thing/ThingChangeType/RELATEDCONTENTt\",\"title\":\"Lebanon eases dollar flow for importers as crisis grows\",\"standout\":{\"scoop\":false}}]\n\n\n"
)

var relatedContentNotificationMessage = kafka.NewFTMessage(map[string]string{
	"Message-Id":        "e9234cdf-0e45-4d87-8276-cbe018bafa60",
	"Message-Timestamp": "2019-10-02T15:13:26.329Z",
	"Message-Type":      "cms-content-published",
	"Origin-System-Id":  "http://cmdb.ft.com/systems/cct",
	"Content-Type":      "application/vnd.ft-upp-article+json",
	"X-Request-Id":      "test-publish-123",
}, `{ "payload": { "title": "Lebanon eases dollar flow for importers as crisis grows", "type": "Article", "standout": { "scoop": false }, "is_related_content_notification": true }, "contentUri": "http://methode-article-mapper.svc.ft.com/content/3cc23068-e501-11e9-9743-db5a370481bc", "lastModified": "2019-10-02T15:13:19.52Z" }`)

var updateNotificationMessage = kafka.NewFTMessage(map[string]string{
	"Message-Id":        "e9234cdf-0e45-4d87-8276-cbe018bafa60",
	"Message-Timestamp": "2019-10-02T15:13:26.329Z",
	"Message-Type":      "cms-content-published",
	"Origin-System-Id":  "http://cmdb.ft.com/systems/cct",
	"Content-Type":      "application/vnd.ft-upp-article+json",
	"X-Request-Id":      "test-publish-123",
}, `{ "payload": { "title": "Lebanon eases dollar flow for importers as crisis grows", "type": "Article", "standout": { "scoop": false } }, "contentUri": "http://methode-article-mapper.svc.ft.com/content/3cc23068-e501-11e9-9743-db5a370481bc", "lastModified": "2019-10-02T15:13:19.52Z" }`)

var typeAllowlist = []string{"application/vnd.ft-upp-article+json", "application/vnd.ft-upp-content-package+json", "application/vnd.ft-upp-audio+json"}

var envVarsNames = []string{groupIdEnvVar, kafkaAddressEnvVar, kafkaLagToleranceEnvVar, topicEnvVar}

type TestArtifacts struct {
	log               *logger.UPPLogger
	dispatcher        *dispatch.Dispatcher
	dispatcherHistory dispatch.History
	kafkaConsumer     *kafka.Consumer
	kafkaProducer     *kafka.Producer
	queueHandler      consumer.MessageQueueHandler
	shutdown          *mocks.ShutdownReg
	router            *mux.Router
	server            *httptest.Server
	healthCheck       *resources.HealthCheck
	subHandler        *resources.SubHandler
}

func setUp(t *testing.T) TestArtifacts {
	log := logger.NewUPPLogger("notifications-push-test", "info")

	envVars := getEnvVars(t)

	dispatcher, dispatcherHistory := createTestDispatcher(t, delay, historySize, log)

	kafkaConsumer := createKafkaConsumer(t, envVars, log)
	kafkaProducer := createKafkaProducer(t, envVars)

	queueHandler := createQueueHandler(t, uriAllowlist, typeAllowlist, resource, "test-api", dispatcher, log)

	shutdown := mocks.NewShutdownReg()
	shutdown.On("RegisterOnShutdown", mock.Anything)

	router := mux.NewRouter()
	server := httptest.NewServer(router)

	keyProcessor, policyProcessor := createAPIGatewayProcessors(t, server, log)

	healthCheck := resources.NewHealthCheck(kafkaConsumer, apiGatewayGTGURL, nil, "notifications-push", log)

	subHandler := resources.NewSubHandler(dispatcher, keyProcessor, policyProcessor, shutdown, heartbeat, log, []string{"Article", "ContentPackage", "Audio"},
		[]string{"Article", "ContentPackage", "Audio", "All", "LiveBlogPackage", "LiveBlogPost", "Content"}, "Article")

	initRouter(router, subHandler, resource, dispatcher, dispatcherHistory, healthCheck, log)

	router.HandleFunc(apiGatewayValidateURL, func(resp http.ResponseWriter, req *http.Request) {
		resp.WriteHeader(http.StatusOK)
	}).Methods("GET")

	return TestArtifacts{
		log:               log,
		dispatcher:        dispatcher,
		dispatcherHistory: dispatcherHistory,
		kafkaConsumer:     kafkaConsumer,
		kafkaProducer:     kafkaProducer,
		queueHandler:      queueHandler,
		shutdown:          shutdown,
		router:            router,
		server:            server,
		healthCheck:       healthCheck,
		subHandler:        subHandler,
	}
}

func getEnvVars(t *testing.T) map[string]string {
	values := make(map[string]string)

	for _, e := range envVarsNames {
		v := os.Getenv(e)
		if v == "" {
			t.Fatalf("environment variable %s empty", v)
		}

		values[e] = v
	}

	return values
}

func createTestDispatcher(t *testing.T, delay time.Duration, historySize int, log *logger.UPPLogger) (*dispatch.Dispatcher, dispatch.History) {
	h := dispatch.NewHistory(historySize)

	e, err := access.CreateEvaluator(
		"data.specialContent.allow",
		[]string{"./opa_modules/special_content.rego"},
	)
	if err != nil {
		t.Fatalf("a problem setting up the OPA evaluator while creating notifications-push's dispatcher: %v", err)
	}

	d := dispatch.NewDispatcher(delay, h, e, log)
	go d.Start()

	return d, h
}

func createKafkaProducer(t *testing.T, envVars map[string]string) *kafka.Producer {
	pc := kafka.ProducerConfig{
		BrokersConnectionString: envVars[kafkaAddressEnvVar],
		Topic:                   envVars[topicEnvVar],
	}

	kp, err := kafka.NewProducer(pc)
	if err != nil {
		t.Fatalf("could not create Kafka producer: %v", err)
	}

	return kp
}

func createKafkaConsumer(t *testing.T, envVars map[string]string, log *logger.UPPLogger) *kafka.Consumer {
	cc := kafka.ConsumerConfig{
		BrokersConnectionString: envVars[kafkaAddressEnvVar],
		ConsumerGroup:           envVars[groupIdEnvVar],
		Options:                 kafka.DefaultConsumerOptions(),
	}

	lt, err := strconv.Atoi(envVars[kafkaLagToleranceEnvVar])
	if err != nil {
		t.Fatalf("could not configure Kafka lag tolerance: %v", err)
	}

	kt := []*kafka.Topic{
		kafka.NewTopic(envVars[topicEnvVar], kafka.WithLagTolerance(int64(lt))),
	}

	kc, err := kafka.NewConsumer(cc, kt, log)
	if err != nil {
		t.Fatalf("could not create Kafka consumer: %v", err)
	}

	return kc
}

func createQueueHandler(t *testing.T, uriAllowList string, typeAllowList []string, resource string, apiURL string, d *dispatch.Dispatcher, log *logger.UPPLogger) consumer.MessageQueueHandler {
	set := consumer.NewSet()
	for _, value := range typeAllowList {
		set.Add(value)
	}
	reg, err := regexp.Compile(uriAllowList)
	if err != nil {
		t.Fatalf("could not compile uriAllowList when creating QueueHandler: %v", err)
	}

	m := consumer.NotificationMapper{
		APIBaseURL:      apiURL,
		APIUrlResource:  resource,
		UpdateEventType: "http://www.ft.com/thing/ThingChangeType/UPDATE",
		IncludeScoop:    true,
	}

	return consumer.NewQueueHandler(reg, set, nil, true, m, d, log)
}

func createAPIGatewayProcessors(t *testing.T, server *httptest.Server, log *logger.UPPLogger) (*access.KeyProcessor, *access.PolicyProcessor) {
	kpURL, err := url.Parse(server.URL + apiGatewayValidateURL)
	if err != nil {
		t.Fatalf("could not parse key processor URL: %v", err)
	}

	ppURL, err := url.Parse(server.URL + apiGatewayPoliciesURL)
	if err != nil {
		t.Fatalf("could not parse policy processor URL: %v", err)
	}

	kp := access.NewKeyProcessor(kpURL, http.DefaultClient, log)
	pp := access.NewPolicyProcessor(ppURL, http.DefaultClient)

	return kp, pp
}

func TestPushNotifications(t *testing.T) {
	testArtifacts := setUp(t)

	defer testArtifacts.shutdown.Shutdown()
	defer testArtifacts.server.Close()

	_ = startTestService(testArtifacts.dispatcher, testArtifacts.kafkaConsumer, testArtifacts.queueHandler, testArtifacts.log)

	ctx, cancel := context.WithCancel(context.Background())

	tests := []struct {
		msg      kafka.FTMessage
		expected string
		xPolicy  string
	}{
		{
			msg:      relatedContentNotificationMessage,
			expected: expectedRelatedNotification,
			xPolicy:  "INTERNAL_UNSTABLE",
		},
	}

	for _, test := range tests {
		testArtifacts.router.HandleFunc(apiGatewayPoliciesURL, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(fmt.Sprintf("{'x-policy':%q}", test.xPolicy)))
		}).Methods("GET")

		testClientWithNotifications(ctx, t, testArtifacts.server.URL, "Article", test.expected)

		err := testArtifacts.kafkaProducer.SendMessage(test.msg)
		if err != nil {
			t.Fatalf("could net send Kafka message %v", err)
		}
	}

	<-time.After(heartbeat * 5)

	cancel()

	//t.Fatal("Test timed out.")
}

func testClientWithNotifications(ctx context.Context, t *testing.T, serverURL string, subType string, expectedBody string) {
	ch, err := startSubscriber(ctx, serverURL, subType)
	assert.NoError(t, err)

	go func() {
		body := <-ch
		assert.Equal(t, "data: []\n\n", body, "Client with type '%s' expects to receive heartbeat message when connecting to the service.", subType)

		for {
			select {
			case <-ctx.Done():
				return
			case body = <-ch:
				if body != "data: []\n\n" {
					assert.Equal(t, expectedBody, body, "Client with type '%s' received incorrect body", subType)
				}
			}
		}
	}()
}

func startTestService(n notificationSystem, consumer *kafka.Consumer, msgHandler consumer.MessageQueueHandler, log *logger.UPPLogger) func() {
	go n.Start()

	go consumer.Start(msgHandler.HandleMessage)

	return func() {
		log.Info("Termination started. Quitting message consumer and notification dispatcher function.")

		err := consumer.Close()
		if err != nil {
			log.WithError(err).Error("Failed to close kafka consumer")
		}

		n.Stop()
	}
}

func startSubscriber(ctx context.Context, serverURL string, subType string) (<-chan string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serverURL+"/content/notifications-push?type="+subType, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", "test-key")

	resp, err := http.DefaultClient.Do(req) //nolint:bodyclose
	if err != nil {
		return nil, err
	}

	ch := make(chan string)
	go func() {

		defer close(ch)
		defer resp.Body.Close()

		buf := make([]byte, 4096)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				idx, err := resp.Body.Read(buf)
				if err != nil {
					return
				}
				ch <- string(buf[:idx])
			}
		}
	}()

	return ch, nil
}
