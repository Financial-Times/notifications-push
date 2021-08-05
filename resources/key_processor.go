package resources

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/Financial-Times/go-logger/v2"
)

const suffixLen = 10

type keyProcessor struct {
	validationURL string
	policiesURL   string
	client        *http.Client
	log           *logger.UPPLogger
}

func NewKeyProcessor(validationURL, policiesURL string, client *http.Client, log *logger.UPPLogger) KeyProcessor {
	return &keyProcessor{
		validationURL: validationURL,
		policiesURL:   policiesURL,
		client:        client,
		log:           log,
	}
}

type KeyErr struct {
	Msg       string
	Status    int
	KeySuffix string
	// Optional description used for logging.
	Description string
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

func NewKeyErrWithDescription(msg string, status int, key string, description string) *KeyErr {
	return &KeyErr{
		Msg:         msg,
		Status:      status,
		KeySuffix:   key,
		Description: description,
	}
}

func (p *keyProcessor) Validate(ctx context.Context, key string) error {
	if key == "" {
		return NewKeyErr("Empty api key", http.StatusUnauthorized, "")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", p.validationURL, nil)
	if err != nil {
		return NewKeyErrWithDescription("Invalid validation URL", http.StatusInternalServerError, "", err.Error())
	}

	req.Header.Set(apiKeyHeaderField, key)

	//if the api key has more than five characters we want to log the last five
	keySuffix := ""
	if len(key) > suffixLen {
		keySuffix = key[len(key)-suffixLen:]
	}
	p.log.
		WithField("url", req.URL.String()).
		WithField("apiKeyLastChars", keySuffix).
		Info("Calling the API Gateway to validate api key")

	resp, err := p.client.Do(req) //nolint:bodyclose
	if err != nil {
		return NewKeyErrWithDescription("Request to validate api key failed", http.StatusInternalServerError, keySuffix, err.Error())
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var errMessage, errDescription string

	switch resp.StatusCode {
	case http.StatusOK:
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	case http.StatusUnauthorized:
		errMessage = "Invalid api key"
	case http.StatusTooManyRequests:
		errMessage = "Rate limit exceeded"
	case http.StatusForbidden:
		errMessage = "Operation forbidden"
	default:
		errMessage = "Request to validate api key returned an unexpected response"
	}

	msg := struct {
		Error string `json:"error"`
	}{}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		errDescription = err.Error()
	} else {
		err = json.Unmarshal(data, &msg)
		if err != nil {
			errDescription = string(data)
		} else {
			errDescription = msg.Error
		}
	}

	return NewKeyErrWithDescription(errMessage, resp.StatusCode, keySuffix, errDescription)
}

var xPoliciesPattern = regexp.MustCompile(`['"]x-policy['"]\s*:\s*['"](.*)?['"]`)

func (p *keyProcessor) GetPolicies(ctx context.Context, key string) ([]string, error) {
	if key == "" {
		// Sanity check. Policies shouldn't be requested without API key validation.
		return nil, NewKeyErr("Empty api key", http.StatusUnauthorized, "")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", p.policiesURL, nil)
	if err != nil {
		return nil, NewKeyErrWithDescription("Invalid policies URL", http.StatusInternalServerError, "", err.Error())
	}

	req.Header.Set(apiKeyHeaderField, key)

	// Use the last characters of the key for logging purposes.
	var keySuffix string
	if n := len(key); n > suffixLen {
		keySuffix = key[n-suffixLen:]
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, NewKeyErrWithDescription("Request to get API key policies failed", http.StatusInternalServerError, keySuffix, err.Error())
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewKeyErrWithDescription("Reading API Key policies failed", http.StatusInternalServerError, keySuffix, err.Error())
	}

	if resp.StatusCode != http.StatusOK {
		errMsg := "Request to get API key policies returned an unexpected response"
		if resp.StatusCode == http.StatusNotFound {
			errMsg = "API key policies not found"
		}

		return nil, NewKeyErrWithDescription(errMsg, resp.StatusCode, keySuffix, string(data))
	}

	matches := xPoliciesPattern.FindStringSubmatch(string(data))
	if matches == nil {
		return nil, NewKeyErr("Decoding API Key policies failed", http.StatusInternalServerError, keySuffix)
	}

	policies := make([]string, 0)

	for _, xPolicy := range strings.Split(matches[1], ",") {
		xPolicy = strings.TrimSpace(xPolicy)
		if xPolicy != "" {
			policies = append(policies, xPolicy)
		}
	}

	return policies, nil
}
