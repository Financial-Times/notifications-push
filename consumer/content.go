package consumer

import (
	"regexp"
	"strings"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/kafka-client-go/v3"
)

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

type ContentQueueHandler struct {
	contentURIWhitelist  *regexp.Regexp
	contentTypeWhitelist *Set
	e2eTestUUIDs         []string
	mapper               NotificationMapper
	dispatcher           notificationDispatcher
	log                  *logger.UPPLogger
}

// NewContentQueueHandler returns a new message handler
func NewContentQueueHandler(contentURIWhitelist *regexp.Regexp, contentTypeWhitelist *Set, e2eTestUUIDs []string, mapper NotificationMapper, dispatcher notificationDispatcher, log *logger.UPPLogger) *ContentQueueHandler {
	return &ContentQueueHandler{
		contentURIWhitelist:  contentURIWhitelist,
		contentTypeWhitelist: contentTypeWhitelist,
		e2eTestUUIDs:         e2eTestUUIDs,
		mapper:               mapper,
		dispatcher:           dispatcher,
		log:                  log,
	}
}

func (qHandler *ContentQueueHandler) HandleMessage(queueMsg kafka.FTMessage) {
	msg := NotificationQueueMessage{queueMsg}
	tid := msg.TransactionID()
	pubEvent, err := msg.AsContent()
	contentType := msg.Headers["Content-Type"]

	monitoringLogger := qHandler.log.WithMonitoringEvent("NotificationsPush", tid, contentType)
	if err != nil {
		monitoringLogger.WithField("message_body", msg.Body).WithError(err).Warn("Skipping event.")
		return
	}

	if msg.HasCarouselTransactionID() {
		monitoringLogger.WithValidFlag(false).WithField("contentUri", pubEvent.ContentURI).Info("Skipping event: Carousel publish event.")
		return
	}

	strippedDirectivesContentType := stripDirectives(contentType)
	isE2ETest := msg.HasE2ETestTransactionID(qHandler.e2eTestUUIDs)
	if !isE2ETest {
		if msg.HasSynthTransactionID() {
			monitoringLogger.WithValidFlag(false).WithField("contentUri", pubEvent.ContentURI).Info("Skipping event: Synthetic transaction ID.")
			return
		}

		if strippedDirectivesContentType == "application/json" || strippedDirectivesContentType == "" {
			if !pubEvent.Matches(qHandler.contentURIWhitelist) {
				monitoringLogger.WithValidFlag(false).WithField("contentUri", pubEvent.ContentURI).Info("Skipping event: contentUri is not in the whitelist.")
				return
			}
		} else {
			if !qHandler.contentTypeWhitelist.Contains(strippedDirectivesContentType) {
				monitoringLogger.WithValidFlag(false).Info("Skipping event: contentType is not the whitelist.")
				return
			}
		}
	}

	pubEvent.ContentTypeHeader = strippedDirectivesContentType
	notification, err := qHandler.mapper.MapNotification(pubEvent, msg.TransactionID())
	if err != nil {
		monitoringLogger.WithError(err).Warn("Skipping event: Cannot build notification for message.")
		return
	}
	notification.IsE2ETest = isE2ETest

	monitoringLogger.
		WithField("resource", notification.APIURL).
		WithField("notification_type", notification.Type).
		Info("Valid notification received")
	qHandler.dispatcher.Send(notification)
}

func stripDirectives(contentType string) string {
	return strings.Split(contentType, ";")[0]
}
