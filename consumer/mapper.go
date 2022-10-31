package consumer

import (
	"fmt"
	"regexp"

	"github.com/Financial-Times/notifications-push/v5/dispatch"
)

// NotificationMapper maps CmsPublicationEvents to Notifications
type NotificationMapper struct {
	APIBaseURL      string
	UpdateEventType string
	APIUrlResource  string
	IncludeScoop    bool
}

// UUIDRegexp enables to check if a string matches a UUID
var UUIDRegexp = regexp.MustCompile("[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}")

const (
	payloadDeletedKey      = "deleted"
	payloadPublishCountKey = "publishCount"
)

// MapNotification maps the given event to a new notification.
func (n NotificationMapper) MapNotification(event NotificationMessage, transactionID string) (dispatch.NotificationModel, error) {
	uuid := UUIDRegexp.FindString(event.ContentURI)
	if uuid == "" {
		// nolint:golint
		return dispatch.NotificationModel{}, fmt.Errorf("ContentURI does not contain a UUID")
	}

	notificationPayloadMap, ok := event.Payload.(map[string]interface{})
	if !ok {
		return dispatch.NotificationModel{}, fmt.Errorf("invalid payload type: %T", event.Payload)
	}

	eventType := n.UpdateEventType
	title := getValueFromPayload("title", notificationPayloadMap)
	contentType := getValueFromPayload("type", notificationPayloadMap)

	// If it's a create event.
	publishCountStr := fmt.Sprintf("%v", notificationPayloadMap[payloadPublishCountKey])
	if publishCountStr == "1" {
		eventType = dispatch.ContentCreateType
	}

	// If it's a delete event.
	if isTrue, deletedIsPresent := notificationPayloadMap[payloadDeletedKey].(bool); deletedIsPresent && isTrue {
		eventType = dispatch.ContentDeleteType
		contentType = resolveTypeFromMessageHeader(event.ContentType)
	}

	notification := dispatch.NotificationModel{
		Type:             eventType,
		ID:               "http://www.ft.com/thing/" + uuid,
		APIURL:           fmt.Sprintf("%s/%s/%s", n.APIBaseURL, n.APIUrlResource, uuid),
		PublishReference: transactionID,
		LastModified:     event.LastModified,
		Title:            title,
		SubscriptionType: contentType,
	}

	if n.IncludeScoop {
		scoop := getScoopFromPayload(notificationPayloadMap)
		notification.Standout = &dispatch.Standout{Scoop: scoop}
	}

	return notification, nil
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
	case "application/vnd.ft-upp-page+json":
		return dispatch.PageType
	case "application/vnd.ft-upp-list+json":
		return dispatch.ListType
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
