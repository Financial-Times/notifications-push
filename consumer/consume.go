package consumer

import (
	"regexp"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/kafka-client-go/v4"
	"github.com/Financial-Times/notifications-push/v5/dispatch"
)

// MessageQueueHandler is a generic interface for implementation of components to handle messages form the kafka queue.
type MessageQueueHandler interface {
	HandleMessage(queueMsg kafka.FTMessage)
}

type notificationDispatcher interface {
	Send(notification dispatch.NotificationModel)
}

var exists = struct{}{}

type Set struct {
	m map[string]struct{}
}

func NewSet() *Set {
	s := &Set{}
	s.m = make(map[string]struct{})
	return s
}

func (s *Set) Add(value string) {
	s.m[value] = exists
}

func (s *Set) Contains(value string) bool {
	_, c := s.m[value]
	return c
}

type QueueHandler struct {
	contentURIAllowlist  *regexp.Regexp
	contentTypeAllowlist *Set
	e2eTestUUIDs         []string
	mapper               NotificationMapper
	dispatcher           notificationDispatcher
	monitorsEvents       bool
	log                  *logger.UPPLogger
}

func NewQueueHandler(contentURIAllowlist *regexp.Regexp, contentTypeAllowlist *Set, e2eTestUUIDs []string, monitorsEvents bool, mapper NotificationMapper, dispatcher notificationDispatcher, log *logger.UPPLogger) *QueueHandler {
	return &QueueHandler{
		contentURIAllowlist:  contentURIAllowlist,
		contentTypeAllowlist: contentTypeAllowlist,
		e2eTestUUIDs:         e2eTestUUIDs,
		mapper:               mapper,
		dispatcher:           dispatcher,
		monitorsEvents:       monitorsEvents,
		log:                  log,
	}
}

func (h *QueueHandler) HandleMessage(queueMsg kafka.FTMessage) {
	msg := NotificationQueueMessage{queueMsg}
	tid := msg.TransactionID()

	pubEvent, err := msg.Unmarshal()

	var logEntry *logger.LogEntry
	if h.monitorsEvents {
		logEntry = h.log.WithMonitoringEvent("NotificationsPush", tid, pubEvent.ContentType)
	} else {
		logEntry = h.log.WithTransactionID(tid)
	}

	if err != nil {
		logEntry.WithError(err).Error("Failed to unmarshall kafka message")
		return
	}

	if msg.HasCarouselTransactionID() {
		logEntry.WithValidFlag(false).WithField("contentUri", pubEvent.ContentURI).Info("Skipping event: Carousel publish event.")
		return
	}

	isE2ETest := msg.HasE2ETestTransactionID(h.e2eTestUUIDs)
	if !isE2ETest {
		if msg.HasSynthTransactionID() {
			logEntry.WithValidFlag(false).WithField("contentUri", pubEvent.ContentURI).Info("Skipping event: Synthetic transaction ID.")
			return
		}

		if pubEvent.ContentType == "application/json" {
			if !pubEvent.Matches(h.contentURIAllowlist) {
				logEntry.WithValidFlag(false).WithField("contentUri", pubEvent.ContentURI).Info("Skipping event: contentUri is not in the allowlist.")
				return
			}
		} else {
			if !h.contentTypeAllowlist.Contains(pubEvent.ContentType) {
				logEntry.WithValidFlag(false).Info("Skipping event: contentType is not in the allowlist.")
				return
			}
		}
	}

	notification, err := h.mapper.MapNotification(pubEvent, msg.TransactionID())
	if err != nil {
		logEntry.WithError(err).Warn("Skipping event: Cannot build notification for message.")
		return
	}
	notification.IsE2ETest = isE2ETest

	logEntry.
		WithField("resource", notification.APIURL).
		WithField("notification_type", notification.Type).
		Info("Valid notification received")

	if !isE2ETest && notification.SubscriptionType == dispatch.ArticleContentType {
		h.log.WithField("eventType", notification.Type).WithField("ID", notification.ID).WithTransactionID(tid).Info("Processed article notification")
	}
	h.dispatcher.Send(notification)
}
