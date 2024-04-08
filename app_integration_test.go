package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/kafka-client-go/v4"
	"github.com/Financial-Times/notifications-push/v5/access"
	"github.com/Financial-Times/notifications-push/v5/consumer"
	"github.com/Financial-Times/notifications-push/v5/dispatch"
	"github.com/Financial-Times/notifications-push/v5/mocks"
	"github.com/Financial-Times/notifications-push/v5/resources"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
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
	articleHeader           = "application/vnd.ft-upp-article+json"
	apiGatewayValidateURL   = "/api-gateway/validate"
	apiGatewayPoliciesURL   = "/api-gateway/policies"
	apiGatewayGTGURL        = "/api-gateway/__gtg"
)

const (
	relatedContentNotificationMessagePath = "./test_data/input/related_content_article_notification_payload_kafka.json"
	expectedRelatedNotification           = "./test_data/output/expected_related_content_notification_data.json"

	articleNotificationMessagePath = "./test_data/input/standard_article_notification_payload_kafka.json"
	expectedUpdateNotification     = "./test_data/output/expected_article_notification_data.json"
)

var typeAllowlist = []string{"application/vnd.ft-upp-article+json", "application/vnd.ft-upp-content-package+json", "application/vnd.ft-upp-audio+json"}

var envVarsNames = []string{groupIdEnvVar, kafkaAddressEnvVar, kafkaLagToleranceEnvVar, topicEnvVar}
var envVarsDefaultValues = map[string]string{
	"GROUP_ID":            "notifications-push-local",
	"KAFKA_ADDRESS":       "localhost:19092",
	"KAFKA_LAG_TOLERANCE": "120",
	"TOPIC":               "PostPublicationEvents",
}

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

type NotificationJSONData struct {
	Data json.RawMessage `json:"data,omitempty"`
}

func setUp(t *testing.T) TestArtifacts {
	log := logger.NewUPPLogger("notifications-push-test-runner", "info")

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
			if envVarsDefaultValues[e] == "" {
				t.Fatalf("environment variable %s empty", e)
			}
			v = envVarsDefaultValues[e]
			t.Logf("environment variable %q is empty default value: %q if you run test from the IDE, set proper values in Environment.", e, envVarsDefaultValues[e])
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
		t.Fatalf("a problem setting up the OPA evaluator while creating service dispatcher: %v", err)
	}

	d := dispatch.NewDispatcher(delay, h, e, log)
	go d.Start()

	return d, h
}

func createKafkaProducer(t *testing.T, envVars map[string]string) *kafka.Producer {
	pc := kafka.ProducerConfig{
		ClusterArn:              nil,
		BrokersConnectionString: envVars[kafkaAddressEnvVar],
		Topic:                   envVars[topicEnvVar],
		Options:                 kafka.DefaultProducerOptions(),
	}

	kp, err := kafka.NewProducer(pc)
	if err != nil {
		t.Errorf("could not create Kafka producer: %v", err)
		return nil
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

	t.Logf("Creating Kafka consumer with: %v, %v", cc, kt)
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

// @Test - This is the main test
func TestPushNotifications(t *testing.T) {
	testArtifacts := setUp(t)

	defer testArtifacts.shutdown.Shutdown()
	defer testArtifacts.server.Close()

	_ = startTestService(testArtifacts.dispatcher, testArtifacts.kafkaConsumer, testArtifacts.queueHandler, testArtifacts.log)

	ctx, cancel := context.WithCancel(context.Background())

	relatedContentArticleKafkaMessage, err := getKafkaFTMessage(relatedContentNotificationMessagePath, articleHeader, testArtifacts.log)
	if err != nil {
		t.Fatalf("Expected test data file is missing: %s", relatedContentNotificationMessagePath)
	}

	relatedContentArticleNotification, err := getNotificationResponseData(expectedRelatedNotification, testArtifacts.log)
	if err != nil {
		t.Fatalf("Expected test notification data file is missing: %s", expectedRelatedNotification)
	}

	standardArticleKafkaMessage, err := getKafkaFTMessage(articleNotificationMessagePath, articleHeader, testArtifacts.log)
	if err != nil {
		t.Fatalf("Expected test data file is missing: %s", articleNotificationMessagePath)
	}

	standardArticleNotification, err := getNotificationResponseData(expectedUpdateNotification, testArtifacts.log)
	if err != nil {
		t.Fatalf("Expected test notification data file is missing: %s", expectedUpdateNotification)
	}

	tests := []struct {
		kafkaMessage         *kafka.FTMessage
		expectedNotification string
		xPolicy              string
	}{
		{
			kafkaMessage:         relatedContentArticleKafkaMessage,
			expectedNotification: relatedContentArticleNotification,
			xPolicy:              "INTERNAL_UNSTABLE",
		},
		{
			kafkaMessage:         standardArticleKafkaMessage,
			expectedNotification: standardArticleNotification,
			xPolicy:              "ADVANCED_SUBSCRIPTION",
		},
		{
			kafkaMessage: relatedContentArticleKafkaMessage,
			// To FIX: With these mocks we actually receive standard UPDATE event, instead of heartbeats only...
			expectedNotification: standardArticleNotification, // Expect only HeartBeat notifications, but: 1. With these mocks we receive UPDATE notification 2. how long to wait?!?
			xPolicy:              "ADVANCED_SUBSCRIPTION",
		},
	}

	for _, test := range tests {
		testArtifacts.router.HandleFunc(apiGatewayPoliciesURL, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(fmt.Sprintf("{'x-policy':%q}", test.xPolicy)))
		}).Methods("GET")

		testClientWithNotifications(ctx, t, testArtifacts.server.URL, "Article", test.expectedNotification)

		// This producer produces events, which are not consumed by the consumer!!!
		err := testArtifacts.kafkaProducer.SendMessage(*test.kafkaMessage)
		if err != nil {
			t.Fatalf("could net send Kafka message: %v", err)
		}

	}

	<-time.After(heartbeat * 10)

	cancel()

	// t.Fatal("Test timed out.")
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
					// assume we expect only one notification per subscriber, otherwise we need to count here in the for loop
					// TODO We have to interrupt the subscriber in this case and complete the test case...
					// ctx.Done()
					//break
					t.Logf("Notification received by the subscriber as expected: %s", expectedBody)
					return
				}
			}
		}
	}()

}

