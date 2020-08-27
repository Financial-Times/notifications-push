package resources_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	logger "github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/notifications-push/v5/mocks"
	"github.com/Financial-Times/notifications-push/v5/resources"
	"github.com/stretchr/testify/assert"
)

func TestIsValidApiKeySuccessful(t *testing.T) {
	t.Parallel()

	client := mocks.ClientWithResponseCode(http.StatusOK)

	l := logger.NewUPPLogger("TEST", "PANIC")
	v := resources.NewKeyValidator("http://api.gateway.url", client, l)

	assert.NoError(t, v.Validate(context.Background(), "long_testing_key"), "Validate should not fail for valid key")
}

func TestIsValidApiKeyError(t *testing.T) {
	t.Parallel()

	client := mocks.ClientWithError(errors.New("client error"))

	l := logger.NewUPPLogger("TEST", "PANIC")
	v := resources.NewKeyValidator("http://api.gateway.url", client, l)

	err := v.Validate(context.Background(), "long_testing_key")
	assert.Error(t, err, "Validate should fail for invalid key")
	keyErr := &resources.KeyErr{}
	assert.True(t, errors.As(err, &keyErr), "Validate should return KeyErr error")
	assert.Equal(t, "Request to validate api key failed", keyErr.Msg)
	assert.Equal(t, http.StatusInternalServerError, keyErr.Status)
}

func TestIsValidApiKeyEmptyKey(t *testing.T) {
	t.Parallel()

	client := mocks.ClientWithError(errors.New("client error"))

	l := logger.NewUPPLogger("TEST", "PANIC")
	v := resources.NewKeyValidator("http://api.gateway.url", client, l)

	err := v.Validate(context.Background(), "")
	assert.Error(t, err, "Validate should fail for invalid key")
	keyErr := &resources.KeyErr{}
	assert.True(t, errors.As(err, &keyErr), "Validate should return KeyErr error")
	assert.Equal(t, "Empty api key", keyErr.Msg)
	assert.Equal(t, http.StatusUnauthorized, keyErr.Status)
}

func TestIsValidApiInvalidGatewayURL(t *testing.T) {
	t.Parallel()

	client := mocks.ClientWithError(errors.New("client error"))

	l := logger.NewUPPLogger("TEST", "PANIC")
	v := resources.NewKeyValidator("://api.gateway.url", client, l)

	err := v.Validate(context.Background(), "long_testing_key")
	assert.Error(t, err, "Validate should fail for invalid key")
	keyErr := &resources.KeyErr{}
	assert.True(t, errors.As(err, &keyErr), "Validate should return KeyErr error")
	assert.Equal(t, "Invalid URL", keyErr.Msg)
	assert.Equal(t, http.StatusInternalServerError, keyErr.Status)
}

func TestIsValidApiKeyResponseUnauthorized(t *testing.T) {
	t.Parallel()

	client := mocks.ClientWithResponseCode(http.StatusUnauthorized)
	l := logger.NewUPPLogger("TEST", "PANIC")
	v := resources.NewKeyValidator("http://api.gateway.url", client, l)

	err := v.Validate(context.Background(), "long_testing_key")
	assert.Error(t, err, "Validate should fail for invalid key")
	keyErr := &resources.KeyErr{}
	assert.True(t, errors.As(err, &keyErr), "Validate should return KeyErr error")
	assert.Equal(t, "Invalid api key", keyErr.Msg)
	assert.Equal(t, http.StatusUnauthorized, keyErr.Status)
}

func TestIsValidApiKeyResponseTooManyRequests(t *testing.T) {
	t.Parallel()

	client := mocks.ClientWithResponseCode(http.StatusTooManyRequests)
	l := logger.NewUPPLogger("TEST", "PANIC")
	v := resources.NewKeyValidator("http://api.gateway.url", client, l)

	err := v.Validate(context.Background(), "long_testing_key")
	assert.Error(t, err, "Validate should fail for invalid key")
	keyErr := &resources.KeyErr{}
	assert.True(t, errors.As(err, &keyErr), "Validate should return KeyErr error")
	assert.Equal(t, "Rate limit exceeded", keyErr.Msg)
	assert.Equal(t, http.StatusTooManyRequests, keyErr.Status)
}

func TestIsValidApiKeyResponseForbidden(t *testing.T) {
	t.Parallel()

	client := mocks.ClientWithResponseCode(http.StatusForbidden)
	l := logger.NewUPPLogger("TEST", "PANIC")
	v := resources.NewKeyValidator("http://api.gateway.url", client, l)

	err := v.Validate(context.Background(), "long_testing_key")
	assert.Error(t, err, "Validate should fail for invalid key")
	keyErr := &resources.KeyErr{}
	assert.True(t, errors.As(err, &keyErr), "Validate should return KeyErr error")
	assert.Equal(t, "Operation forbidden", keyErr.Msg)
	assert.Equal(t, http.StatusForbidden, keyErr.Status)
}

func TestIsValidApiKeyResponseInternalServerError(t *testing.T) {
	t.Parallel()

	client := mocks.ClientWithResponseCode(http.StatusInternalServerError)
	l := logger.NewUPPLogger("TEST", "PANIC")
	v := resources.NewKeyValidator("http://api.gateway.url", client, l)

	err := v.Validate(context.Background(), "long_testing_key")
	assert.Error(t, err, "Validate should fail for invalid key")
	keyErr := &resources.KeyErr{}
	assert.True(t, errors.As(err, &keyErr), "Validate should return KeyErr error")
	assert.Equal(t, "Request to validate api key returned an unexpected response", keyErr.Msg)
	assert.Equal(t, http.StatusInternalServerError, keyErr.Status)
}

func TestIsValidApiKeyResponseOtherServerError(t *testing.T) {
	t.Parallel()

	client := mocks.ClientWithResponseCode(http.StatusGatewayTimeout)
	l := logger.NewUPPLogger("TEST", "PANIC")
	v := resources.NewKeyValidator("http://api.gateway.url", client, l)

	err := v.Validate(context.Background(), "long_testing_key")
	assert.Error(t, err, "Validate should fail for invalid key")
	keyErr := &resources.KeyErr{}
	assert.True(t, errors.As(err, &keyErr), "Validate should return KeyErr error")
	assert.Equal(t, "Request to validate api key returned an unexpected response", keyErr.Msg)
	assert.Equal(t, http.StatusGatewayTimeout, keyErr.Status)
}
