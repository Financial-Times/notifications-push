package resources

import (
	"context"
	"fmt"
	"net/http"
	"time"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/service-status-go/gtg"
)

const panicGuideURL = "https://runbooks.in.ft.com/upp-notifications-push"

type RequestStatusFn func(ctx context.Context, url string) (int, error)

type kafkaConsumer interface {
	ConnectivityCheck() error
	MonitorCheck() error
}

type HealthCheck struct {
	consumer             kafkaConsumer
	StatusFunc           RequestStatusFn
	apiGatewayGTGAddress string
	serviceName          string
}

func NewHealthCheck(kafkaConsumer kafkaConsumer, apiGatewayGTGAddress string, statusFunc RequestStatusFn, serviceName string) *HealthCheck {
	return &HealthCheck{
		consumer:             kafkaConsumer,
		apiGatewayGTGAddress: apiGatewayGTGAddress,
		StatusFunc:           statusFunc,
		serviceName:          serviceName,
	}
}

func (h *HealthCheck) Health() func(w http.ResponseWriter, r *http.Request) {
	var checks []fthealth.Check
	checks = append(checks, h.kafkaConnectivityCheck())
	checks = append(checks, h.kafkaLagCheck())
	checks = append(checks, h.apiGatewayCheck())

	hc := fthealth.TimedHealthCheck{
		HealthCheck: fthealth.HealthCheck{
			SystemCode:  "upp-notifications-push",
			Name:        h.serviceName,
			Description: "Checks if all the dependent services are reachable and healthy.",
			Checks:      checks,
		},
		Timeout: 10 * time.Second,
	}
	return fthealth.Handler(hc)
}

func (h *HealthCheck) kafkaLagCheck() fthealth.Check {
	return fthealth.Check{
		ID:               "kafka-lagcheck",
		Name:             "KafkaLagcheck",
		Severity:         3,
		BusinessImpact:   "Notifications about newly modified/published content will be delayed.",
		TechnicalSummary: "Kafka is lagging",
		PanicGuide:       panicGuideURL,
		Checker:          h.checkKafkaConsumerLag,
	}
}

func (h *HealthCheck) kafkaConnectivityCheck() fthealth.Check {
	return fthealth.Check{
		ID:               "kafka-reachable",
		Name:             "KafkaReachable",
		Severity:         1,
		BusinessImpact:   "Notifications about newly modified/published content will not reach this app, nor will they reach its clients.",
		TechnicalSummary: "Kafka is not reachable/healthy",
		PanicGuide:       panicGuideURL,
		Checker:          h.checkKafkaConsumerReachable,
	}
}

func (h *HealthCheck) GTG() gtg.Status {
	if _, err := h.checkKafkaConsumerReachable(); err != nil {
		return gtg.Status{GoodToGo: false, Message: err.Error()}
	}

	if _, err := h.checkAPIGatewayService(); err != nil {
		return gtg.Status{GoodToGo: false, Message: err.Error()}
	}

	return gtg.Status{GoodToGo: true}
}

func (h *HealthCheck) checkKafkaConsumerReachable() (string, error) {
	err := h.consumer.ConnectivityCheck()
	if err == nil {
		return "Connectivity to kafka is OK.", nil
	}
	return "", err
}

func (h *HealthCheck) checkKafkaConsumerLag() (string, error) {
	err := h.consumer.MonitorCheck()
	if err == nil {
		return "Kafka consumer is not lagging", nil
	}
	return "", err
}

// checks if apiGateway service is available
func (h *HealthCheck) apiGatewayCheck() fthealth.Check {
	return fthealth.Check{
		ID:               "api-gateway-check",
		Name:             "ApiGatewayCheck",
		Severity:         1,
		BusinessImpact:   "If apiGateway service is not available, consumer's helthcheck will return false ",
		TechnicalSummary: "Checking if apiGateway service is available or not",
		PanicGuide:       panicGuideURL,
		Checker:          h.checkAPIGatewayService,
	}
}

func (h *HealthCheck) checkAPIGatewayService() (string, error) {
	if h.StatusFunc == nil {
		return "", fmt.Errorf("no status func")
	}
	statusCode, err := h.StatusFunc(context.Background(), h.apiGatewayGTGAddress)
	if err != nil {
		return "", err
	}

	if statusCode == http.StatusOK {
		return "ApiGateway service is working", nil
	}

	return "", fmt.Errorf("unable to verify ApiGateway service is working")
}