func startTestService(n notificationSystem,
	consumer *kafka.Consumer,
	msgHandler consumer.MessageQueueHandler,
	log *logger.UPPLogger) func() {
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

	// Subscribe for notifications with TIMEOUT
	//client := &http.Client{
	//	Timeout: 8 * time.Second,
	//}
	//resp, err := client.Do(req)
	//if err != nil {
	//	return nil, fmt.Errorf("error occurred while making request: %v", err)
	//}

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

// getKafkaFTMessage t.Helper() function, which constructs FTMessage kafka event based on payload from file
// By default the event is expected to be Article. If not correct header for Content-Type shall be specified.
func getKafkaFTMessage(pathToJSONFile string, notificationTypeHeader string, log *logger.UPPLogger) (*kafka.FTMessage, error) {
	currentTime := time.Now().UTC()
	formattedCurrentTime := currentTime.Format("2006-01-02T15:04:05.999Z")

	// Default notification Type to Article
	if notificationTypeHeader == "" {
		notificationTypeHeader = "application/vnd.ft-upp-article+json"
	}

	headers := map[string]string{}
	headers["X-Request-Id"] = "test-publish-123"
	headers["Message-Timestamp"] = formattedCurrentTime
	headers["Content-Type"] = notificationTypeHeader
	headers["Message-Type"] = "cms-content-published"
	headers["Origin-System-Id"] = "http://cmdb.ft.com/systems/cct"
	headers["Message-Id"] = uuid.New().String()

	body, err := getKafkaPayload(pathToJSONFile, log)
	if err != nil {
		return nil, err
	}

	ftMessage := kafka.NewFTMessage(headers, string(body))
	return &ftMessage, nil
}

// getKafkaPayload t.Helper() function, which reads json from a file and return content as []byte
func getKafkaPayload(pathToJSONFile string, log *logger.UPPLogger) ([]byte, error) {
	file, err := os.Open(pathToJSONFile)
	if err != nil {
		log.Errorf("error: %v while opening file: %v", err, pathToJSONFile)
		return nil, err
	}

	body, err := io.ReadAll(file)
	if err != nil {
		log.Errorf("error: %v while reading file: %v", err, pathToJSONFile)
		return nil, err
	}
	return body, nil
}

// getNotificationResponseData reads the notification expected formatted payload
// and returns it as string representation.
func getNotificationResponseData(pathToJSONFile string, log *logger.UPPLogger) (string, error) {
	data, err := getKafkaPayload(pathToJSONFile, log)
	if err != nil {
		return "", err
	}
	var notificationData NotificationJSONData
	if err := json.Unmarshal(data, &notificationData); err != nil {
		log.Errorf("error: cannot unmarshal expected notification as JSON: %v", err)
		return "", err
	}

	if len(notificationData.Data) == 0 {
		log.Errorf("No data present in expected notification as JSON: %s", pathToJSONFile)
		return "", err
	}

	// Convert JSON Pretty to one row JSON without spaces between elements
	jsonString := string(notificationData.Data)
	jsonString = strings.ReplaceAll(jsonString, "\\\\", "\\")
	jsonString = strings.ReplaceAll(jsonString, "\r\n", "")
	jsonString = strings.ReplaceAll(jsonString, "\n", "")
	jsonString = strings.ReplaceAll(jsonString, "    ", "")
	jsonString = strings.ReplaceAll(jsonString, "  ", "")
	jsonString = strings.ReplaceAll(jsonString, ": ", ":")
	jsonString = "data: " + jsonString + "\n\n\n"
	return jsonString, nil
}
