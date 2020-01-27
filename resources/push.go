package resources

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	logger "github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/notifications-push/v4/dispatch"
)

const (
	HeartbeatMsg            = "[]"
	apiKeyHeaderField       = "X-Api-Key"
	apiKeyQueryParam        = "apiKey"
	ClientAdrKey            = "X-Forwarded-For"
	defaultSubscriptionType = "Article"
)

var supportedContentTypes = []string{"Article", "ContentPackage", "Audio", "All"}

type keyValidator interface {
	Validate(ctx context.Context, key string) error
}

// SubHandler manages subscription requests
type SubHandler struct {
	dispatcher      dispatch.Registrar
	validator       keyValidator
	heartbeatPeriod time.Duration
	log             *logger.UPPLogger
}

func NewSubHandler(dispatcher dispatch.Registrar,
	validator keyValidator,
	heartbeatPeriod time.Duration,
	log *logger.UPPLogger) *SubHandler {
	return &SubHandler{
		dispatcher:      dispatcher,
		validator:       validator,
		heartbeatPeriod: heartbeatPeriod,
		log:             log,
	}
}

func (h *SubHandler) HandleSubscription(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-type", "text/event-stream; charset=UTF-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	apiKey := getAPIKey(r)
	err := h.validator.Validate(r.Context(), apiKey)
	if err != nil {
		keyErr := &KeyErr{}
		if !errors.As(err, &keyErr) {
			http.Error(w, "Cannot stream.", http.StatusInternalServerError)
			return
		}
		http.Error(w, keyErr.Msg, keyErr.Status)
		return
	}

	subscriptionParam, err := resolveSubType(r)
	if err != nil {
		h.log.WithError(err).Error("Invalid content type")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	monitorParam := r.URL.Query().Get("monitor")
	isMonitor, _ := strconv.ParseBool(monitorParam)

	s := h.dispatcher.Subscribe(getClientAddr(r), subscriptionParam, isMonitor)
	defer h.dispatcher.Unsubscribe(s)

	h.listenForNotifications(r.Context(), s, w)
}

// listenForNotifications starts listening on the subscribes channel for notifications
func (h *SubHandler) listenForNotifications(ctx context.Context, s dispatch.Subscriber, w http.ResponseWriter) {
	bw := bufio.NewWriter(w)
	timer := time.NewTimer(h.heartbeatPeriod)
	logEntry := h.log.WithField("subscriberId", s.Id()).WithField("subscriber", s.Address())

	write := func(notification string) error {
		_, err := bw.WriteString("data: " + notification + "\n\n")
		if err != nil {
			return err
		}

		err = bw.Flush()
		if err != nil {
			return err
		}

		flusher := w.(http.Flusher)
		flusher.Flush()
		return nil
	}
	//first thing we write is a heartbeat
	err := write(HeartbeatMsg)
	if err != nil {
		logEntry.WithError(err).Error("Sending heartbeat to subscriber has failed ")
		return
	}

	logEntry.Info("Heartbeat sent to subscriber successfully")
	for {
		select {
		case notification := <-s.NotificationChannel():
			err := write(notification)
			if err != nil {
				logEntry.WithError(err).Error("Error while sending notification to subscriber")
				return
			}
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(h.heartbeatPeriod)
		case <-timer.C:
			err := write(HeartbeatMsg)
			if err != nil {
				logEntry.WithError(err).Error("Sending heartbeat to subscriber has failed ")
				return
			}

			timer.Reset(h.heartbeatPeriod)

			logEntry.Info("Heartbeat sent to subscriber successfully")
		case <-ctx.Done():
			logEntry.Info("Notification subscriber disconnected remotely")
			return
		}
	}
}

func getClientAddr(r *http.Request) string {
	xForwardedFor := r.Header.Get(ClientAdrKey)
	if xForwardedFor != "" {
		addr := strings.Split(xForwardedFor, ",")
		return addr[0]
	}
	return r.RemoteAddr
}

func resolveSubType(r *http.Request) (string, error) {
	contentType := r.URL.Query().Get("type")
	if contentType == "" {
		return defaultSubscriptionType, nil
	}
	for _, t := range supportedContentTypes {
		if strings.ToLower(contentType) == strings.ToLower(t) {
			return contentType, nil
		}
	}
	return "", fmt.Errorf("The specified type (%s) is unsupported", contentType)
}

//ApiKey is provided either as a request param or as a header.
func getAPIKey(r *http.Request) string {
	apiKey := r.Header.Get(apiKeyHeaderField)
	if apiKey != "" {
		return apiKey
	}

	return r.URL.Query().Get(apiKeyQueryParam)
}
