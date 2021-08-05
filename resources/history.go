package resources

import (
	"errors"
	"net/http"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/notifications-push/v5/dispatch"
)

type HistoryHandler struct {
	history      dispatch.History
	keyProcessor KeyProcessor
	log          *logger.UPPLogger
}

func NewHistoryHandler(history dispatch.History, keyProcessor KeyProcessor, log *logger.UPPLogger) *HistoryHandler {
	return &HistoryHandler{
		history:      history,
		keyProcessor: keyProcessor,
		log:          log,
	}
}

// History returns history data.
func (h *HistoryHandler) History(w http.ResponseWriter, r *http.Request) {
	var subscriptionOptions []dispatch.SubscriptionOption

	// API calls without an API key are supported however if such is present we want
	// to deduce whether advanced notifications are requested based on its policies.
	apiKey := getAPIKey(r)
	if apiKey != "" {
		if err := h.keyProcessor.Validate(r.Context(), apiKey); err != nil {
			logEntry := h.log.WithError(err)

			keyErr := &KeyErr{}
			if errors.As(err, &keyErr) {
				if keyErr.KeySuffix != "" {
					logEntry = logEntry.WithField("apiKeyLastChars", keyErr.KeySuffix)
				}
				if keyErr.Description != "" {
					logEntry = logEntry.WithField("description", keyErr.Description)
				}

				http.Error(w, keyErr.Msg, keyErr.Status)
			} else {
				http.Error(w, "Validating API key failed", http.StatusInternalServerError)
			}

			logEntry.Warn("Validating API key failed")
			return
		}

		policies, err := h.keyProcessor.GetPolicies(r.Context(), apiKey)
		if err != nil {
			logEntry := h.log.WithError(err)

			keyErr := &KeyErr{}
			if errors.As(err, &keyErr) {
				if keyErr.KeySuffix != "" {
					logEntry = logEntry.WithField("apiKeyLastChars", keyErr.KeySuffix)
				}
				if keyErr.Description != "" {
					logEntry = logEntry.WithField("description", keyErr.Description)
				}

				http.Error(w, keyErr.Msg, keyErr.Status)
			} else {
				http.Error(w, "Extracting API key policies failed", http.StatusInternalServerError)
			}

			logEntry.Warn("Extracting API key x-policies failed")
			return
		}

		for _, p := range policies {
			if p == advancedNotificationsXPolicy {
				subscriptionOptions = append(subscriptionOptions, dispatch.CreateEventOption)
				break
			}
		}
	}

	var notifications []dispatch.NotificationResponse
	for _, n := range h.history.Notifications() {
		notifications = append(notifications, dispatch.CreateNotificationResponse(n, subscriptionOptions))
	}

	errMsg := "Serving /__history request"

	w.Header().Set("Content-type", "application/json; charset=UTF-8")

	historyJSON, err := dispatch.MarshalNotificationResponsesJSON(notifications)
	if err != nil {
		h.log.WithError(err).Warn(errMsg)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	_, err = w.Write(historyJSON)
	if err != nil {
		h.log.WithError(err).Warn(errMsg)
		http.Error(w, "", http.StatusInternalServerError)
	}
}
