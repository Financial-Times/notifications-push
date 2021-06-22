package dispatch

// subscription types
const (
	AnnotationsType           = "Annotations"
	ArticleContentType        = "Article"
	ContentPackageType        = "ContentPackage"
	AudioContentType          = "Audio"
	LiveBlogPackageType       = "LiveBlogPackage"
	LiveBlogPostType          = "LiveBlogPost"
	ContentPlaceholderType    = "Content"
	PageType                  = "Page"
	AllContentType            = "All"
	CreateEventConsideredType = "INTERNAL_UNSTABLE"
)

// notification types
const (
	ContentUpdateType    = "http://www.ft.com/thing/ThingChangeType/UPDATE"
	ContentCreateType    = "http://www.ft.com/thing/ThingChangeType/CREATE"
	ContentDeleteType    = "http://www.ft.com/thing/ThingChangeType/DELETE"
	AnnotationUpdateType = "http://www.ft.com/thing/ThingChangeType/ANNOTATIONS_UPDATE"
)

// Notification model
type Notification struct {
	APIURL           string    `json:"apiUrl"`
	ID               string    `json:"id"`
	Type             string    `json:"type"`
	SubscriberID     string    `json:"subscriberId,omitempty"`
	PublishReference string    `json:"publishReference,omitempty"`
	LastModified     string    `json:"lastModified,omitempty"`
	NotificationDate string    `json:"notificationDate,omitempty"`
	Title            string    `json:"title,omitempty"`
	Standout         *Standout `json:"standout,omitempty"`
	SubscriptionType string    `json:"-"`
	IsE2ETest        bool      `json:"-"`
}

// Standout model for a Notification
type Standout struct {
	Scoop bool `json:"scoop"`
}

func consolidateNotificationType(notification Notification, isCreateAllowed bool) Notification {
	notificationType := notification.Type
	if notificationType == ContentCreateType && !isCreateAllowed {
		notificationType = ContentUpdateType
	}

	return Notification{
		APIURL:           notification.APIURL,
		ID:               notification.ID,
		Type:             notificationType,
		SubscriberID:     notification.SubscriberID,
		PublishReference: notification.PublishReference,
		LastModified:     notification.LastModified,
		NotificationDate: notification.NotificationDate,
		Title:            notification.Title,
		Standout:         notification.Standout,
		SubscriptionType: notification.SubscriptionType,
		IsE2ETest:        notification.IsE2ETest,
	}
}
