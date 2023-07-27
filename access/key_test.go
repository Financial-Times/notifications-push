package access

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/notifications-push/v5/mocks"

	"github.com/stretchr/testify/assert"
)

var apiGatewayRawURL = "http://api.gateway.url"

func urlFromRawString(r string) *url.URL {
	parsedURL, _ := url.Parse(r)

	return parsedURL
}

func TestIsValidApiKeySuccessful(t *testing.T) {
	t.Parallel()

	apiGatewayURL := urlFromRawString(apiGatewayRawURL)
	client := mocks.ClientWithResponseCode(http.StatusOK)

	l := logger.NewUPPLogger("TEST", "PANIC")
	p := NewKeyProcessor(apiGatewayURL, client, l)

	assert.NoError(t, p.Validate(context.Background(), "long_testing_key"), "Validate should not fail for valid key")
}

func TestIsValidApiKeyError(t *testing.T) {
	t.Parallel()

	apiGatewayURL := urlFromRawString(apiGatewayRawURL)
	client := mocks.ClientWithError(fmt.Errorf("client error"))

	l := logger.NewUPPLogger("TEST", "PANIC")
	p := NewKeyProcessor(apiGatewayURL, client, l)

	err := p.Validate(context.Background(), "long_testing_key")
	assert.Error(t, err, "Validate should fail for invalid key")
	keyErr := &KeyErr{}
	assert.True(t, errors.As(err, &keyErr), "Validate should return KeyErr error")
	assert.Equal(t, "Request to validate api key failed", keyErr.Msg)
	assert.Equal(t, http.StatusInternalServerError, keyErr.Status)
}

func TestIsValidApiKeyEmptyKey(t *testing.T) {
	t.Parallel()

	apiGatewayURL := urlFromRawString(apiGatewayRawURL)
	client := mocks.ClientWithError(fmt.Errorf("client error"))

	l := logger.NewUPPLogger("TEST", "PANIC")
	p := NewKeyProcessor(apiGatewayURL, client, l)

	err := p.Validate(context.Background(), "")
	assert.Error(t, err, "Validate should fail for invalid key")
	keyErr := &KeyErr{}
	assert.True(t, errors.As(err, &keyErr), "Validate should return KeyErr error")
	assert.Equal(t, "Empty api key", keyErr.Msg)
	assert.Equal(t, http.StatusUnauthorized, keyErr.Status)
}

func TestIsValidApiKeyResponseUnauthorized(t *testing.T) {
	t.Parallel()

	apiGatewayURL := urlFromRawString(apiGatewayRawURL)
	client := mocks.ClientWithResponseCode(http.StatusUnauthorized)

	l := logger.NewUPPLogger("TEST", "PANIC")
	p := NewKeyProcessor(apiGatewayURL, client, l)

	err := p.Validate(context.Background(), "long_testing_key")
	assert.Error(t, err, "Validate should fail for invalid key")
	keyErr := &KeyErr{}
	assert.True(t, errors.As(err, &keyErr), "Validate should return KeyErr error")
	assert.Equal(t, "Invalid api key", keyErr.Msg)
	assert.Equal(t, http.StatusUnauthorized, keyErr.Status)
}

func TestIsValidApiKeyResponseTooManyRequests(t *testing.T) {
	t.Parallel()

	apiGatewayURL := urlFromRawString(apiGatewayRawURL)
	client := mocks.ClientWithResponseCode(http.StatusTooManyRequests)

	l := logger.NewUPPLogger("TEST", "PANIC")
	p := NewKeyProcessor(apiGatewayURL, client, l)

	err := p.Validate(context.Background(), "long_testing_key")
	assert.Error(t, err, "Validate should fail for invalid key")
	keyErr := &KeyErr{}
	assert.True(t, errors.As(err, &keyErr), "Validate should return KeyErr error")
	assert.Equal(t, "Rate limit exceeded", keyErr.Msg)
	assert.Equal(t, http.StatusTooManyRequests, keyErr.Status)
}

func TestIsValidApiKeyResponseForbidden(t *testing.T) {
	t.Parallel()

	apiGatewayURL := urlFromRawString(apiGatewayRawURL)
	client := mocks.ClientWithResponseCode(http.StatusForbidden)

	l := logger.NewUPPLogger("TEST", "PANIC")
	p := NewKeyProcessor(apiGatewayURL, client, l)

	err := p.Validate(context.Background(), "long_testing_key")
	assert.Error(t, err, "Validate should fail for invalid key")
	keyErr := &KeyErr{}
	assert.True(t, errors.As(err, &keyErr), "Validate should return KeyErr error")
	assert.Equal(t, "Operation forbidden", keyErr.Msg)
	assert.Equal(t, http.StatusForbidden, keyErr.Status)
}

func TestIsValidApiKeyResponseInternalServerError(t *testing.T) {
	t.Parallel()

	apiGatewayURL := urlFromRawString(apiGatewayRawURL)
	client := mocks.ClientWithResponseCode(http.StatusInternalServerError)

	l := logger.NewUPPLogger("TEST", "PANIC")
	p := NewKeyProcessor(apiGatewayURL, client, l)

	err := p.Validate(context.Background(), "long_testing_key")
	assert.Error(t, err, "Validate should fail for invalid key")
	keyErr := &KeyErr{}
	assert.True(t, errors.As(err, &keyErr), "Validate should return KeyErr error")
	assert.Equal(t, "Request to validate api key returned an unexpected response", keyErr.Msg)
	assert.Equal(t, http.StatusInternalServerError, keyErr.Status)
}

func TestIsValidApiKeyResponseOtherServerError(t *testing.T) {
	t.Parallel()

	apiGatewayURL := urlFromRawString(apiGatewayRawURL)
	client := mocks.ClientWithResponseCode(http.StatusGatewayTimeout)

	l := logger.NewUPPLogger("TEST", "PANIC")
	p := NewKeyProcessor(apiGatewayURL, client, l)

	err := p.Validate(context.Background(), "long_testing_key")
	assert.Error(t, err, "Validate should fail for invalid key")
	keyErr := &KeyErr{}
	assert.True(t, errors.As(err, &keyErr), "Validate should return KeyErr error")
	assert.Equal(t, "Request to validate api key returned an unexpected response", keyErr.Msg)
	assert.Equal(t, http.StatusGatewayTimeout, keyErr.Status)
}
