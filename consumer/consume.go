package consumer

import (
	kafkav1 "github.com/Financial-Times/kafka-client-go/kafka"
	kafkav2 "github.com/Financial-Times/kafka-client-go/v2"
	"github.com/Financial-Times/notifications-push/v5/dispatch"
)

type ContentQueueProcessor interface {
	HandleMessage(queueMsg kafkav2.FTMessage)
}

type MetadataQueueProcessor interface {
	HandleMessage(queueMsg kafkav1.FTMessage) error
}

type notificationDispatcher interface {
	Send(notification dispatch.NotificationModel)
}

type MessageQueueRouter struct {
	contentHandler  ContentQueueProcessor
	metadataHandler MetadataQueueProcessor
}

func NewMessageQueueHandler(contentHandler ContentQueueProcessor, metadataHandler MetadataQueueProcessor) *MessageQueueRouter {
	return &MessageQueueRouter{
		contentHandler:  contentHandler,
		metadataHandler: metadataHandler,
	}
}

func (h *MessageQueueRouter) HandleContentMessage(queueMsg kafkav2.FTMessage) {
	h.contentHandler.HandleMessage(queueMsg)
}

func (h *MessageQueueRouter) HandleMetadataMessage(queueMsg kafkav1.FTMessage) error {
	if h.metadataHandler != nil && isAnnotationMessage(queueMsg.Headers) {
		return h.metadataHandler.HandleMessage(queueMsg)
	}
	return nil
}

func isAnnotationMessage(msgHeaders map[string]string) bool {
	msgType, ok := msgHeaders["Message-Type"]
	if !ok {
		return false
	}
	return msgType == "concept-annotation"
}
