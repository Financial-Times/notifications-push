package consumer

import (
	"github.com/Financial-Times/kafka-client-go/kafka"
	"github.com/Financial-Times/notifications-push/v5/dispatch"
)

// MessageQueueHandler is a generic interface for implementation of components to handle messages form the kafka queue.
type MessageQueueHandler interface {
	HandleMessage(queueMsg kafka.FTMessage) error
}

type notificationDispatcher interface {
	Send(notification dispatch.NotificationModel)
}

type MessageQueueRouter struct {
	contentHandler  MessageQueueHandler
	metadataHandler MessageQueueHandler
}

func NewMessageQueueHandler(contentHandler, metadataHandler MessageQueueHandler) *MessageQueueRouter {
	return &MessageQueueRouter{
		contentHandler:  contentHandler,
		metadataHandler: metadataHandler,
	}
}

func (h *MessageQueueRouter) HandleMessage(queueMsg kafka.FTMessage) error {
	if h.metadataHandler != nil && isAnnotationMessage(queueMsg.Headers) {
		return h.metadataHandler.HandleMessage(queueMsg)
	}
	return h.contentHandler.HandleMessage(queueMsg)
}

func isAnnotationMessage(msgHeaders map[string]string) bool {
	msgType, ok := msgHeaders["Message-Type"]
	if !ok {
		return false
	}
	return msgType == "concept-annotation"
}
