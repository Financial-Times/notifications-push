package consumer

import "github.com/Financial-Times/notifications-push/v5/publication"

// Item struct represents the payload in NotificationMessage
// We use omitempty to indicate nonmandatory fields
type Item struct {
	ContentType   string                    `json:"type"`
	Title         string                    `json:"title"`
	Publication   *publication.Publications `json:"publication,omitempty"`
	EditorialDesk string                    `json:"editorialDesk,omitempty"`
	PublishCount  int                       `json:"publishCount,omitempty"`
	Deleted       bool                      `json:"deleted,omitempty"`
	Standout      *Standout                 `json:"standout,omitempty"`
	ContentID     string                    `json:"ContentID"`
}

// Standout
type Standout struct {
	EditorsChoice bool `json:"editorsChoice"`
	Exclusive     bool `json:"exclusive"`
	Scoop         bool `json:"scoop"`
}
