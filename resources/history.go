package resources

import (
	"net/http"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/notifications-push/v5/dispatch"
)

// History returns history data
func History(history dispatch.History, log *logger.UPPLogger) func(w http.ResponseWriter, r *http.Request) {
	errMsg := "Serving /__history request"
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "application/json; charset=UTF-8")

		var notifications []dispatch.NotificationResponse
		for _, n := range history.Notifications() {
			notifications = append(notifications, dispatch.CreateNotificationResponse(n, nil))
		}

		historyJSON, err := dispatch.MarshalNotificationResponsesJSON(notifications)
		if err != nil {
			log.WithError(err).Warn(errMsg)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		_, err = w.Write(historyJSON)
		if err != nil {
			log.WithError(err).Warn(errMsg)
			http.Error(w, "", http.StatusInternalServerError)
		}
	}
}
