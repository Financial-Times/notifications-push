package dispatch

// subscription types
const (
	AnnotationsType    = "Annotations"
	ArticleContentType = "Article"
	ContentPackageType = "ContentPackage"
	AudioContentType   = "Audio"
	LiveBlogPackage    = "LiveBlogPackage"
	LiveBlogPost       = "LiveBlogPost"
	AllContentType     = "All"
)

// notification types
const (
	ContentUpdateType    = "http://www.ft.com/thing/ThingChangeType/UPDATE"
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
}

// Standout model for a Notification
type Standout struct {
	Scoop bool `json:"scoop"`
}
