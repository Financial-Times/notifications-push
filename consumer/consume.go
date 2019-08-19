package consumer

import (
	"regexp"
	"strings"

	log "github.com/Financial-Times/go-logger"
	"github.com/Financial-Times/kafka-client-go/kafka"
	"github.com/Financial-Times/notifications-push/dispatch"
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

// MessageQueueHandler is a generic interface for implementation of components to hendle messages form the kafka queue.
type MessageQueueHandler interface {
	HandleMessage(queueMsg kafka.FTMessage) error
}

type simpleMessageQueueHandler struct {
	contentUriWhitelist  *regexp.Regexp
	contentTypeWhitelist *Set
	mapper               NotificationMapper
	dispatcher           dispatch.Dispatcher
}

// NewMessageQueueHandler returns a new message handler
func NewMessageQueueHandler(contentUriWhitelist *regexp.Regexp, contentTypeWhitelist *Set, mapper NotificationMapper, dispatcher dispatch.Dispatcher) MessageQueueHandler {
	return &simpleMessageQueueHandler{
		contentUriWhitelist:  contentUriWhitelist,
		contentTypeWhitelist: contentTypeWhitelist,
		mapper:               mapper,
		dispatcher:           dispatcher,
	}
}

func (qHandler *simpleMessageQueueHandler) HandleMessage(queueMsg kafka.FTMessage) error {
	msg := NotificationQueueMessage{queueMsg}
	tid := msg.TransactionID()
	pubEvent, err := msg.ToPublicationEvent()
	contentType := msg.Headers["Content-Type"]

	monitoringLogger := log.WithMonitoringEvent("NotificationsPush", tid, contentType)
	if err != nil {
		monitoringLogger.WithField("message_body", msg.Body).WithError(err).Warn("Skipping event.")
		return err
	}

	if msg.HasCarouselTransactionID() {
		monitoringLogger.WithValidFlag(false).WithField("contentUri", pubEvent.ContentURI).Info("Skipping event: Carousel publish event.")
		return nil
	}

	if msg.HasSynthTransactionID() {
		monitoringLogger.WithValidFlag(false).WithField("contentUri", pubEvent.ContentURI).Info("Skipping event: Synthetic transaction ID.")
		return nil
	}

	strippedDirectivesContentType := stripDirectives(contentType)
	if strippedDirectivesContentType == "application/json" || strippedDirectivesContentType == "" {
		if !pubEvent.Matches(qHandler.contentUriWhitelist) {
			monitoringLogger.WithValidFlag(false).WithField("contentUri", pubEvent.ContentURI).Info("Skipping event: contentUri is not in the whitelist.")
			return nil
		}
	} else {
		if !qHandler.contentTypeWhitelist.Contains(strippedDirectivesContentType) {
			monitoringLogger.WithValidFlag(false).Info("Skipping event: contentType is not the whitelist.")
			return nil
		}
	}

	pubEvent.ContentTypeHeader = strippedDirectivesContentType
	notification, err := qHandler.mapper.MapNotification(pubEvent, msg.TransactionID())
	if err != nil {
		monitoringLogger.WithError(err).Warn("Skipping event: Cannot build notification for message.")
		return err
	}

	log.WithField("resource", notification.APIURL).WithField("transaction_id", notification.PublishReference).Info("Valid notification received")
	qHandler.dispatcher.Send(notification)

	return nil
}

func stripDirectives(contentType string) string {
	return strings.Split(contentType, ";")[0]
}
