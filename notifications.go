package main

type notificationBuilder struct {
	APIBaseURL string
}

type notification struct {
	APIURL string `json:"apiUrl"`
	ID     string `json:"id"`
	Type   string `json:"type"`
}

type notificationUPP struct {
	notification
	LastModified     string `json:"lastModified"`
	PublishReference string `json:"publishReference"`
}

func (nb notificationBuilder) buildNotification(cmsPubEvent cmsPublicationEvent) *notification {
	if cmsPubEvent.UUID == "" {
		return nil
	}

	empty := false
	switch v := cmsPubEvent.Payload.(type) {
	case nil:
		empty = true
	case string:
		if len(v) == 0 {
			empty = true
		}
	case map[string]interface{}:
		if len(v) == 0 {
			empty = true
		}
	}
	eventType := "UPDATE"
	if empty {
		eventType = "DELETE"
	}
	return &notification{
		Type:   "http://www.ft.com/thing/ThingChangeType/" + eventType,
		ID:     "http://www.ft.com/thing/" + cmsPubEvent.UUID,
		APIURL: nb.APIBaseURL + "/content/" + cmsPubEvent.UUID,
	}
}

func buildUPPNotification(n *notification, tid, lastModified string) *notificationUPP {
	return &notificationUPP{
		notification:     *n,
		LastModified:     lastModified,
		PublishReference: tid,
	}
}
