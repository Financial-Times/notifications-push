package consumer

import (
	"github.com/Financial-Times/go-logger/v2"
	kafkav1 "github.com/Financial-Times/kafka-client-go/kafka"
)

type MetadataQueueHandler struct {
	originWhitelist []string
	mapper          NotificationMapper
	dispatcher      notificationDispatcher
	log             *logger.UPPLogger
}

func NewMetadataQueueHandler(originWhitelist []string, mapper NotificationMapper, dispatcher notificationDispatcher, log *logger.UPPLogger) *MetadataQueueHandler {
	return &MetadataQueueHandler{
		originWhitelist: originWhitelist,
		mapper:          mapper,
		dispatcher:      dispatcher,
		log:             log,
	}
}

type MetadataServiceMessage struct {
	msg kafkav1.FTMessage
}

func (sm *MetadataServiceMessage) GetBody() string {
	return sm.msg.Body
}

func (sm *MetadataServiceMessage) GetHeaders() map[string]string {
	return sm.msg.Headers
}

func (h *MetadataQueueHandler) HandleMessage(queueMsg kafkav1.FTMessage) error {
	msg := NotificationQueueMessage{&MetadataServiceMessage{queueMsg}}
	tid := msg.TransactionID()
	entry := h.log.WithTransactionID(tid)
	entry.Info("Handling metadata message..")

	event, err := msg.AsMetadata()
	if err != nil {
		entry.WithField("message_body", msg.GetBody()).WithError(err).Warn("Skipping annotation event.")
		return err
	}

	if msg.HasSynthTransactionID() {
		entry.WithValidFlag(false).WithField("contentURI", event.ContentURI).Info("Skipping annotation event: Synthetic transaction ID.")
		return nil
	}

	if !h.IsAllowedOrigin(msg.OriginID()) {
		entry.WithValidFlag(false).Info("Skipping annotation event: origin system is not the whitelist.")
		return nil
	}

	notification, err := h.mapper.MapMetadataNotification(event, msg.TransactionID())
	if err != nil {
		entry.WithField("message_body", msg.GetBody()).WithError(err).Warn("Could not map event to Annotations message")
		return err
	}
	entry.WithField("resource", notification.APIURL).Info("Valid annotation notification received")
	h.dispatcher.Send(notification)

	return nil
}

func (h *MetadataQueueHandler) IsAllowedOrigin(origin string) bool {
	for _, o := range h.originWhitelist {
		if o == origin {
			return true
		}
	}
	return false
}
