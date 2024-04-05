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
	annotationMessageType  = "concept-annotation"
)

// MapNotification maps the given event to a new notification.
func (n NotificationMapper) MapNotification(event NotificationMessage, transactionID string) (dispatch.NotificationModel, error) {
	uuid := UUIDRegexp.FindString(event.ContentURI)
	if uuid == "" {
		// nolint:golint
		return dispatch.NotificationModel{}, fmt.Errorf("ContentURI does not contain a UUID")
	}

	eventType := n.UpdateEventType
	contentType := event.Payload.ContentType
	if contentType == "" && event.MessageType == annotationMessageType {
		contentType = dispatch.AnnotationsType
	}

	// If it's a create event.
	if event.Payload.PublishCount == 1 {
		eventType = dispatch.ContentCreateType
	}

	// If it's a delete event.
	if event.Payload.Deleted {
		eventType = dispatch.ContentDeleteType
		contentType = resolveTypeFromMessageHeader(event.ContentType)
	}

	if event.Payload.IsRelatedContentNotification {
		eventType = dispatch.RelatedContentType
	}

	notification := dispatch.NotificationModel{
		Type:             eventType,
		ID:               "http://www.ft.com/thing/" + uuid,
		APIURL:           fmt.Sprintf("%s/%s/%s", n.APIBaseURL, n.APIUrlResource, uuid),
		PublishReference: transactionID,
		LastModified:     event.LastModified,
		EditorialDesk:    event.Payload.EditorialDesk,
		Title:            event.Payload.Title,
		SubscriptionType: contentType,
		Publication:      event.Payload.Publication,
	}

	if event.Payload.Standout != nil {
		notification.Standout = &dispatch.Standout{Scoop: event.Payload.Standout.Scoop}
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
