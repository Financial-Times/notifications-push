package consumer

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Financial-Times/notifications-push/v5/dispatch"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
)

func TestMapToUpdateNotification(t *testing.T) {
	t.Parallel()

	itemJSON := `{"title":"This is a title", "standout": {"scoop": true}, "type": "Article", "publishCount": 2}`

	var payload Item

	err := json.Unmarshal([]byte(itemJSON), &payload)
	assert.NoError(t, err)

	id, _ := uuid.NewV4()
	event := NotificationMessage{
		ContentURI:   "http://list-transformer-pr-uk-up.svc.ft.com:8081/list/blah/" + id.String(),
		LastModified: "2016-11-02T10:54:22.234Z",
		Payload:      payload,
	}

	mapper := NotificationMapper{
		APIBaseURL:      "test.api.ft.com",
		UpdateEventType: "http://www.ft.com/thing/ThingChangeType/UPDATE",
		IncludeScoop:    true,
	}

	n, err := mapper.MapNotification(event, "tid_test1")

	assert.Nil(t, err, "The mapping should not return an error")
	assert.Equal(t, "http://www.ft.com/thing/ThingChangeType/UPDATE", n.Type, "It is an UPDATE notification")
	assert.Equal(t, "This is a title", n.Title, "Title should pe mapped correctly")
	assert.Equal(t, true, n.Standout.Scoop, "Scoop field should be mapped correctly")
	assert.Equal(t, "Article", n.SubscriptionType, "SubscriptionType field should be mapped correctly")
}

func TestMapToCreateNotification(t *testing.T) {
	t.Parallel()

	itemJSON := `{"title":"This is a title", "standout": {"scoop": true}, "type": "Article", "publishCount": 1}`

	var payload Item

	err := json.Unmarshal([]byte(itemJSON), &payload)
	assert.NoError(t, err)

	id, _ := uuid.NewV4()
	event := NotificationMessage{
		ContentURI:   "http://list-transformer-pr-uk-up.svc.ft.com:8081/list/blah/" + id.String(),
		LastModified: "2016-11-02T10:54:22.234Z",
		Payload:      payload,
	}

	mapper := NotificationMapper{
		APIBaseURL:     "test.api.ft.com",
		APIUrlResource: "list",
		IncludeScoop:   true,
	}

	n, err := mapper.MapNotification(event, "tid_test1")

	assert.Nil(t, err, "The mapping should not return an error")
	assert.Equal(t, "http://www.ft.com/thing/ThingChangeType/CREATE", n.Type, "It is an CREATE notification")
	assert.Equal(t, "This is a title", n.Title, "Title should pe mapped correctly")
	assert.Equal(t, true, n.Standout.Scoop, "Scoop field should be mapped correctly")
	assert.Equal(t, "Article", n.SubscriptionType, "SubscriptionType field should be mapped correctly")
}

func TestMapToUpdateNotification_ForContentWithVersion3UUID(t *testing.T) {
	t.Parallel()

	payload := Item{}

	event := NotificationMessage{
		ContentURI:   "http://list-transformer-pr-uk-up.svc.ft.com:8081/list/blah/" + uuid.NewV3(uuid.UUID{}, "id").String(),
		LastModified: "2016-11-02T10:54:22.234Z",
		Payload:      payload,
	}

	mapper := NotificationMapper{
		APIBaseURL:      "test.api.ft.com",
		APIUrlResource:  "list",
		UpdateEventType: "http://www.ft.com/thing/ThingChangeType/UPDATE",
	}

	n, err := mapper.MapNotification(event, "tid_test1")

	assert.Equal(t, "http://www.ft.com/thing/ThingChangeType/UPDATE", n.Type, "It is an UPDATE notification")
	assert.Nil(t, err, "The mapping should not return an error")
	assert.Equal(t, "", n.Title, "Empty title should pe mapped correctly")
}

