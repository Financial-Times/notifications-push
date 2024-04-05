//go:build integration
// +build integration

package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"
	"time"

	"github.com/Financial-Times/notifications-push/v5/access"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/kafka-client-go/v4"
	"github.com/Financial-Times/notifications-push/v5/consumer"
	"github.com/Financial-Times/notifications-push/v5/dispatch"
	"github.com/Financial-Times/notifications-push/v5/mocks"
	"github.com/Financial-Times/notifications-push/v5/resources"
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

var invalidContentURIMsg = kafka.NewFTMessage(map[string]string{
	"Message-Id":        "58b55a73-3074-44ed-999f-ea7ff7b48605",
	"Message-Timestamp": "2019-10-02T15:13:26.329Z",
	"Message-Type":      "concept-annotation",
	"Origin-System-Id":  "http://cmdb.ft.com/systems/pac",
	"Content-Type":      "application/json",
	"X-Request-Id":      "test-publish-123",
}, `{"contentUri": "http://invalid.svc.ft.com/annotations/4de8b414-c5aa-11e9-a8e9-296ca66511c9","lastModified": "2019-10-02T15:13:19.52Z","payload": {"uuid":"4de8b414-c5aa-11e9-a8e9-296ca66511c9","annotations":[{"thing":{ "id":"http://www.ft.com/thing/68678217-1d06-4600-9d43-b0e71a333c2a","predicate":"about"}}]}}`)
var articleMsgWithRelatedContentNotificationFlag = kafka.NewFTMessage(map[string]string{
	"Message-Id":        "e9234cdf-0e45-4d87-8276-cbe018bafa60",
	"Message-Timestamp": "2019-10-02T15:13:26.329Z",
	"Message-Type":      "cms-content-published",
	"Origin-System-Id":  "http://cmdb.ft.com/systems/cct",
	"Content-Type":      "application/vnd.ft-upp-article+json",
	"X-Request-Id":      "test-publish-1234",
}, `{ "payload": { "title": "Lebanon eases dollar flow for importers as crisis grows", "type": "Article", "standout": { "scoop": false }, "is_related_content_notification": true }, "contentUri": "http://methode-article-mapper.svc.ft.com/content/3cc23068-e501-11e9-9743-db5a370481bc", "lastModified": "2019-10-02T15:13:19.52Z" }`)

// handlers vars
var (
	apiGatewayValidateURL = "/api-gateway/validate"
	apiGatewayPoliciesURL = "/api-gateway/policies"
	apiGatewayGTGURL      = "/api-gateway/__gtg"
	heartbeat             = time.Second * 1
	resource              = "content"
)

