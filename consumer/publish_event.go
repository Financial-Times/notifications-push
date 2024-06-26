package consumer

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/Financial-Times/kafka-client-go/v4"
	"github.com/Financial-Times/notifications-push/v5/publication"
)

// NotificationQueueMessage is a wrapper for the queue consumer message type
type NotificationQueueMessage struct {
	kafka.FTMessage
}

// HasSynthTransactionID checks if the message is synthetic
func (msg NotificationQueueMessage) HasSynthTransactionID() bool {
	tid := msg.TransactionID()
	return strings.HasPrefix(tid, "SYNTH")
}

// HasE2ETestTransactionID checks if the message is e2e test
func (msg NotificationQueueMessage) HasE2ETestTransactionID(e2eTestUUIDs []string) bool {
	tid := msg.TransactionID()
	for _, testUUID := range e2eTestUUIDs {
		if strings.Contains(tid, testUUID) {
			return true
		}
	}

	return false
}

// HasCarouselTransactionID checks if the message is generated by the publish carousel
func (msg NotificationQueueMessage) HasCarouselTransactionID() bool {
	return carouselTransactionIDRegExp.MatchString(msg.TransactionID())
}

var carouselTransactionIDRegExp = regexp.MustCompile(`^.+_carousel_[\d]{10}.*$`) //`^(tid_[\S]+)_carousel_[\d]{10}.*$`

// TransactionID returns the message TID
func (msg NotificationQueueMessage) TransactionID() string {
	return msg.Headers["X-Request-Id"]
}

func (msg NotificationQueueMessage) Unmarshal() (NotificationMessage, error) {
	var event NotificationMessage
	err := json.Unmarshal([]byte(msg.Body), &event)
	if err != nil {
		return NotificationMessage{}, err
	}

	event.ContentType = msg.Headers["Content-Type"]
	event.MessageType = msg.Headers["Message-Type"]

	return event, nil
}

type NotificationMessage struct {
	ContentURI   string
	ContentType  string
	LastModified string
	MessageType  string
	Payload      Payload `json:"Payload,omitempty"`
}

// Matches is a method that returns True if the ContentURI of a publication event
// matches a allowlist regexp
func (e NotificationMessage) Matches(allowlist *regexp.Regexp) bool {
	return allowlist.MatchString(e.ContentURI)
}

// Item struct represents the payload in NotificationMessage
// We use omitempty to indicate nonmandatory fields
type Payload struct {
	ContentType                  string                    `json:"type"`
	Title                        string                    `json:"title"`
	Publication                  *publication.Publications `json:"publication,omitempty"`
	EditorialDesk                string                    `json:"editorialDesk,omitempty"`
	PublishCount                 int                       `json:"publishCount,omitempty"`
	Deleted                      bool                      `json:"deleted,omitempty"`
	IsRelatedContentNotification bool                      `json:"is_related_content_notification,omitempty"`
	Standout                     *Standout                 `json:"standout,omitempty"`
	ContentID                    string                    `json:"ContentID"`
}

// Standout
type Standout struct {
	EditorsChoice bool `json:"editorsChoice"`
	Exclusive     bool `json:"exclusive"`
	Scoop         bool `json:"scoop"`
}
