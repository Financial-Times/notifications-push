package resources

import (
	"bytes"
	"net/http"
	"time"

	"errors"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/kafka-client-go/kafka"
	"github.com/Financial-Times/service-status-go/gtg"
)

type HealthCheck struct {
	Consumer kafka.Consumer
	ApiGatewayGTGAddress string
}

func NewHealthCheck(kafkaConsumer kafka.Consumer, apiGatewayGTGAddress string) *HealthCheck {
	return &HealthCheck{
		Consumer: kafkaConsumer,
		ApiGatewayGTGAddress: apiGatewayGTGAddress,
	}
}

func (h *HealthCheck) Health() func(w http.ResponseWriter, r *http.Request) {

	var checks []fthealth.Check
	checks = append(checks, h.queueCheck())
	checks = append(checks, h.apiGatewayCheck())

	hc := fthealth.TimedHealthCheck{
		HealthCheck: fthealth.HealthCheck{
			SystemCode:  "upp-notifications-push",
			Name:        "Notifications Push",
			Description: "Checks if all the dependent services are reachable and healthy.",
			Checks:      checks,
		},
		Timeout: 10 * time.Second,
	}
	return fthealth.Handler(hc)
}

// Check is the the NotificationsPushHealthcheck method that checks if the kafka queue is available
func (h *HealthCheck) queueCheck() fthealth.Check {
	return fthealth.Check{
		ID:               "message-queue-reachable",
		Name:             "MessageQueueReachable",
		Severity:         1,
		BusinessImpact:   "Notifications about newly modified/published content will not reach this app, nor will they reach its clients.",
		TechnicalSummary: "Message queue is not reachable/healthy",
		PanicGuide:       "https://dewey.ft.com/upp-notifications-push.html",
		Checker:          h.checkAggregateMessageQueueReachable,
	}
}

func (h *HealthCheck) GTG() gtg.Status {
	if _, err := h.checkAggregateMessageQueueReachable(); err != nil {
		return gtg.Status{GoodToGo: false, Message: err.Error()}
	}

	if _, err := h.checkApiGatewayService(); err != nil {
		return gtg.Status{GoodToGo: false, Message: err.Error()}
	}

	return gtg.Status{GoodToGo: true}
}

func (h *HealthCheck) checkAggregateMessageQueueReachable() (string, error) {
	// ISSUE: consumer's helthcheck always returns true
	err := h.Consumer.ConnectivityCheck()
	if err == nil {
		return "Connectivity to kafka is OK.", nil
	}

	return "Error connecting to kafka", errors.New("Error connecting to kafka queue")
}

// checks if apiGateway service is available
func (h *HealthCheck) apiGatewayCheck() fthealth.Check {
	return fthealth.Check{
		ID:               "api-gateway-check",
		Name:             "ApiGatewayCheck",
		Severity:         1,
		BusinessImpact:   "If apiGateway service is not available, consumer's helthcheck will return false ",
		TechnicalSummary: "Checking if apiGateway service is available or not",
		PanicGuide:       "https://dewey.ft.com/upp-notifications-push.html",
		Checker:          h.checkApiGatewayService,
	}
}


func (h *HealthCheck) checkApiGatewayService() (string, error) {

	r, err := http.NewRequest("GET", h.ApiGatewayGTGAddress, bytes.NewReader([]byte("")))
	if err != nil {
		return "", errors.New("Error creating request")
	}

	httpClient := &http.Client{Timeout: time.Second * 15}
	res, err := httpClient.Do(r)
	if err != nil {
		return "", errors.New("Error making http request to GTG endpoint")
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK || res.StatusCode == http.StatusForbidden {
		return "ApiGateway service is working", nil
	}

	return "", errors.New("Unable to verify ApiGateway service is working")

}
