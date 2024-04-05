//go:build integration
// +build integration

package main

import (
	"encoding/json"
	"fmt"
	"github.com/Financial-Times/kafka-client-go/v4" //nolint:goimports
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time" //nolint:goimports
)

const (
	kafkaTopic        = "PostPublicationEvents"
	subscriberTimeout = 5 // in seconds, shall be synced with NOTIFICATIONS_DELAY configured for the service
)

type FTKafkaMessageMock struct {
	Headers map[string]string `json:"headers,omitempty"`
	Body    json.RawMessage   `json:"body,omitempty"`
}

// nolint:gocognit
func TestService(t *testing.T) {
	serviceBaseURL := os.Getenv("INTERNAL_BASE_URL")
	if serviceBaseURL == "" {
		t.Skip("Skipping this test because INTERNAL_BASE_URL is not set, so we are not in Docker Compose Environment.")
	}

	tests := []struct {
		name                  string
		xPolicies             string
		apiKey                string // Check Portal: https://apigateway.in.ft.com/key-form/developer
		subscriberEndpoint    string
		requestMethod         string
		kafkaTopic            string
		kafkaPayloadPath      string
		responseStatusCode    int
		expectedInResponse    string
		notExpectedInResponse string
		expectedError         error
	}{
		{
			name:                  "Update of ContentRelation shall not notify client who is subscribed for Article only",
			xPolicies:             "INTERNAL_UNSTABLE",
			apiKey:                "123456TEST",
			subscriberEndpoint:    "/content/notifications-push",
			requestMethod:         http.MethodGet,
			responseStatusCode:    http.StatusOK,
			kafkaTopic:            "PostPublicationEvents",
			kafkaPayloadPath:      "testData/input-entries/ContentRelationKafkaPayload.json",
			expectedInResponse:    "data: []", // Receives only heartbeat, not notification, as event type is not "Article"
			notExpectedInResponse: "RELATEDCONTENT",
		},
		{
			name:                  "Update of Article with is_related_content_notification shall notify subscribers with policy INTERNAL_UNSTABLE only",
			xPolicies:             "INTERNAL_UNSTABLE",
			apiKey:                "123456INTERNAL_UNSTABLE",
			subscriberEndpoint:    "/content/notifications-push",
			requestMethod:         http.MethodGet,
			responseStatusCode:    http.StatusOK,
			kafkaTopic:            "PostPublicationEvents",
			kafkaPayloadPath:      "testData/input-entries/ArticleKafkaPayload.json",
			expectedInResponse:    "RELATEDCONTENT", // Received Notification as X-Policy is INTERNAL_UNSTABLE and event type is "Article"
			notExpectedInResponse: "",
		},
		{
			name:                  "Skip Update of Article with is_related_content_notification when subscribers has no policy INTERNAL_UNSTABLE",
			xPolicies:             "EXTERNAL_TEST",
			apiKey:                "123456EXTERNAL_TEST",
			subscriberEndpoint:    "/content/notifications-push",
			requestMethod:         http.MethodGet,
			responseStatusCode:    http.StatusOK,
			kafkaTopic:            "PostPublicationEvents",
			kafkaPayloadPath:      "testData/input-entries/ArticleKafkaPayload.json",
			expectedInResponse:    "data: []", // Receives only heartbeat, as the Policy INTERNAL_UNSTABLE is not set for this API-KEY
			notExpectedInResponse: "RELATEDCONTENT",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			httpResultChannel := make(chan struct {
				resp *http.Response
				err  error
			})
			// Call MakeHTTPRequest for a subscriber in a Go Routine asynchronously
			go func() {
				resp, err := MakeHTTPRequest(t, test.requestMethod, test.subscriberEndpoint, test.xPolicies, test.apiKey) //nolint:bodyclose
				httpResultChannel <- struct {
					resp *http.Response
					err  error
				}{resp, err}
			}()

			// Prepare and send Kafka Event for Notification
			var kafkaMessageBytes []byte
			if test.kafkaPayloadPath != "" {
				kafkaMessageBytes = GetJSONFromFile(t, test.kafkaPayloadPath)
			}

			// Consider Using kafka.FTMessage here instead
			var kMessage FTKafkaMessageMock
			if err := json.Unmarshal(kafkaMessageBytes, &kMessage); err != nil {
				t.Errorf("Error unmarshalling JSON file to Kafka FTMessage: %v", err)
			}

			brokers := os.Getenv("KAFKA_ADDRESS")
			if brokers == "" {
				brokers = "kafka:9092"
			}
			PostKafkaRequest(t, brokers, kafkaTopic, kMessage)

			// Wait for MakeHTTPRequest to Timeout as it is long-pooling request with heartbeat every 30 sec
			log.Printf("Waiting for response(s) to subscriber HTTP Long Pooling Request...")
			result := <-httpResultChannel

			if result.err != nil {
				if test.expectedError == nil {
					t.Fatalf("unexpected error occurred: %v", result.err)
				}

				if result.err.Error() != test.expectedError.Error() {
					t.Fatalf("expected error: %v, got: %v", test.expectedError, result.err)
				}
				return
			}

			// Response Body Close
			defer func() {
				if err := result.resp.Body.Close(); err != nil {
					t.Errorf("Failed to close response body: %v", err)
				}
			}()

			if test.expectedError != nil {
				t.Fatalf("expected error did not occur: %v", test.expectedError)
				return
			}

			if result.resp.StatusCode != test.responseStatusCode {
				t.Errorf("expected status code: %v, but got: %v", test.responseStatusCode, result.resp.StatusCode)
				return
			}

			b, err := io.ReadAll(result.resp.Body)
			if err != nil {
				// We always have error, as there is a TimeOut to Quit of LongPooling request
				// We must check if the Timeout is the only issue of response to the client.
				log.Printf("Timeout interrupts the LongPooling request with response err: %v", err)
			}

			responseBodies := string(b)
			if test.expectedInResponse != "" && !strings.Contains(responseBodies, test.expectedInResponse) {
				t.Errorf("fail: expected response text: %s, not found in response: %s", test.expectedInResponse, responseBodies)
			}

			if test.notExpectedInResponse != "" && strings.Contains(responseBodies, test.notExpectedInResponse) {
				t.Errorf("fail: not expected text: %s, found in response: %s", test.notExpectedInResponse, responseBodies)
			}
		})
	}
}

