package dispatch

// subscription types
const (
	AnnotationsType        = "Annotations"
	ArticleContentType     = "Article"
	ContentPackageType     = "ContentPackage"
	AudioContentType       = "Audio"
	LiveBlogPackageType    = "LiveBlogPackage"
	LiveBlogPostType       = "LiveBlogPost"
	ContentPlaceholderType = "Content"
	PageType               = "Page"
	AllContentType         = "All"
	ListsType              = "List"
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
	PublishReference string
	LastModified     string
	NotificationDate string
	Title            string
	Standout         *Standout
	SubscriptionType string
	IsE2ETest        bool
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

func CreateNotificationResponse(notification NotificationModel, subscriberOptions []SubscriptionOption) NotificationResponse {
	notificationType := notification.Type

	hasCreateEventOption := false
	for _, o := range subscriberOptions {
		if o == CreateEventOption {
			hasCreateEventOption = true
			break
		}
	}

	if notificationType == ContentCreateType && !hasCreateEventOption {
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
