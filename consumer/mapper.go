package consumer

import (
	"errors"
	"regexp"

	"github.com/Financial-Times/notifications-push/dispatcher"
)

// NotificationMapper maps CmsPublicationEvents to Notifications
type NotificationMapper struct {
	APIBaseURL string
	Resource   string
}

// UUIDRegexp enables to check if a string matches a UUID
var UUIDRegexp = regexp.MustCompile("[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}")

// MapNotification maps the given event to a new notification.
func (n NotificationMapper) MapNotification(event PublicationEvent, transactionID string) (dispatcher.Notification, error) {
	UUID := UUIDRegexp.FindString(event.ContentURI)
	if UUID == "" {
		return dispatcher.Notification{}, errors.New("ContentURI does not contain a UUID")
	}

	var eventType string
	var standout *dispatcher.Standout
	var title = ""
	var contentType = ""

	if event.HasEmptyPayload() {
		eventType = "DELETE"
	} else {
		eventType = "UPDATE"
		notificationPayloadMap, ok := event.Payload.(map[string]interface{})
		if ok {
			contentType = getValueFromPayload("type", notificationPayloadMap)
			if contentType != "Content" {
				title = getValueFromPayload("title", notificationPayloadMap)
				standout = &dispatcher.Standout{Scoop: getScoopFromPayload(notificationPayloadMap)}
			}
		}
	}

	return dispatcher.Notification{
		Type:             "http://www.ft.com/thing/ThingChangeType/" + eventType,
		ID:               "http://www.ft.com/thing/" + UUID,
		APIURL:           n.APIBaseURL + "/" + n.Resource + "/" + UUID,
		PublishReference: transactionID,
		LastModified:     event.LastModified,
		Title:            title,
		Standout:		  standout,
		ContentType:      contentType,
	}, nil
}

func getScoopFromPayload(notificationPayloadMap map[string]interface{}) bool {
	var standout = notificationPayloadMap["standout"]
	if standout != nil {
		standoutMap, ok := standout.(map[string]interface{})
		if ok && standoutMap["scoop"] != nil {
			return standoutMap["scoop"].(bool)
		}
	}

	return false
}

func getValueFromPayload(key string, payload map[string]interface{}) string {
	if payload[key] != nil {
		return payload[key].(string)
	}

	return ""
}
