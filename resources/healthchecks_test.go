package resources

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Financial-Times/notifications-push/v5/mocks"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

func TestHealthcheck(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		statusFn          RequestStatusFn
		kafkaConsumerMock *mocks.KafkaConsumer
		expectedStatus    int
		expectedBody      string
	}{
		"Success - all ok": {
			statusFn: func(ctx context.Context, url string) (int, error) {
				return http.StatusOK, nil
			},
			kafkaConsumerMock: &mocks.KafkaConsumer{
				ConnectivityCheckF: func() error {
					return nil
				},
				MonitorCheckF: func() error {
					return nil
				},
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"ok":true}`,
		},
		"Fail because of kafka connectivity": {
			statusFn: func(ctx context.Context, url string) (int, error) {
				return http.StatusOK, nil
			},
			kafkaConsumerMock: &mocks.KafkaConsumer{
				ConnectivityCheckF: func() error {
					return fmt.Errorf("sample error")
				},
				MonitorCheckF: func() error {
					return nil
				},
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"ok":false,"severity":1}`,
		},
		"Fail because of kafka lag": {
			statusFn: func(ctx context.Context, url string) (int, error) {
				return http.StatusOK, nil
			},
			kafkaConsumerMock: &mocks.KafkaConsumer{
				ConnectivityCheckF: func() error {
					return nil
				},
				MonitorCheckF: func() error {
					return fmt.Errorf("lag error")
				},
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"ok":false,"severity":3}`,
		},
		"Fail because of ApiGateway does not return 200 OK": {
			statusFn: func(ctx context.Context, url string) (int, error) {
				return http.StatusForbidden, nil
			},
			kafkaConsumerMock: &mocks.KafkaConsumer{
				ConnectivityCheckF: func() error {
					return nil
				},
				MonitorCheckF: func() error {
					return nil
				},
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"ok":false,"severity":1}`,
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			hc := NewHealthCheck(test.kafkaConsumerMock, "randomAddress", test.statusFn, "notifications-push")

			req, err := http.NewRequest("GET", "/__health", nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			servicesRouter := mux.NewRouter()
			servicesRouter.HandleFunc("/__health", hc.Health()).Methods("GET")

			servicesRouter.ServeHTTP(rr, req)

			buf := new(bytes.Buffer)
			_, _ = buf.ReadFrom(rr.Body)
			body := buf.String()

			assert.Equal(t, test.expectedStatus, rr.Code, name+" failed")
			assert.Contains(t, body, test.expectedBody, name+" failed")
		})
	}
}