// Call to promote subscriber for Default (Article) notifications
func MakeHTTPRequest(t *testing.T,
	requestMethod string,
	subscriberEndpoint string,
	xPolicies string,
	apiKey string,
) (*http.Response, error) {
	t.Helper()

	baseURL := os.Getenv("INTERNAL_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Consider adding ?type=Article in case default subscription is "All". See README.md for all supported types.
	req, err := http.NewRequest(requestMethod, baseURL+subscriberEndpoint, nil)
	if err != nil {
		//return nil, fmt.Errorf(error: fail to create request to subscribe for notifications: %v", err)
		t.Fatalf("error: fail to create request to subscribe for notifications: %v", err)
	}

	if xPolicies != "" {
		req.Header.Set("X-Policy", xPolicies)
	}

	if apiKey != "" {
		req.Header.Set("X-Api-Key", apiKey)
	}
	// API Gateway can return X-HTML instead of JSON if requested
	req.Header.Set("Content-Type", "application/json")

	// Call the service to Subscribe for notifications with TIMEOUT 60 seconds
	client := &http.Client{
		Timeout: subscriberTimeout * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error occurred while making request: %v", err)
	}

	return resp, nil
}

// Helper function, which reads from test data JSON file and return the content as byte slice.
func GetJSONFromFile(t *testing.T, pathToJSON string) []byte {
	t.Helper()

	file, err := os.Open(pathToJSON)
	if err != nil {
		t.Fatalf("error: %v while opening file: %v", err, pathToJSON)
	}

	body, err := io.ReadAll(file)
	if err != nil {
		t.Errorf("error: %v while reading file: %v", err, pathToJSON)
	}
	return body
}

// Converts read JSON test file to kafka message and call function to send the event
func PostKafkaRequest(t *testing.T, brokers string, kafkaTopic string, kData FTKafkaMessageMock) {
	t.Helper()
	headers := kData.Headers // map[string]string
	msg := string(kData.Body)

	err := writeToKafka(brokers, kafkaTopic, headers, msg)
	if err != nil {
		t.Errorf("Test failed to write to kafka for Broker: %q, Topic: %q, Error: %v", brokers, kafkaTopic, err)
	}
}

// Original way to create FTMessage is:
// headers := map[string]string{}
// headers["X-Request-Id"] = tid
// headers["Message-Timestamp"] = now
// headers["Content-Type"] = contentType
// headers["Message-Type"] = "cms-content-published"
// headers["Origin-System-Id"] = "http://cmdb.ft.com/systems/cct"
// headers["Message-Id"] = c.uuidCreator.CreateUUID()
func writeToKafka(brokerList string, topic string, headers map[string]string, msg string) error {
	config := kafka.ProducerConfig{
		BrokersConnectionString: brokerList,
		Topic:                   topic,
	}

	producer, err := buildKafkaConsumerProducer(config)
	if err != nil {
		return err
	}

	// Ensure the producer is closed when the function exits
	defer func() {
		if closeErr := producer.Close(); closeErr != nil {
			log.Printf("Failed to close Kafka producer: %v", closeErr)
		}
	}()

	// Create and send the message
	msgToSend := kafka.NewFTMessage(headers, msg)
	err = producer.SendMessage(msgToSend)
	if err != nil {
		log.Println("Failed to send message to Kafka:", err)
		return err
	}

	return nil
}

func buildKafkaConsumerProducer(options kafka.ProducerConfig) (*kafka.Producer, error) {
	producerConfig := kafka.ProducerConfig{
		ClusterArn:              nil,
		BrokersConnectionString: options.BrokersConnectionString,
		Topic:                   options.Topic,
		Options:                 kafka.DefaultProducerOptions(),
	}
	producer, err := kafka.NewProducer(producerConfig)
	if err != nil {
		log.Fatalf("Failed to create Kafka producer for: %#q: %v", options.BrokersConnectionString, err)
		return producer, err
	}

	return producer, nil
}
