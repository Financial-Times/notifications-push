package dispatch

import (
	"github.com/Financial-Times/notifications-push/v5/access"
	"github.com/Financial-Times/notifications-push/v5/publication"
)

// subscription types
const (
	AnnotationsType        = "Annotations"
	ArticleContentType     = "Article"
	ContentPackageType     = "ContentPackage"
	AudioContentType       = "Audio"
	LiveBlogPackageType    = "LiveBlogPackage"
	LiveBlogPostType       = "LiveBlogPost"
	PageType               = "Page"
	AllContentType         = "All"
	ListType               = "List"
	ContentPlaceholderType = "Content"
)

// notification types
const (
	ContentUpdateType    = "http://www.ft.com/thing/ThingChangeType/UPDATE"
	ContentCreateType    = "http://www.ft.com/thing/ThingChangeType/CREATE"
	ContentDeleteType    = "http://www.ft.com/thing/ThingChangeType/DELETE"
	AnnotationUpdateType = "http://www.ft.com/thing/ThingChangeType/ANNOTATIONS_UPDATE"
)

// NotificationModel model
type NotificationModel struct {
	APIURL           string
	ID               string
	Type             string
	SubscriberID     string
	EditorialDesk    string
	PublishReference string
	LastModified     string
	NotificationDate string
	Title            string
	Standout         *Standout
	SubscriptionType string
	IsE2ETest        bool
	Publication      *publication.Publications
}

// NotificationResponse view
type NotificationResponse struct {
	APIURL           string    `json:"apiUrl"`
	ID               string    `json:"id"`
	Type             string    `json:"type"`
	SubscriberID     string    `json:"subscriberId,omitempty"`
	PublishReference string    `json:"publishReference,omitempty"`
	LastModified     string    `json:"lastModified,omitempty"`
	NotificationDate string    `json:"notificationDate,omitempty"`
	Title            string    `json:"title,omitempty"`
	Standout         *Standout `json:"standout,omitempty"`
}

// Standout model for a NotificationResponse
type Standout struct {
	Scoop bool `json:"scoop"`
}

func CreateNotificationResponse(notification NotificationModel, subscriberOptions *access.NotificationSubscriptionOptions) NotificationResponse {
	notificationType := notification.Type

	if notificationType == ContentCreateType && !subscriberOptions.ReceiveAdvancedNotifications {
		notificationType = ContentUpdateType
	}

	return NotificationResponse{
		APIURL:           notification.APIURL,
		ID:               notification.ID,
		Type:             notificationType,
		SubscriberID:     notification.SubscriberID,
		PublishReference: notification.PublishReference,
		LastModified:     notification.LastModified,
		NotificationDate: notification.NotificationDate,
		Title:            notification.Title,
		Standout:         notification.Standout,
	}
}
