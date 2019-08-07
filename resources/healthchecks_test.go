package resources

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/gorilla/mux"
)

type HttpClientMock struct {
	GetStatusCodeF func(url string) (int, error)
}

func (m *HttpClientMock) GetStatusCode(url string) (int, error) {
	if m.GetStatusCodeF != nil {
		return m.GetStatusCodeF(url)
	}
	return 0, errors.New("not implemented")
}

type KafkaConsumerMock struct {
	ConnectivityCheckF func() error
}

func (m *KafkaConsumerMock) ConnectivityCheck() error {
	if m.ConnectivityCheckF != nil {
		return m.ConnectivityCheckF()
	}
	return errors.New("Not implemented")
}

func TestHealthcheck(t *testing.T) {

	tests := []struct {
		name              string
		httpClientMock    *HttpClientMock
		kafkaConsumerMock *KafkaConsumerMock
		expectedOk        bool
	}{
		{
			name: "Success - both ok",
			httpClientMock: &HttpClientMock{
				GetStatusCodeF: func(url string) (int, error) {
					return 200, nil
				},
			},
			kafkaConsumerMock: &KafkaConsumerMock{
				ConnectivityCheckF: func() error {
					return nil
				},
			},
			expectedOk: true,
		},
		{
			name: "Fail because of kafka",
			httpClientMock: &HttpClientMock{
				GetStatusCodeF: func(url string) (int, error) {
					return 200, nil
				},
			},
			kafkaConsumerMock: &KafkaConsumerMock{
				ConnectivityCheckF: func() error {
					return errors.New("Sample error")
				},
			},
			expectedOk: false,
		},
		{
			name: "Fail because of ApiGateway endpoint",
			httpClientMock: &HttpClientMock{
				GetStatusCodeF: func(url string) (int, error) {
					return 404, nil
				},
			},
			kafkaConsumerMock: &KafkaConsumerMock{
				ConnectivityCheckF: func() error {
					return nil
				},
			},
			expectedOk: false,
		},
		{
			name: "Sucess when APIGateway returns 500",
			httpClientMock: &HttpClientMock{
				GetStatusCodeF: func(url string) (int, error) {
					return 403, nil
				},
			},
			kafkaConsumerMock: &KafkaConsumerMock{
				ConnectivityCheckF: func() error {
					return nil
				},
			},
			expectedOk: true,
		},
	}

	for _, test := range tests {
		fmt.Printf("Runnning test %s \n", test.name)

		hc := NewHealthCheck(test.kafkaConsumerMock, "randomAddress", test.httpClientMock)

		req, err := http.NewRequest("GET", "/__health", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		servicesRouter := mux.NewRouter()
		servicesRouter.HandleFunc("/__health", hc.Health()).Methods("GET")

		servicesRouter.ServeHTTP(rr, req)

		buf := new(bytes.Buffer)
		buf.ReadFrom(rr.Body)
		response := buf.String()

		var healthResult fthealth.HealthResult
		err = json.Unmarshal([]byte(response), &healthResult)
		if err != nil {
			t.Fatal(err)
		}

		if healthResult.Ok != test.expectedOk {
			t.Fatalf("Incorrect test result; expectedOk was %v, got %v", test.expectedOk, healthResult.Ok)
		}

	}

}