func TestPushNotifications(t *testing.T) {
	t.Parallel()

	l := logger.NewUPPLogger("TEST", "info")

	// dispatch vars
	var (
		delay       = time.Millisecond * 200
		historySize = 50
	)
	// message consumer vars
	var (
		uriAllowlist                    = `^http://(methode|wordpress-article|content)(-collection|-content-placeholder)?-(mapper|unfolder)(-pr|-iw)?(-uk-.*)?\.svc\.ft\.com(:\d{2,5})?/(content|complementarycontent)/[\w-]+.*$`
		typeAllowlist                   = []string{"application/vnd.ft-upp-article+json", "application/vnd.ft-upp-content-package+json", "application/vnd.ft-upp-audio+json"}
		expectedArticleNotificationBody = "data: [{\"apiUrl\":\"test-api/content/3cc23068-e501-11e9-9743-db5a370481bc\",\"id\":\"http://www.ft.com/thing/3cc23068-e501-11e9-9743-db5a370481bc\",\"type\":\"http://www.ft.com/thing/ThingChangeType/UPDATE\",\"title\":\"Lebanon eases dollar flow for importers as crisis grows\",\"standout\":{\"scoop\":false}}]\n\n\n"
		sendDelay                       = time.Millisecond * 190
	)

	// mocks
	queue := &mocks.KafkaConsumer{}
	reg := mocks.NewShutdownReg()
	reg.On("RegisterOnShutdown", mock.Anything)
	defer reg.Shutdown()
	// dispatcher
	d, h, err := startDispatcher(delay, historySize, l)
	defer d.Stop()
	assert.NoError(t, err)

	// server
	router := mux.NewRouter()
	server := httptest.NewServer(router)
	defer server.Close()

	// handler
	hc := resources.NewHealthCheck(queue, apiGatewayGTGURL, nil, "notifications-push", l)

	keyProcessorURL, _ := url.Parse(server.URL + apiGatewayValidateURL)
	// We need a PolicyProcessor, which may return different policies based on API key
	policyProcessorURL, _ := url.Parse(server.URL + apiGatewayPoliciesURL)

	keyProcessor := access.NewKeyProcessor(keyProcessorURL, http.DefaultClient, l)
	policyProcessor := access.NewPolicyProcessor(policyProcessorURL, http.DefaultClient)

	s := resources.NewSubHandler(d, keyProcessor, policyProcessor, reg, heartbeat, l, []string{"Article", "ContentPackage", "Audio"},
		[]string{"Article", "ContentPackage", "Audio", "All", "LiveBlogPackage", "LiveBlogPost", "Content"}, "Article")

	initRouter(router, s, resource, d, h, hc, l)

	// key validation
	router.HandleFunc(apiGatewayValidateURL, func(resp http.ResponseWriter, req *http.Request) {
		resp.WriteHeader(http.StatusOK)
	}).Methods("GET")

	// key policies
	router.HandleFunc(apiGatewayPoliciesURL, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{'x-policy':''"))
	}).Methods("GET")

	// context that controls the life of all subscribers
	ctx, cancel := context.WithCancel(context.Background())

	testHealthCheckEndpoints(ctx, t, server.URL, queue, hc)

	testClientWithNONotifications(ctx, t, server.URL, heartbeat, "Audio")
	testClientWithNotifications(ctx, t, server.URL, "Article", expectedArticleNotificationBody)
	testClientWithNotifications(ctx, t, server.URL, "All", expectedArticleNotificationBody)
	testClientShouldNotReceiveNotification(ctx, t, server.URL, "ContentPackage", expectedArticleNotificationBody)
	testClientWithXPolicyNotifications(ctx, t, server.URL, "Article", expectedArticleNotificationBody, router, "TEST_POLICY")

	reg.AssertNumberOfCalls(t, "RegisterOnShutdown", 5)

	// message producer
	go func() {
		msgs := []kafka.FTMessage{
			articleMsg,
			syntheticMsg,
			invalidContentTypeMsg,
			invalidContentURIMsg,
			articleMsgWithRelatedContentNotificationFlag,
		}
		var buff bytes.Buffer
		log := logger.NewUnstructuredLogger()
		log.SetOutput(&buff)
		queueHandler := createMsgQueue(t, uriAllowlist, typeAllowlist, resource, "test-api", d, log)
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(sendDelay):
				for _, msg := range msgs {
					queueHandler.HandleMessage(msg)
					buffMessage := buff.String()
					if !assert.NotContains(t, buffMessage, "error") {
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

func testHealthCheckEndpoints(ctx context.Context, t *testing.T, serverURL string, queue *mocks.KafkaConsumer, hc *resources.HealthCheck) {
	tests := map[string]struct {
		url            string
		expectedStatus int
		expectedBody   string
		clientFunc     resources.RequestStatusFn
		kafkaFunc      func() error
	}{"gtg endpoint success": {
		url: "/__gtg",
		clientFunc: func(ctx context.Context, url string) (int, error) {
			return http.StatusOK, nil
		},
		kafkaFunc: func() error {
			return nil
		},
		expectedStatus: http.StatusOK,
		expectedBody:   "OK",
	},
		"gtg endpoint kafka failure": {
			url: "/__gtg",
			clientFunc: func(ctx context.Context, url string) (int, error) {
				return http.StatusOK, nil
			},
			kafkaFunc: func() error {
				return fmt.Errorf("error connecting to kafka queue")
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody:   "error connecting to kafka queue",
		},
		"gtg endpoint ApiGateway failure": {
			url: "/__gtg",
			clientFunc: func(ctx context.Context, url string) (int, error) {
				return http.StatusServiceUnavailable, fmt.Errorf("gateway failed")
			},
			kafkaFunc: func() error {
				return nil
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody:   "gateway failed",
		},
		"responds on build-info": {
			url: "/__build-info",
			clientFunc: func(ctx context.Context, url string) (int, error) {
				return http.StatusOK, nil
			},
			kafkaFunc: func() error {
				return nil
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"version":`,
		},
		"responds on ping": {
			url: "/__ping",
			clientFunc: func(ctx context.Context, url string) (int, error) {
				return http.StatusOK, nil
			},
			kafkaFunc: func() error {
				return nil
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "pong",
		},
	}
	backupClientFunc := hc.StatusFunc
	backupKafkaFunc := queue.ConnectivityCheckF
	defer func() {
		hc.StatusFunc = backupClientFunc
		queue.ConnectivityCheckF = backupKafkaFunc
	}()
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {

			hc.StatusFunc = test.clientFunc
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

// Tests a subscriber that expects only notifications because of its X-Policy: INTERNAL_UNSTABLE
func testClientWithXPolicyNotifications(ctx context.Context, t *testing.T, serverURL string, subType string, expectedBody string, router *mux.Router, policy string) {
	router.HandleFunc(apiGatewayPoliciesURL, func(w http.ResponseWriter, _ *http.Request) {
		// We may add logic to get X-Api-Policy from request and based on it to return x-policy instead of param
		_, _ = w.Write([]byte("{'x-policy':'" + policy + "'"))
	}).Methods("GET")

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

// Tests a subscriber should not receive a notification
func testClientShouldNotReceiveNotification(ctx context.Context, t *testing.T, serverURL string, subType string, expectedBody string) {
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
				assert.NotEqual(t, expectedBody, body, "Client with type '%s' received a notification when they shouldn't", subType)
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
				assert.InEpsilon(t, heartbeat.Nanoseconds(), delta.Nanoseconds(), 0.05, "No Notification Client with type '%s' expects to receive heartbeat on time.")
				assert.Equal(t, "data: []\n\n", body, "No Notification Client with type '%s' expects to receive only heartbeat messages.")
				start = start.Add(heartbeat)
			}
		}
	}()
}

func startSubscriberWithXPolicy(ctx context.Context, serverURL string, subType string, policy string) (<-chan string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serverURL+"/content/notifications-push?type="+subType, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", "123456INTERNAL_UNSTABLE")
	req.Header.Set("X-Policy", policy)

	var resp *http.Response
	fmt.Printf("Request URL: %s", req.URL.String())
	resp, err = http.DefaultClient.Do(req) //nolint:bodyclose
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

func startSubscriber(ctx context.Context, serverURL string, subType string) (<-chan string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serverURL+"/content/notifications-push?type="+subType, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", "test-key")

	var resp *http.Response
	resp, err = http.DefaultClient.Do(req) //nolint:bodyclose
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

func startDispatcher(delay time.Duration, historySize int, log *logger.UPPLogger) (*dispatch.Dispatcher, dispatch.History, error) {
	h := dispatch.NewHistory(historySize)
	oa := access.GetOPAAgentForTesting(log)
	d := dispatch.NewDispatcher(delay, h, oa, log)
	go d.Start()
	return d, h, nil
}

func createMsgQueue(t *testing.T, uriAllowlist string, typeAllowlist []string, resource string, apiURL string, d *dispatch.Dispatcher, log *logger.UPPLogger) consumer.MessageQueueHandler {
	set := consumer.NewSet()
	for _, value := range typeAllowlist {
		set.Add(value)
	}
	reg, err := regexp.Compile(uriAllowlist)
	assert.NoError(t, err)

	mapper := consumer.NotificationMapper{
		APIBaseURL:      apiURL,
		APIUrlResource:  resource,
		UpdateEventType: "http://www.ft.com/thing/ThingChangeType/UPDATE",
		IncludeScoop:    true,
	}

	return consumer.NewQueueHandler(reg, set, nil, true, mapper, d, log)
}
