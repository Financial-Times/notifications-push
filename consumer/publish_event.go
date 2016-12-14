package consumer

import (
	"encoding/json"
	"regexp"
	"strings"

	queueConsumer "github.com/Financial-Times/message-queue-gonsumer/consumer"
)

// NotificationQueueMessage is a wrapper for the queue consumer message type
type NotificationQueueMessage struct {
	queueConsumer.Message
}

// HasSynthTransactionID checks if the message is synthetic
func (msg NotificationQueueMessage) HasSynthTransactionID() bool {
	tid := msg.TransactionID()
	return strings.HasPrefix(tid, "SYNTH")
}

// TransactionID returns the message TID
func (msg NotificationQueueMessage) TransactionID() string {
	return msg.Headers["X-Request-Id"]
}

// ToCmsPublicationEvent converts the message to a CmsPublicationEvent
func (msg NotificationQueueMessage) ToPublicationEvent() (event PublicationEvent, err error) {
	err = json.Unmarshal([]byte(msg.Body), &event)
	return event, err
}

type PublicationEvent struct {
	ContentURI   string      `json:"contentUri"`
	Payload      interface{} `json:"payload"`
	LastModified string      `json:"lastModified"`
}

func (e PublicationEvent) Matches(whiteList *regexp.Regexp) bool {
	return whiteList.MatchString(e.ContentURI)
}

func (e PublicationEvent) HasEmptyPayload() bool {
	switch v := e.Payload.(type) {
	case nil:
		return true
	case string:
		if len(v) == 0 {
			return true
		}
	case map[string]interface{}:
		if len(v) == 0 {
			return true
		}
	}
	return false
}
