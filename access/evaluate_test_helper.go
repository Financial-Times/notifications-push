package access

import (
	"net/http"
	"os"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/opa-client-go"
)

func GetOPAAgentForTesting(l *logger.UPPLogger) Agent {
	url := os.Getenv("OPA_URL")
	if url == "" {
		url = "http://localhost:8181"
	}
	paths := map[string]string{
		NotificationPush: "notifications_push/special_content",
	}

	c := http.DefaultClient

	opaClient := opa.NewOpenPolicyAgentClient(url, paths, opa.WithLogger(l), opa.WithHttpClient(c))
	a := NewOpenPolicyAgent(opaClient, l)

	return a
}
