package resources

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"

	logger "github.com/Financial-Times/go-logger/v2"
)

const APIKeyHeaderField = "X-Api-Key"
const suffixLen = 10

type KeyValidator struct {
	APIGatewayURL string
	client        *http.Client
	log           *logger.UPPLogger
}

func NewKeyValidator(url string, client *http.Client, log *logger.UPPLogger) *KeyValidator {
	return &KeyValidator{
		APIGatewayURL: url,
		client:        client,
		log:           log,
	}
}

type KeyErr struct {
	Msg       string
	Status    int
	KeySuffix string
}

func (e *KeyErr) Error() string {
	return e.Msg
}

func NewKeyErr(msg string, status int, key string) *KeyErr {
	return &KeyErr{
		Msg:       msg,
		Status:    status,
		KeySuffix: key,
	}
}

func (v *KeyValidator) Validate(ctx context.Context, key string) error {
	if key == "" {
		return NewKeyErr("Empty api key", http.StatusUnauthorized, "")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", v.APIGatewayURL, nil)
	if err != nil {
		v.log.WithField("url", v.APIGatewayURL).WithError(err).Error("Invalid URL for api key validation")
		return NewKeyErr("Invalid URL", http.StatusInternalServerError, "")
	}

	req.Header.Set(APIKeyHeaderField, key)

	//if the api key has more than five characters we want to log the last five
	keySuffix := ""
	if len(key) > suffixLen {
		keySuffix = key[len(key)-suffixLen:]
	}
	v.log.WithField("url", req.URL.String()).WithField("apiKeyLastChars", keySuffix).Info("Calling the API Gateway to validate api key")

	resp, err := v.client.Do(req) //nolint:bodyclose
	if err != nil {
		v.log.WithField("url", req.URL.String()).WithField("apiKeyLastChars", keySuffix).WithError(err).Error("Cannot send request to the API Gateway")
		return NewKeyErr("Request to validate api key failed", http.StatusInternalServerError, keySuffix)
	}
	defer func() {
		_, _ = io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return v.logFailedRequest(resp, keySuffix)
	}

	return nil
}

func (v *KeyValidator) logFailedRequest(resp *http.Response, keySuffix string) *KeyErr {
	msg := struct {
		Error string `json:"error"`
	}{}
	responseBody := ""
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		v.log.WithField("apiKeyLastChars", keySuffix).WithError(err).Warnf("Getting API Gateway response body failed")
	} else {
		err = json.Unmarshal(data, &msg)
		if err != nil {
			v.log.WithField("apiKeyLastChars", keySuffix).Warnf("Decoding API Gateway response body as json failed: %v", err)
			responseBody = string(data[:])
		}
	}
	errMsg := ""
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		v.log.WithField("apiKeyLastChars", keySuffix).Errorf("Invalid api key: %v", responseBody)
		errMsg = "Invalid api key"
	case http.StatusTooManyRequests:
		v.log.WithField("apiKeyLastChars", keySuffix).Errorf("API key rate limit exceeded: %v", responseBody)
		errMsg = "Rate limit exceeded"
	case http.StatusForbidden:
		v.log.WithField("apiKeyLastChars", keySuffix).Errorf("Operation forbidden: %v", responseBody)
		errMsg = "Operation forbidden"
	default:
		v.log.WithField("apiKeyLastChars", keySuffix).Errorf("Received unexpected status code from the API Gateway: %d, error message: %v", resp.StatusCode, responseBody)
		errMsg = "Request to validate api key returned an unexpected response"
	}

	return NewKeyErr(errMsg, resp.StatusCode, keySuffix)
}
