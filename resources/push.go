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
	"github.com/Financial-Times/notifications-push/v5/dispatch"
)

const (
	HeartbeatMsg            = "[]"
	apiKeyHeaderField       = "X-Api-Key"
	apiKeyQueryParam        = "apiKey"
	ClientAdrKey            = "X-Forwarded-For"
	defaultSubscriptionType = dispatch.ArticleContentType
)

var supportedSubscriptionTypes = map[string]bool{
	strings.ToLower(dispatch.AnnotationsType):    true,
	strings.ToLower(dispatch.ArticleContentType): true,
	strings.ToLower(dispatch.ContentPackageType): true,
	strings.ToLower(dispatch.AudioContentType):   true,
	strings.ToLower(dispatch.AllContentType):     true,
	strings.ToLower(dispatch.LiveBlogPackage):    true,
	strings.ToLower(dispatch.LiveBlogPost):       true,
}

type keyValidator interface {
	Validate(ctx context.Context, key string) error
}

type notifier interface {
	Subscribe(address string, subTypes []string, monitoring bool) dispatch.Subscriber
	Unsubscribe(subscriber dispatch.Subscriber)
}

type onShutdown interface {
	RegisterOnShutdown(f func())
}

// SubHandler manages subscription requests
type SubHandler struct {
	notif           notifier
	validator       keyValidator
	shutdown        onShutdown
	heartbeatPeriod time.Duration
	log             *logger.UPPLogger
}

func NewSubHandler(n notifier,
	validator keyValidator,
	shutdown onShutdown,
	heartbeatPeriod time.Duration,
	log *logger.UPPLogger) *SubHandler {
	return &SubHandler{
		notif:           n,
		validator:       validator,
		shutdown:        shutdown,
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

	subscriptionParams, err := resolveSubType(r)
	if err != nil {
		h.log.WithError(err).Error("Invalid content type")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	monitorParam := r.URL.Query().Get("monitor")
	isMonitor, _ := strconv.ParseBool(monitorParam)

	s := h.notif.Subscribe(getClientAddr(r), subscriptionParams, isMonitor)
	defer h.notif.Unsubscribe(s)

	ctx, cancel := context.WithCancel(r.Context())
	h.shutdown.RegisterOnShutdown(cancel)
	h.listenForNotifications(ctx, s, w)
}

// listenForNotifications starts listening on the subscribes channel for notifications
func (h *SubHandler) listenForNotifications(ctx context.Context, s dispatch.Subscriber, w http.ResponseWriter) {
	bw := bufio.NewWriter(w)
	timer := time.NewTimer(h.heartbeatPeriod)
	logEntry := h.log.WithField("subscriberId", s.ID()).WithField("subscriber", s.Address())

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
		case notification := <-s.Notifications():
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

func resolveSubType(r *http.Request) ([]string, error) {
	retVal := make([]string, 0)
	values := r.URL.Query()
	subTypes := values["type"]
	if len(subTypes) == 0 {
		return []string{defaultSubscriptionType}, nil
	}
	// subTypes are being send by the client (subscriber), and needs to be matched with such string value
	var subType string
	for _, subType = range subTypes {
		if supportedSubscriptionTypes[strings.ToLower(subType)] {
			if subType == dispatch.AllContentType {
				retVal = append(retVal, dispatch.AllAllowedList...)
				continue
			}
			retVal = append(retVal, subType)
		}
	}

	if len(retVal) == 0 {
		return nil, fmt.Errorf("specified type (%s) is unsupported", subType)
	}

	return retVal, nil
}

//ApiKey is provided either as a request param or as a header.
func getAPIKey(r *http.Request) string {
	apiKey := r.Header.Get(apiKeyHeaderField)
	if apiKey != "" {
		return apiKey
	}

	return r.URL.Query().Get(apiKeyQueryParam)
}