func TestMapToDeleteNotification(t *testing.T) {
	t.Parallel()

	itemJSON := `{"deleted": true}`
	var payload Item
	err := json.Unmarshal([]byte(itemJSON), &payload)
	assert.NoError(t, err)

	id, _ := uuid.NewV4()
	event := NotificationMessage{
		ContentURI:   "http://list-transformer-pr-uk-up.svc.ft.com:8080/list/blah/" + id.String(),
		LastModified: "2016-11-02T10:54:22.234Z",
		Payload:      payload,
	}

	mapper := NotificationMapper{
		APIBaseURL:     "test.api.ft.com",
		APIUrlResource: "list",
	}

	n, err := mapper.MapNotification(event, "tid_test1")

	assert.Equal(t, "http://www.ft.com/thing/ThingChangeType/DELETE", n.Type, "It is an DELETE notification")
	assert.Nil(t, err, "The mapping should not return an error")
}

func TestMapToDeleteNotification_ContentTypeHeader(t *testing.T) {
	t.Parallel()

	itemJSON := `{"deleted": true}`
	var payload Item
	err := json.Unmarshal([]byte(itemJSON), &payload)
	assert.NoError(t, err)

	id, _ := uuid.NewV4()
	event := NotificationMessage{
		ContentURI:   "http://list-transformer-pr-uk-up.svc.ft.com:8080/list/blah/" + id.String(),
		LastModified: "2016-11-02T10:54:22.234Z",
		ContentType:  "application/vnd.ft-upp-article-internal+json",
		Payload:      payload,
	}

	mapper := NotificationMapper{
		APIBaseURL:     "test.api.ft.com",
		APIUrlResource: "list",
	}

	n, err := mapper.MapNotification(event, "tid_test1")

	assert.Equal(t, "http://www.ft.com/thing/ThingChangeType/DELETE", n.Type, "It should be a DELETE notification")
	assert.Equal(t, "Article", n.SubscriptionType, "SubscriptionType should be mapped based on the message header")
	assert.Nil(t, err, "The mapping should not return an error")
}

func TestNotificationMapper_MapNotification_Page(t *testing.T) {
	t.Parallel()

	itemJSON := `{"title": "Page title", "type": "Page"}`
	var payload Item
	err := json.Unmarshal([]byte(itemJSON), &payload)
	assert.NoError(t, err)

	id, _ := uuid.NewV4()
	event := NotificationMessage{
		ContentURI: "http://upp-notifications-creator.svc.ft.com/content/" + id.String(),
		Payload:    payload,
	}

	mapper := NotificationMapper{
		APIBaseURL:      "test.api.ft.com",
		UpdateEventType: "http://www.ft.com/thing/ThingChangeType/UPDATE",
		APIUrlResource:  "pages",
	}

	notification, err := mapper.MapNotification(event, "tid_test1")

	assert.Nil(t, err, "The mapping should not return an error")
	assert.Equal(t, "test.api.ft.com/pages/"+id.String(), notification.APIURL)
	assert.Equal(t, "http://www.ft.com/thing/"+id.String(), notification.ID)
	assert.Equal(t, "http://www.ft.com/thing/ThingChangeType/UPDATE", notification.Type)
	assert.Equal(t, "Page title", notification.Title)
	assert.Equal(t, "Page", notification.SubscriptionType)
}

func TestNotificationMappingFailure(t *testing.T) {
	t.Parallel()

	payload := Item{}

	event := NotificationMessage{
		ContentURI:   "http://list-transformer-pr-uk-up.svc.ft.com:8080/list/blah",
		LastModified: "2016-11-02T10:54:22.234Z",
		Payload:      payload,
	}

	mapper := NotificationMapper{
		APIBaseURL:     "test.api.ft.com",
		APIUrlResource: "list",
	}

	_, err := mapper.MapNotification(event, "tid_test1")

	assert.NotNil(t, err, "The mapping should fail")
}

func TestNotificationMappingFieldsNotExtractedFromPayload(t *testing.T) {
	t.Parallel()

	itemJSON := `{}`
	var payload Item
	err := json.Unmarshal([]byte(itemJSON), &payload)
	assert.NoError(t, err)

	id, _ := uuid.NewV4()
	event := NotificationMessage{
		ContentURI:   "http://list-transformer-pr-uk-up.svc.ft.com:8081/list/blah/" + id.String(),
		LastModified: "2016-11-02T10:54:22.234Z",
		Payload:      payload,
	}

	mapper := NotificationMapper{
		APIBaseURL:      "test.api.ft.com",
		APIUrlResource:  "list",
		UpdateEventType: "http://www.ft.com/thing/ThingChangeType/UPDATE",
		IncludeScoop:    true,
	}

	n, err := mapper.MapNotification(event, "tid_test1")

	assert.Nil(t, err, "The mapping should not return an error")
	assert.Equal(t, "http://www.ft.com/thing/ThingChangeType/UPDATE", n.Type, "It is an UPDATE notification")
	assert.Empty(t, n.Title, "Title should be empty when it cannot be extracted from payload")
	//assert.Equal(t, false, n.Standout.Scoop, "Scoop field should be set to false when it cannot be extracted from payload")
	assert.Equal(t, "", n.SubscriptionType, "SubscriptionType field should be empty when it cannot be extracted from payload")
}

