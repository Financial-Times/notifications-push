package resources

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/notifications-push/v5/dispatch"
	"github.com/Financial-Times/notifications-push/v5/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestHistoryHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                  string
		apiKey                string
		apiKeyProcessor       KeyProcessor
		notifications         []dispatch.NotificationModel
		expectedStatusCode    int
		expectedNotifications []dispatch.NotificationResponse
		expectedErr           string
	}{
		{
			name:               "Empty history",
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "Non-empty history",
			notifications: []dispatch.NotificationModel{
				{
					ID:    "8ffcc0a0-1334-4325-b4bb-d16dccf597e8",
					Type:  dispatch.ContentUpdateType,
					Title: "Update Notification",
				},
				{
					ID:    "8ffcc0a0-1334-4325-b4bb-d16dccf597e8",
					Type:  dispatch.ContentDeleteType,
					Title: "Delete Notification",
				},
			},
			expectedStatusCode: http.StatusOK,
			expectedNotifications: []dispatch.NotificationResponse{
				{
					ID:    "8ffcc0a0-1334-4325-b4bb-d16dccf597e8",
					Type:  dispatch.ContentUpdateType,
					Title: "Update Notification",
				},
				{
					ID:    "8ffcc0a0-1334-4325-b4bb-d16dccf597e8",
					Type:  dispatch.ContentDeleteType,
					Title: "Delete Notification",
				},
			},
		},
		{
			name:   "History with API key for standard notifications",
			apiKey: "api-key",
			apiKeyProcessor: func() KeyProcessor {
				processor := &mocks.KeyProcessor{}

				processor.
					On("Validate", mock.Anything, "api-key").
					Return(nil)

				processor.
					On("GetPolicies", mock.Anything, "api-key").
					Return([]string{}, nil)

				return processor
			}(),
			notifications: []dispatch.NotificationModel{
				{
					ID:    "8ffcc0a0-1334-4325-b4bb-d16dccf597e8",
					Type:  dispatch.ContentCreateType,
					Title: "Create Notification",
				},
				{
					ID:    "8ffcc0a0-1334-4325-b4bb-d16dccf597e8",
					Type:  dispatch.ContentUpdateType,
					Title: "Update Notification",
				},
				{
					ID:    "8ffcc0a0-1334-4325-b4bb-d16dccf597e8",
					Type:  dispatch.ContentDeleteType,
					Title: "Delete Notification",
				},
			},
			expectedStatusCode: http.StatusOK,
			expectedNotifications: []dispatch.NotificationResponse{
				{
					ID:    "8ffcc0a0-1334-4325-b4bb-d16dccf597e8",
					Type:  dispatch.ContentUpdateType,
					Title: "Create Notification",
				},
				{
					ID:    "8ffcc0a0-1334-4325-b4bb-d16dccf597e8",
					Type:  dispatch.ContentUpdateType,
					Title: "Update Notification",
				},
				{
					ID:    "8ffcc0a0-1334-4325-b4bb-d16dccf597e8",
					Type:  dispatch.ContentDeleteType,
					Title: "Delete Notification",
				},
			},
		},
		{
			name:   "History with API key for advanced notifications",
			apiKey: "api-key",
			apiKeyProcessor: func() KeyProcessor {
				processor := &mocks.KeyProcessor{}

				processor.
					On("Validate", mock.Anything, "api-key").
					Return(nil)

				processor.
					On("GetPolicies", mock.Anything, "api-key").
					Return([]string{advancedNotificationsXPolicy}, nil)

				return processor
			}(),
			notifications: []dispatch.NotificationModel{
				{
					ID:    "8ffcc0a0-1334-4325-b4bb-d16dccf597e8",
					Type:  dispatch.ContentCreateType,
					Title: "Create Notification",
				},
				{
					ID:    "8ffcc0a0-1334-4325-b4bb-d16dccf597e8",
					Type:  dispatch.ContentUpdateType,
					Title: "Update Notification",
				},
				{
					ID:    "8ffcc0a0-1334-4325-b4bb-d16dccf597e8",
					Type:  dispatch.ContentDeleteType,
					Title: "Delete Notification",
				},
			},
			expectedStatusCode: http.StatusOK,
			expectedNotifications: []dispatch.NotificationResponse{
				{
					ID:    "8ffcc0a0-1334-4325-b4bb-d16dccf597e8",
					Type:  dispatch.ContentCreateType,
					Title: "Create Notification",
				},
				{
					ID:    "8ffcc0a0-1334-4325-b4bb-d16dccf597e8",
					Type:  dispatch.ContentUpdateType,
					Title: "Update Notification",
				},
				{
					ID:    "8ffcc0a0-1334-4325-b4bb-d16dccf597e8",
					Type:  dispatch.ContentDeleteType,
					Title: "Delete Notification",
				},
			},
		},
		{
			name:   "Invalid API key",
			apiKey: "api-key",
			apiKeyProcessor: func() KeyProcessor {
				processor := &mocks.KeyProcessor{}
				err := &KeyErr{
					Msg:    "validation error",
					Status: http.StatusUnauthorized,
				}

				processor.
					On("Validate", mock.Anything, "api-key").
					Return(err)

				return processor
			}(),
			expectedStatusCode: http.StatusUnauthorized,
			expectedErr:        "validation error\n",
		},
		{
			name:   "Error retrieving API key policies",
			apiKey: "api-key",
			apiKeyProcessor: func() KeyProcessor {
				processor := &mocks.KeyProcessor{}
				err := &KeyErr{
					Msg:    "policies error",
					Status: http.StatusInternalServerError,
				}

				processor.
					On("Validate", mock.Anything, "api-key").
					Return(nil)

				processor.
					On("GetPolicies", mock.Anything, "api-key").
					Return(nil, err)

				return processor
			}(),
			expectedStatusCode: http.StatusInternalServerError,
			expectedErr:        "policies error\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			log := logger.NewUPPLogger("TEST", "PANIC")

			history := dispatch.NewHistory(len(test.notifications))
			for _, notification := range test.notifications {
				history.Push(notification)
			}

			historyHandler := NewHistoryHandler(history, test.apiKeyProcessor, log)

			req, err := http.NewRequest("GET", "/__history", nil)
			require.NoError(t, err)

			if test.apiKey != "" {
				req.Header.Set(apiKeyHeaderField, test.apiKey)
			}

			w := httptest.NewRecorder()
			historyHandler.History(w, req)

			assert.Equal(t, test.expectedStatusCode, w.Code)

			body, err := io.ReadAll(w.Body)
			require.NoError(t, err)

			if test.expectedErr != "" {
				assert.Equal(t, test.expectedErr, string(body))
				return
			}

			var notifications []dispatch.NotificationResponse
			require.NoError(t, json.Unmarshal(body, &notifications))

			assert.Equal(t, test.expectedNotifications, notifications)
		})
	}

}
