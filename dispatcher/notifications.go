package dispatcher

// Notification model
type Notification struct {
	APIURL           string   `json:"apiUrl"`
	ID               string   `json:"id"`
	Type             string   `json:"type"`
	PublishReference string   `json:"publishReference,omitempty"`
	LastModified     string   `json:"lastModified,omitempty"`
	NotificationDate string   `json:"notificationDate,omitempty"`
	Title            string   `json:"title,omitempty"`
	Standout         *Standout `json:"standout"`
	ContentType      string   `json:"-"`
}

// Standout model for a Notification
type Standout struct {
	Scoop bool `json:"scoop"`
}
