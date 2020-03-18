// +build integration

package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	log "github.com/Financial-Times/go-logger"
	logger "github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/kafka-client-go/kafka"
	"github.com/Financial-Times/notifications-push/v4/consumer"
	"github.com/Financial-Times/notifications-push/v4/dispatch"
	"github.com/Financial-Times/notifications-push/v4/mocks"
	"github.com/Financial-Times/notifications-push/v4/resources"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

var articleMsg = kafka.NewFTMessage(map[string]string{
	"Message-Id":        "e9234cdf-0e45-4d87-8276-cbe018bafa60",
	"Message-Timestamp": "2019-10-02T15:13:26.329Z",
	"Message-Type":      "cms-content-published",
	"Origin-System-Id":  "http://cmdb.ft.com/systems/cct",
	"Content-Type":      "application/vnd.ft-upp-article+json",
	"X-Request-Id":      "test-publish-123",
}, `{ "payload": { "title": "Lebanon eases dollar flow for importers as crisis grows", "type": "Article", "standout": { "scoop": false } }, "contentUri": "http://methode-article-mapper.svc.ft.com/content/3cc23068-e501-11e9-9743-db5a370481bc", "lastModified": "2019-10-02T15:13:19.52Z" }`)

var syntheticMsg = kafka.NewFTMessage(map[string]string{
	"Message-Id":        "e9234cdf-0e45-4d87-8276-cbe018bafa60",
	"Message-Timestamp": "2019-10-02T15:13:26.329Z",
	"Message-Type":      "cms-content-published",
	"Origin-System-Id":  "http://cmdb.ft.com/systems/methode-web-pub",
	"Content-Type":      "application/vnd.ft-upp-article+json",
	"X-Request-Id":      "SYNTH-123",
}, `{"payload":{"title":"Synthetic message","type":"Article","standout":{"scoop":false}},"contentUri":"synthetic/3cc23068-e501-11e9-9743-db5a370481bc","lastModified":"2019-10-02T15:13:19.52Z"}`)

var invalidContentTypeMsg = kafka.NewFTMessage(map[string]string{
	"Message-Id":        "e9234cdf-0e45-4d87-8276-cbe018bafa60",
	"Message-Timestamp": "2019-10-02T15:13:26.329Z",
	"Message-Type":      "cms-content-published",
	"Origin-System-Id":  "http://cmdb.ft.com/systems/methode-web-pub",
	"Content-Type":      "application/invalid-type",
	"X-Request-Id":      "test-publish-123",
}, `{"payload":{"title":"Invalid type message","type":"Article","standout":{"scoop":false}},"contentUri":"invalid type/3cc23068-e501-11e9-9743-db5a370481bc","lastModified":"2019-10-02T15:13:19.52Z"}`)

var annotationMsg = kafka.NewFTMessage(map[string]string{
	"Message-Id":        "58b55a73-3074-44ed-999f-ea7ff7b48605",
	"Message-Timestamp": "2019-10-02T15:13:26.329Z",
	"Message-Type":      "concept-annotation",
	"Origin-System-Id":  "http://cmdb.ft.com/systems/pac",
	"Content-Type":      "application/json",
	"X-Request-Id":      "test-publish-123",
}, `{"uuid":"4de8b414-c5aa-11e9-a8e9-296ca66511c9","annotations":[{"thing":{"id":"http://www.ft.com/thing/68678217-1d06-4600-9d43-b0e71a333c2a","predicate":"about"}}]}`)