func TestNotificationMappingMetadata(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL:      "test.api.ft.com",
		UpdateEventType: "http://www.ft.com/thing/ThingChangeType/ANNOTATIONS_UPDATE",
		APIUrlResource:  "content",
	}

	testTID := "tid_test"

	itemJSON := `{"ContentID": "d1b430b9-0ce2-4b85-9c7b-5b700e8519fe"}`
	var payload Item
	err := json.Unmarshal([]byte(itemJSON), &payload)
	assert.NoError(t, err)

	tests := map[string]struct {
		Event    NotificationMessage
		HasError bool
		Expected dispatch.NotificationModel
	}{
		"Success": {
			Event: NotificationMessage{
				ContentURI:   "http://annotations-rw-neo4j.svc.ft.com/annotations/d1b430b9-0ce2-4b85-9c7b-5b700e8519fe",
				LastModified: "2019-11-10T14:34:25.209Z",
				Payload:      payload,
				ContentType:  "application/json",
				MessageType:  annotationMessageType,
			},
			Expected: dispatch.NotificationModel{
				APIURL:           "test.api.ft.com/content/d1b430b9-0ce2-4b85-9c7b-5b700e8519fe",
				ID:               "http://www.ft.com/thing/d1b430b9-0ce2-4b85-9c7b-5b700e8519fe",
				Type:             dispatch.AnnotationUpdateType,
				PublishReference: testTID,
				LastModified:     "2019-11-10T14:34:25.209Z",
				SubscriptionType: dispatch.AnnotationsType,
			},
		},
		"Invalid UUID in contentURI": {
			Event: NotificationMessage{
				ContentURI:   "http://annotations-rw-neo4j.svc.ft.com/annotations/invalid-uuid",
				LastModified: "2019-11-10T14:34:25.209Z",
			},
			HasError: true,
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			n, err := mapper.MapNotification(test.Event, testTID)
			if test.HasError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.Expected, n)
		})
	}
}

func TestNotificationMappingEmptyStandoutForLists(t *testing.T) {
	t.Parallel()

	itemJSON := `{"title":"This is a title", "type": "List"}`
	var payload Item
	err := json.Unmarshal([]byte(itemJSON), &payload)
	assert.NoError(t, err)

	id, _ := uuid.NewV4()
	event := NotificationMessage{
		ContentURI:   "http://list-transformer-pr-uk-up.svc.ft.com:8081/list/blah/" + id.String(),
		LastModified: "2016-11-02T10:54:22.234Z",
		Payload:      payload,
	}

	mapper := NotificationMapper{
		APIBaseURL:      "test.api.ft.com",
		UpdateEventType: "http://www.ft.com/thing/ThingChangeType/UPDATE",
		APIUrlResource:  "lists",
	}

	n, err := mapper.MapNotification(event, "tid_test1")

	mappedAPIURL := fmt.Sprintf("test.api.ft.com/lists/%s", id.String())

	assert.Nil(t, err, "The mapping should not return an error")
	assert.Equal(t, "http://www.ft.com/thing/ThingChangeType/UPDATE", n.Type, "It is an UPDATE notification")
	assert.Equal(t, "This is a title", n.Title, "Title should be mapped correctly")
	assert.Nil(t, n.Standout, "Scoop field should be mapped correctly")
	assert.Equal(t, "List", n.SubscriptionType, "SubscriptionType field should be mapped correctly")
	assert.Equal(t, mappedAPIURL, n.APIURL, "API URL field should be mapped correctly")
}

