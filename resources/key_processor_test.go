package resources_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/notifications-push/v5/mocks"
	"github.com/Financial-Times/notifications-push/v5/resources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const apiGatewayURL = "http://api.gateway.url"

func TestIsValidApiKeySuccessful(t *testing.T) {
	t.Parallel()

	client := mocks.ClientWithResponseCode(http.StatusOK)

	l := logger.NewUPPLogger("TEST", "PANIC")
	p := resources.NewKeyProcessor(apiGatewayURL, apiGatewayURL, client, l)

	assert.NoError(t, p.Validate(context.Background(), "long_testing_key"), "Validate should not fail for valid key")
}

func TestIsValidApiKeyError(t *testing.T) {
	t.Parallel()

	client := mocks.ClientWithError(errors.New("client error"))

	l := logger.NewUPPLogger("TEST", "PANIC")
	p := resources.NewKeyProcessor(apiGatewayURL, apiGatewayURL, client, l)

	err := p.Validate(context.Background(), "long_testing_key")
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
	p := resources.NewKeyProcessor(apiGatewayURL, apiGatewayURL, client, l)

	err := p.Validate(context.Background(), "")
	assert.Error(t, err, "Validate should fail for invalid key")
	keyErr := &resources.KeyErr{}
	assert.True(t, errors.As(err, &keyErr), "Validate should return KeyErr error")
	assert.Equal(t, "Empty api key", keyErr.Msg)
	assert.Equal(t, http.StatusUnauthorized, keyErr.Status)
}

func TestIsValidApiInvalidValidationGatewayURL(t *testing.T) {
	t.Parallel()

	client := mocks.ClientWithError(errors.New("client error"))

	l := logger.NewUPPLogger("TEST", "PANIC")
	p := resources.NewKeyProcessor(":"+apiGatewayURL, apiGatewayURL, client, l)

	err := p.Validate(context.Background(), "long_testing_key")
	assert.Error(t, err, "Validate should fail for invalid key")
	keyErr := &resources.KeyErr{}
	assert.True(t, errors.As(err, &keyErr), "Validate should return KeyErr error")
	assert.Equal(t, "Invalid validation URL", keyErr.Msg)
	assert.Equal(t, http.StatusInternalServerError, keyErr.Status)
}

func TestIsValidApiKeyResponseUnauthorized(t *testing.T) {
	t.Parallel()

	client := mocks.ClientWithResponseCode(http.StatusUnauthorized)
	l := logger.NewUPPLogger("TEST", "PANIC")
	p := resources.NewKeyProcessor(apiGatewayURL, apiGatewayURL, client, l)

	err := p.Validate(context.Background(), "long_testing_key")
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
	p := resources.NewKeyProcessor(apiGatewayURL, apiGatewayURL, client, l)

	err := p.Validate(context.Background(), "long_testing_key")
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
	p := resources.NewKeyProcessor(apiGatewayURL, apiGatewayURL, client, l)

	err := p.Validate(context.Background(), "long_testing_key")
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
	p := resources.NewKeyProcessor(apiGatewayURL, apiGatewayURL, client, l)

	err := p.Validate(context.Background(), "long_testing_key")
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
	p := resources.NewKeyProcessor(apiGatewayURL, apiGatewayURL, client, l)

	err := p.Validate(context.Background(), "long_testing_key")
	assert.Error(t, err, "Validate should fail for invalid key")
	keyErr := &resources.KeyErr{}
	assert.True(t, errors.As(err, &keyErr), "Validate should return KeyErr error")
	assert.Equal(t, "Request to validate api key returned an unexpected response", keyErr.Msg)
	assert.Equal(t, http.StatusGatewayTimeout, keyErr.Status)
}

func TestKeyProcessor_GetPolicies(t *testing.T) {
	t.Parallel()

	const (
		apiKey       = "long_test_key_x"
		apiKeySuffix = "test_key_x"
	)

	tests := []struct {
		name             string
		httpClient       *http.Client
		validationURL    string
		policiesURL      string
		apiKey           string
		expectedErr      *resources.KeyErr
		expectedPolicies []string
	}{
		{
			name:          "Empty API Key",
			httpClient:    mocks.ClientWithResponseCode(http.StatusOK),
			validationURL: apiGatewayURL,
			policiesURL:   apiGatewayURL,
			apiKey:        "",
			expectedErr:   resources.NewKeyErr("Empty api key", http.StatusUnauthorized, ""),
		},
		{
			name:          "Invalid Policies URL",
			httpClient:    mocks.ClientWithResponseCode(http.StatusOK),
			validationURL: apiGatewayURL,
			policiesURL:   ":" + apiGatewayURL,
			apiKey:        apiKey,
			expectedErr:   resources.NewKeyErr("Invalid policies URL", http.StatusInternalServerError, ""),
		},
		{
			name:          "HTTP Client Error",
			httpClient:    mocks.ClientWithError(errors.New("client error")),
			validationURL: apiGatewayURL,
			policiesURL:   apiGatewayURL,
			apiKey:        apiKey,
			expectedErr:   resources.NewKeyErr("Request to get API key policies failed", http.StatusInternalServerError, apiKeySuffix),
		},
		{
			name:          "Not Found",
			httpClient:    mocks.ClientWithResponseCode(http.StatusNotFound),
			validationURL: apiGatewayURL,
			policiesURL:   apiGatewayURL,
			apiKey:        apiKey,
			expectedErr:   resources.NewKeyErr("API key policies not found", http.StatusNotFound, apiKeySuffix),
		},
		{
			name:          "Generic Error",
			httpClient:    mocks.ClientWithResponseBody(http.StatusTeapot, "server error"),
			validationURL: apiGatewayURL,
			policiesURL:   apiGatewayURL,
			apiKey:        apiKey,
			expectedErr:   resources.NewKeyErr("Request to get API key policies returned an unexpected response", http.StatusTeapot, apiKeySuffix),
		},
		{
			name:          "Decoding Error",
			httpClient:    mocks.ClientWithResponseBody(http.StatusOK, `{'x-policy':'1, 2, 3}`),
			validationURL: apiGatewayURL,
			policiesURL:   apiGatewayURL,
			apiKey:        apiKey,
			expectedErr:   resources.NewKeyErr("Decoding API Key policies failed", http.StatusInternalServerError, apiKeySuffix),
		},
		{
			name:             "Empty Policies",
			httpClient:       mocks.ClientWithResponseBody(http.StatusOK, `{'x-policy':''}`),
			validationURL:    apiGatewayURL,
			policiesURL:      apiGatewayURL,
			apiKey:           apiKey,
			expectedPolicies: []string{},
		},
		{
			name:             "Non-empty Policies",
			httpClient:       mocks.ClientWithResponseBody(http.StatusOK, `{'x-policy':'1, 2, 3'}`),
			validationURL:    apiGatewayURL,
			policiesURL:      apiGatewayURL,
			apiKey:           apiKey,
			expectedPolicies: []string{"1", "2", "3"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			log := logger.NewUPPLogger("TEST", "PANIC")
			processor := resources.NewKeyProcessor(test.validationURL, test.policiesURL, test.httpClient, log)

			policies, err := processor.GetPolicies(context.Background(), test.apiKey)

			if test.expectedErr != nil {
				require.Error(t, err)
				assert.Nil(t, policies)

				keyErr := &resources.KeyErr{}
				require.True(t, errors.As(err, &keyErr))

				// Strip optional description used for logging.
				keyErr.Description = ""

				assert.Equal(t, test.expectedErr, keyErr)
				return
			}

			require.Nil(t, err)
			assert.Equal(t, test.expectedPolicies, policies)
		})
	}
}