func TestPushNotifications(t *testing.T) {

	l := logger.NewUPPLogger("TEST", "PANIC")
	log.InitLogger("TEST", "PANIC")

	// handlers vars
	var (
		apiGatewayURL    = "/api-gateway"
		apiGatewayGTGURL = "/api-gateway/__gtg"
		heartbeat        = time.Second * 1
		resource         = "content"
	)
	// dispatch vars
	var (
		delay       = time.Millisecond * 200
		historySize = 50
	)
	// message consumer vars
	var (
		uriWhitelist                    = `^http://(methode|wordpress-article|content)(-collection|-content-placeholder)?-(mapper|unfolder)(-pr|-iw)?(-uk-.*)?\.svc\.ft\.com(:\d{2,5})?/(content|complementarycontent)/[\w-]+.*$`
		typeWhitelist                   = []string{"application/vnd.ft-upp-article+json", "application/vnd.ft-upp-content-package+json", "application/vnd.ft-upp-audio+json"}
		originWhitelist                 = []string{"http://cmdb.ft.com/systems/pac", "http://cmdb.ft.com/systems/methode-web-pub", "http://cmdb.ft.com/systems/next-video-editor"}
		expectedArticleNotificationBody = "data: [{\"apiUrl\":\"test-api/content/3cc23068-e501-11e9-9743-db5a370481bc\",\"id\":\"http://www.ft.com/thing/3cc23068-e501-11e9-9743-db5a370481bc\",\"type\":\"http://www.ft.com/thing/ThingChangeType/UPDATE\",\"title\":\"Lebanon eases dollar flow for importers as crisis grows\",\"standout\":{\"scoop\":false}}]\n\n\n"
		expectedPACNotificationBody     = "data: [{\"apiUrl\":\"test-api/content/4de8b414-c5aa-11e9-a8e9-296ca66511c9\",\"id\":\"http://www.ft.com/thing/4de8b414-c5aa-11e9-a8e9-296ca66511c9\",\"type\":\"http://www.ft.com/thing/ThingChangeType/ANNOTATIONS_UPDATE\"}]\n\n\n"
		sendDelay                       = time.Millisecond * 190
	)

	// mocks
	queue := &mocks.KafkaConsumer{}
	statusClient := &mocks.StatusCodeClient{}
	reg := mocks.NewShutdownReg()
	reg.On("RegisterOnShutdown", mock.Anything)
	defer reg.Shutdown()
	// dispatcher
	d, h := startDispatcher(delay, historySize)
	defer d.Stop()

	// consumer
	msgQueue := createMsgQueue(t, uriWhitelist, typeWhitelist, originWhitelist, resource, "test-api", d)

	// server
	router := mux.NewRouter()
	server := httptest.NewServer(router)
	defer server.Close()

	// handler
	hc := resources.NewHealthCheck(queue, apiGatewayGTGURL, statusClient)

	keyValidator := resources.NewKeyValidator(server.URL+apiGatewayURL, http.DefaultClient, l)
	s := resources.NewSubHandler(d, keyValidator, reg, heartbeat, l)

	initRouter(router, s, resource, d, h, hc)

	// key validation
	router.HandleFunc(apiGatewayURL, func(resp http.ResponseWriter, req *http.Request) {
		resp.WriteHeader(http.StatusOK)
	}).Methods("GET")

	// context that controls the live of all subscribers
	ctx, cancel := context.WithCancel(context.Background())

	testHealthcheckEndpoints(ctx, t, server.URL, queue, statusClient)

	testClientWithNONotifications(ctx, t, server.URL, heartbeat, "Audio")
	testClientWithNotifications(ctx, t, server.URL, "Article", expectedArticleNotificationBody)
	testClientWithNotifications(ctx, t, server.URL, "All", expectedArticleNotificationBody)
	testClientWithNotifications(ctx, t, server.URL, "Annotations", expectedPACNotificationBody)
	reg.AssertNumberOfCalls(t, "RegisterOnShutdown", 4)

	// message producer
	go func() {
		msgs := []kafka.FTMessage{
			articleMsg,
			syntheticMsg,
			invalidContentTypeMsg,
			annotationMsg,
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(sendDelay):
				for _, msg := range msgs {
					err := msgQueue.HandleMessage(msg)
					if err != nil {
						assert.NoError(t, err)
						return
					}
				}
			}
		}
	}()

	<-time.After(heartbeat * 5)
	// shutdown test
	cancel()

}

