package consumer

import (
	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/kafka-client-go/v3"
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

func (h *MetadataQueueHandler) HandleMessage(queueMsg kafka.FTMessage) {
	msg := NotificationQueueMessage{queueMsg}
	tid := msg.TransactionID()
	entry := h.log.WithTransactionID(tid)

	event, err := msg.AsMetadata()
	if err != nil {
		entry.WithField("message_body", msg.Body).WithError(err).Warn("Skipping annotation event.")
		return
	}

	if msg.HasSynthTransactionID() {
		entry.WithValidFlag(false).WithField("contentURI", event.ContentURI).Info("Skipping annotation event: Synthetic transaction ID.")
		return
	}

	if !h.IsAllowedOrigin(msg.OriginID()) {
		entry.WithValidFlag(false).Info("Skipping annotation event: origin system is not the whitelist.")
		return
	}

	notification, err := h.mapper.MapMetadataNotification(event, msg.TransactionID())
	if err != nil {
		entry.WithField("message_body", msg.Body).WithError(err).Warn("Could not map event to Annotations message")
		return
	}
	entry.WithField("resource", notification.APIURL).Info("Valid annotation notification received")
	h.dispatcher.Send(notification)
}

func (h *MetadataQueueHandler) IsAllowedOrigin(origin string) bool {
	for _, o := range h.originWhitelist {
		if o == origin {
			return true
		}
	}
	return false
}
