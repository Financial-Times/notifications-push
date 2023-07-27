package access

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Financial-Times/notifications-push/v5/dispatch"
	"github.com/Financial-Times/notifications-push/v5/mocks"
)

func TestPolicy_GetNotificationSubscriptionOptions(t *testing.T) {
	t.Parallel()

	const (
		apiKey       = "long_test_key_x"
		apiKeySuffix = "test_key_x"
	)

	tests := []struct {
		name         string
		httpClient   *http.Client
		policiesURL  string
		apiKey       string
		expectedErr  *PolicyErr
		expectedOpts *dispatch.NotificationSubscriptionOptions
	}{
		{
			name:        "Empty API Key",
			httpClient:  mocks.ClientWithResponseCode(http.StatusOK),
			policiesURL: apiGatewayRawURL,
			apiKey:      "",
			expectedErr: NewPolicyErr("Empty api key used to get X-Policies", http.StatusUnauthorized, "", ""),
		},
		{
			name:        "HTTP Client Error",
			httpClient:  mocks.ClientWithError(fmt.Errorf("client error")),
			policiesURL: apiGatewayRawURL,
			apiKey:      apiKey,
			expectedErr: NewPolicyErr("Request to get X-Policies assigned to API key failed", http.StatusInternalServerError, apiKeySuffix, ""),
		},
		{
			name:        "Not Found",
			httpClient:  mocks.ClientWithResponseCode(http.StatusNotFound),
			policiesURL: apiGatewayRawURL,
			apiKey:      apiKey,
			expectedErr: NewPolicyErr("X-Policies assigned to API key not found", http.StatusNotFound, apiKeySuffix, ""),
		},
		{
			name:        "Generic Error",
			httpClient:  mocks.ClientWithResponseBody(http.StatusTeapot, "server error"),
			policiesURL: apiGatewayRawURL,
			apiKey:      apiKey,
			expectedErr: NewPolicyErr("Request to get X-Policies assigned to API key returned an unexpected response", http.StatusTeapot, apiKeySuffix, ""),
		},
		{
			name:        "Decoding Error",
			httpClient:  mocks.ClientWithResponseBody(http.StatusOK, `{'x-policy':'1, 2, 3}`),
			policiesURL: apiGatewayRawURL,
			apiKey:      apiKey,
			expectedErr: NewPolicyErr("Decoding X-Policies assigned to API key failed", http.StatusInternalServerError, apiKeySuffix, ""),
		},
		{
			name:        "No X-Policies sent",
			httpClient:  mocks.ClientWithResponseBody(http.StatusOK, `{'x-policy':''}`),
			policiesURL: apiGatewayRawURL,
			apiKey:      apiKey,
			expectedOpts: &dispatch.NotificationSubscriptionOptions{
				ReceiveAdvancedNotifications: false,
			},
		},
		{
			name:        "X-Policy for advanced notifications sent",
			httpClient:  mocks.ClientWithResponseBody(http.StatusOK, `{'x-policy':'ADVANCED_NOTIFICATIONS'}`),
			policiesURL: apiGatewayRawURL,
			apiKey:      apiKey,
			expectedOpts: &dispatch.NotificationSubscriptionOptions{
				ReceiveAdvancedNotifications: true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			apiGatewayURL := urlFromRawString(apiGatewayRawURL)

			processor := NewPolicyProcessor(apiGatewayURL, test.httpClient)

			opts, err := processor.GetNotificationSubscriptionOptions(context.Background(), test.apiKey)

			if test.expectedErr != nil {
				require.Error(t, err)
				assert.Nil(t, opts)

				policyErr := &PolicyErr{}
				require.True(t, errors.As(err, &policyErr))

				// Strip optional description used for logging.
				policyErr.Description = ""

				assert.Equal(t, test.expectedErr, policyErr)
				return
			}

			require.Nil(t, err)
			assert.Equal(t, test.expectedOpts, opts)
		})
	}
}