func testHealthcheckEndpoints(ctx context.Context, t *testing.T, serverURL string, queue *mocks.KafkaConsumer, statusClient *mocks.StatusCodeClient) {

	tests := map[string]struct {
		url            string
		expectedStatus int
		expectedBody   string
		clientFunc     func(string) (int, error)
		kafkaFunc      func() error
	}{"gtg endpoint success": {
		url: "/__gtg",
		clientFunc: func(url string) (int, error) {
			return 200, nil
		},
		kafkaFunc: func() error {
			return nil
		},
		expectedStatus: 200,
		expectedBody:   "OK",
	},
		"gtg endpoint kafka failure": {
			url: "/__gtg",
			clientFunc: func(url string) (int, error) {
				return 200, nil
			},
			kafkaFunc: func() error {
				return errors.New("sample error")
			},
			expectedStatus: 503,
			expectedBody:   "error connecting to kafka queue",
		},
		"gtg endpoint ApiGateway failure": {
			url: "/__gtg",
			clientFunc: func(url string) (int, error) {
				return 503, errors.New("gateway failed")
			},
			kafkaFunc: func() error {
				return nil
			},
			expectedStatus: 503,
			expectedBody:   "gateway failed",
		},
		"responds on build-info": {
			url: "/__build-info",
			clientFunc: func(url string) (int, error) {
				return 200, nil
			},
			kafkaFunc: func() error {
				return nil
			},
			expectedStatus: 200,
			expectedBody:   `{"version":`,
		},
		"responds on ping": {
			url: "/__ping",
			clientFunc: func(url string) (int, error) {
				return 200, nil
			},
			kafkaFunc: func() error {
				return nil
			},
			expectedStatus: 200,
			expectedBody:   "pong",
		},
	}
	backupClientFunc := statusClient.GetStatusCodeF
	backupKafkaFunc := queue.ConnectivityCheckF
	defer func() {
		statusClient.GetStatusCodeF = backupClientFunc
		queue.ConnectivityCheckF = backupKafkaFunc
	}()
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			statusClient.GetStatusCodeF = test.clientFunc
			queue.ConnectivityCheckF = test.kafkaFunc

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, serverURL+test.url, nil)
			if err != nil {
				t.Fatalf("could not create request: %v", err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("failed making request: %v", err)
			}
			defer resp.Body.Close()

			buf := new(bytes.Buffer)
			_, _ = buf.ReadFrom(resp.Body)
			body := buf.String()

			assert.Equal(t, test.expectedStatus, resp.StatusCode)
			assert.Contains(t, body, test.expectedBody)
		})
	}
}

// Tests a subscriber that expects only notifications
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
				assert.Equal(t, expectedBody, body, "Client with type '%s' received incorrect body", subType)
			}
		}
	}()
}

// Tests a subscriber that expects only heartbeats
func testClientWithNONotifications(ctx context.Context, t *testing.T, serverURL string, heartbeat time.Duration, subType string) {

	ch, err := startSubscriber(ctx, serverURL, subType)
	assert.NoError(t, err)

	go func() {

		body := <-ch
		assert.Equal(t, "data: []\n\n", body, "Client with type '%s' expects to receive heartbeat message when connecting to the service.", subType)
		start := time.Now()

		for {
			select {
			case <-ctx.Done():
				return
			case body := <-ch:
				delta := time.Since(start)
				assert.InEpsilon(t, heartbeat.Nanoseconds(), delta.Nanoseconds(), 0.05, "No Notification Client with type '%s' expects to receive heatbests on time.")
				assert.Equal(t, "data: []\n\n", body, "No Notification Client with type '%s' expects to receive only heartbeat messages.")
				start = start.Add(heartbeat)
			}
		}
	}()
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

func startDispatcher(delay time.Duration, historySize int) (*dispatch.Dispatcher, dispatch.History) {
	h := dispatch.NewHistory(historySize)
	d := dispatch.NewDispatcher(delay, h)
	go d.Start()
	return d, h
}

func createMsgQueue(t *testing.T, uriWhitelist string, typeWhitelist []string, originWhitelist []string, resource string, apiURL string, d *dispatch.Dispatcher) consumer.MessageQueueHandler {
	set := consumer.NewSet()
	for _, value := range typeWhitelist {
		set.Add(value)
	}
	reg, err := regexp.Compile(uriWhitelist)
	assert.NoError(t, err)

	mapper := consumer.NotificationMapper{
		Resource:   resource,
		APIBaseURL: apiURL,
		Property:   &conceptTimeReader{},
	}
	contentHandler := consumer.NewContentQueueHandler(reg, set, mapper, d)
	metadataHandler := consumer.NewMetadataQueueHandler(originWhitelist, mapper, d)

	return consumer.NewMessageQueueHandler(contentHandler, metadataHandler)
}
