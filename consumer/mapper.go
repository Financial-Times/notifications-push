package consumer

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/Financial-Times/notifications-push/v5/dispatch"
)

// NotificationMapper maps CmsPublicationEvents to Notifications
type NotificationMapper struct {
	APIBaseURL string
	Resource   string
}

// UUIDRegexp enables to check if a string matches a UUID
var UUIDRegexp = regexp.MustCompile("[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}")

const payloadDeletedFlag = "deleted"

// MapNotification maps the given event to a new notification.
func (n NotificationMapper) MapNotification(event ContentMessage, transactionID string) (dispatch.Notification, error) {
	UUID := UUIDRegexp.FindString(event.ContentURI)
	if UUID == "" {
		// nolint:golint
		return dispatch.Notification{}, errors.New("ContentURI does not contain a UUID")
	}

	var eventType string
	var scoop bool
	var title = ""
	var contentType = ""

	notificationPayloadMap, ok := event.Payload.(map[string]interface{})
	if !ok {
		return dispatch.Notification{}, fmt.Errorf("invalid payload type: %T", event.Payload)
	}

	deleteFlag, _ := notificationPayloadMap[payloadDeletedFlag].(bool)

	if deleteFlag {
		eventType = dispatch.ContentDeleteType
		contentType = resolveTypeFromMessageHeader(event.ContentTypeHeader)
	} else {
		eventType = dispatch.ContentUpdateType
		title = getValueFromPayload("title", notificationPayloadMap)
		contentType = getValueFromPayload("type", notificationPayloadMap)
		scoop = getScoopFromPayload(notificationPayloadMap)
	}

	return dispatch.Notification{
		Type:             eventType,
		ID:               "http://www.ft.com/thing/" + UUID,
		APIURL:           n.APIBaseURL + "/" + n.Resource + "/" + UUID,
		PublishReference: transactionID,
		LastModified:     event.LastModified,
		Title:            title,
		Standout:         &dispatch.Standout{Scoop: scoop},
		SubscriptionType: contentType,
	}, nil
}

func (n NotificationMapper) MapMetadataNotification(event AnnotationsMessage, transactionID string) (dispatch.Notification, error) {
	UUID := UUIDRegexp.FindString(event.ContentURI)
	if UUID == "" {
		return dispatch.Notification{}, errors.New("contentURI does not contain a UUID")
	}
	if event.Payload == nil {
		return dispatch.Notification{}, errors.New("payload missing")
	}

	return dispatch.Notification{
		Type:             dispatch.AnnotationUpdateType,
		ID:               "http://www.ft.com/thing/" + UUID,
		APIURL:           n.APIBaseURL + "/" + n.Resource + "/" + UUID,
		PublishReference: transactionID,
		SubscriptionType: dispatch.AnnotationsType,
		LastModified:     event.LastModified,
	}, nil
}

func resolveTypeFromMessageHeader(contentTypeHeader string) string {
	switch contentTypeHeader {
	case "application/vnd.ft-upp-article-internal+json":
		return dispatch.ArticleContentType
	case "application/vnd.ft-upp-content-package+json":
		return dispatch.ContentPackageType
	case "application/vnd.ft-upp-audio+json":
		return dispatch.AudioContentType
	case "application/vnd.ft-upp-live-blog-post-internal+json":
		return dispatch.LiveBlogPostType
	case "application/vnd.ft-upp-live-blog-package-internal+json":
		return dispatch.LiveBlogPackageType
	default:
		return ""
	}
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
