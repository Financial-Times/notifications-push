package access

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/Financial-Times/go-logger/v2"
)

type KeyProcessor struct {
	client *http.Client
	url    *url.URL
	log    *logger.UPPLogger
}

func NewKeyProcessor(u *url.URL, c *http.Client, l *logger.UPPLogger) *KeyProcessor {
	return &KeyProcessor{
		url:    u,
		client: c,
		log:    l,
	}
}

func (kp *KeyProcessor) Validate(ctx context.Context, key string) error {
	if key == "" {
		return NewKeyErr("Empty api key", http.StatusUnauthorized, "", "")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", kp.url.String(), nil)
	if err != nil {
		kp.log.WithField("url", kp.url).WithError(err).Error("Invalid URL for api key validation")
		return NewKeyErr("Invalid URL", http.StatusInternalServerError, "", "")
	}

	req.Header.Set(apiKeyHeaderField, key)

	keySuffix := keySuffixLogging(key)

	kp.log.WithField("url", req.URL.String()).WithField("apiKeyLastChars", keySuffix).Info("Calling the API Gateway to validate api key")

	resp, err := kp.client.Do(req)
	if err != nil {
		kp.log.WithField("url", req.URL.String()).WithField("apiKeyLastChars", keySuffix).WithError(err).Error("Cannot send request to the API Gateway")
		return NewKeyErr("Request to validate api key failed", http.StatusInternalServerError, keySuffix, "")
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return kp.logFailedRequest(resp, keySuffix)
	}

	return nil
}

func (kp *KeyProcessor) logFailedRequest(resp *http.Response, keySuffix string) *KeyErr {
	msg := struct {
		Error string `json:"error"`
	}{}
	responseBody := ""
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		kp.log.WithField("apiKeyLastChars", keySuffix).WithError(err).Errorf("Getting API Gateway response body failed")
	} else {
		err = json.Unmarshal(data, &msg)
		if err != nil {
			kp.log.WithField("apiKeyLastChars", keySuffix).Errorf("Decoding API Gateway response body as json failed: %v", err)
			responseBody = string(data[:])
		}
	}
	errMsg := ""
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		kp.log.WithField("apiKeyLastChars", keySuffix).Errorf("Invalid api key: %s", responseBody)
		errMsg = "Invalid api key"
	case http.StatusTooManyRequests:
		kp.log.WithField("apiKeyLastChars", keySuffix).Errorf("API key rate limit exceeded: %s", responseBody)
		errMsg = "Rate limit exceeded"
	case http.StatusForbidden:
		kp.log.WithField("apiKeyLastChars", keySuffix).Errorf("Operation forbidden: %s", responseBody)
		errMsg = "Operation forbidden"
	default:
		kp.log.WithField("apiKeyLastChars", keySuffix).Errorf("Received unexpected status code from the API Gateway: %d, error message: %v", resp.StatusCode, responseBody)
		errMsg = "Request to validate api key returned an unexpected response"
	}

	return NewKeyErr(errMsg, resp.StatusCode, keySuffix, "")
}

type KeyErr struct {
	Msg         string
	Status      int
	KeySuffix   string
	Description string
}

func NewKeyErr(m string, s int, k string, d string) *KeyErr {
	return &KeyErr{
		Msg:         m,
		Status:      s,
		KeySuffix:   k,
		Description: d,
	}
}

func (e *KeyErr) Error() string {
	return e.Msg
}