func TestNotificationMappingEmptyStandoutForPages(t *testing.T) {
	t.Parallel()

	itemJSON := `{"title":"This is a title", "type": "Page"}`
	var payload Item
	err := json.Unmarshal([]byte(itemJSON), &payload)
	assert.NoError(t, err)

	id, _ := uuid.NewV4()
	event := NotificationMessage{
		ContentURI:   "http://upp-notifications-creator.svc.ft.com/content/e" + id.String(),
		LastModified: "2016-11-02T10:54:22.234Z",
		Payload:      payload,
	}

	mapper := NotificationMapper{
		APIBaseURL:      "test.api.ft.com",
		UpdateEventType: "http://www.ft.com/thing/ThingChangeType/UPDATE",
		APIUrlResource:  "pages",
	}

	n, err := mapper.MapNotification(event, "tid_test1")

	mappedAPIURL := fmt.Sprintf("test.api.ft.com/pages/%s", id.String())

	assert.Nil(t, err, "The mapping should not return an error")
	assert.Equal(t, "http://www.ft.com/thing/ThingChangeType/UPDATE", n.Type, "It is an UPDATE notification")
	assert.Equal(t, "This is a title", n.Title, "Title should be mapped correctly")
	assert.Nil(t, n.Standout, "Scoop field should be mapped correctly")
	assert.Equal(t, "Page", n.SubscriptionType, "SubscriptionType field should be mapped correctly")
	assert.Equal(t, mappedAPIURL, n.APIURL, "API URL field should be mapped correctly")
}

func TestMapToCreateNotificationSustainableViews(t *testing.T) {
	t.Parallel()

	itemJSON := `{"title":"Policymakers and business urged to push plant-based products", "standout": {"scoop": true}, "type": "Article", "publishCount": 1, "publication": ["8e6c705e-1132-42a2-8db0-c295e29e8658"]}`

	var payload Item
	err := json.Unmarshal([]byte(itemJSON), &payload)
	assert.NoError(t, err)

	id, _ := uuid.NewV4()
	event := NotificationMessage{
		ContentURI:   "http://list-transformer-pr-uk-up.svc.ft.com:8081/list/blah/" + id.String(),
		LastModified: "2016-11-02T10:54:22.234Z",
		Payload:      payload,
	}

	mapper := NotificationMapper{
		APIBaseURL:     "test.api.ft.com",
		APIUrlResource: "list",
		IncludeScoop:   true,
	}

	n, err := mapper.MapNotification(event, "tid_test1")

	assert.Nil(t, err, "The mapping should not return an error")
	assert.Equal(t, "http://www.ft.com/thing/ThingChangeType/CREATE", n.Type, "It is an CREATE notification")
	assert.Equal(t, "Policymakers and business urged to push plant-based products", n.Title, "Title should pe mapped correctly")
	assert.Equal(t, true, n.Standout.Scoop, "Scoop field should be mapped correctly")
	assert.Equal(t, "Article", n.SubscriptionType, "SubscriptionType field should be mapped correctly")
}

func TestMapToCreateNotificationFTPink(t *testing.T) {
	t.Parallel()

	itemJSON := `{"title":"Hyundai upgrades forecast on strong electric vehicle sales", "standout": {"scoop": true}, "type": "Article", "publishCount": 1, "publication": ["88fdde6c-2aa4-4f78-af02-9f680097cfd6"]}`

	var payload Item
	err := json.Unmarshal([]byte(itemJSON), &payload)
	assert.NoError(t, err)

	id, _ := uuid.NewV4()
	event := NotificationMessage{
		ContentURI:   "http://list-transformer-pr-uk-up.svc.ft.com:8081/list/blah/" + id.String(),
		LastModified: "2016-11-02T10:54:22.234Z",
		Payload:      payload,
	}

	mapper := NotificationMapper{
		APIBaseURL:     "test.api.ft.com",
		APIUrlResource: "list",
		IncludeScoop:   true,
	}

	n, err := mapper.MapNotification(event, "tid_test1")

	assert.Nil(t, err, "The mapping should not return an error")
	assert.Equal(t, "http://www.ft.com/thing/ThingChangeType/CREATE", n.Type, "It is an CREATE notification")
	assert.Equal(t, "Hyundai upgrades forecast on strong electric vehicle sales", n.Title, "Title should pe mapped correctly")
	assert.Equal(t, true, n.Standout.Scoop, "Scoop field should be mapped correctly")
	assert.Equal(t, "Article", n.SubscriptionType, "SubscriptionType field should be mapped correctly")
}
