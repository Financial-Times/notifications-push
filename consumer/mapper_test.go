package consumer

import (
	"testing"

	"github.com/Financial-Times/notifications-push/v5/dispatch"
	uuid "github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
)

func TestMapToUpdateNotification(t *testing.T) {
	t.Parallel()

	standout := map[string]interface{}{"scoop": true}
	payload := map[string]interface{}{"title": "This is a title", "standout": standout, "type": "Article"}
	id, _ := uuid.NewV4()
	event := ContentMessage{
		ContentURI:   "http://list-transformer-pr-uk-up.svc.ft.com:8081/list/blah/" + id.String(),
		LastModified: "2016-11-02T10:54:22.234Z",
		Payload:      payload,
	}

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "list",
	}

	n, err := mapper.MapNotification(event, "tid_test1")

	assert.Nil(t, err, "The mapping should not return an error")
	assert.Equal(t, "http://www.ft.com/thing/ThingChangeType/UPDATE", n.Type, "It is an UPDATE notification")
	assert.Equal(t, "This is a title", n.Title, "Title should pe mapped correctly")
	assert.Equal(t, true, n.Standout.Scoop, "Scoop field should be mapped correctly")
	assert.Equal(t, "Article", n.SubscriptionType, "SubscriptionType field should be mapped correctly")
}

func TestMapToUpdateNotification_ForContentWithVersion3UUID(t *testing.T) {
	t.Parallel()

	payload := map[string]interface{}{}

	event := ContentMessage{
		ContentURI:   "http://list-transformer-pr-uk-up.svc.ft.com:8081/list/blah/" + uuid.NewV3(uuid.UUID{}, "id").String(),
		LastModified: "2016-11-02T10:54:22.234Z",
		Payload:      payload,
	}

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "list",
	}

	n, err := mapper.MapNotification(event, "tid_test1")

	assert.Equal(t, "http://www.ft.com/thing/ThingChangeType/UPDATE", n.Type, "It is an UPDATE notification")
	assert.Nil(t, err, "The mapping should not return an error")
	assert.Equal(t, "", n.Title, "Empty title should pe mapped correctly")
}

func TestMapToDeleteNotification(t *testing.T) {
	t.Parallel()
	id, _ := uuid.NewV4()
	event := ContentMessage{
		ContentURI:   "http://list-transformer-pr-uk-up.svc.ft.com:8080/list/blah/" + id.String(),
		LastModified: "2016-11-02T10:54:22.234Z",
		Payload: map[string]interface{}{
			"deleted": true,
		},
	}

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "list",
	}

	n, err := mapper.MapNotification(event, "tid_test1")

	assert.Equal(t, "http://www.ft.com/thing/ThingChangeType/DELETE", n.Type, "It is an DELETE notification")
	assert.Nil(t, err, "The mapping should not return an error")
}

func TestMapToDeleteNotificationWithDeleteFlag(t *testing.T) {
	t.Parallel()
	id, _ := uuid.NewV4()
	event := ContentMessage{
		ContentURI:   "http://list-transformer-pr-uk-up.svc.ft.com:8080/list/blah/" + id.String(),
		LastModified: "2016-11-02T10:54:22.234Z",
		Payload: map[string]interface{}{
			"deleted": true,
		},
	}

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "list",
	}

	n, err := mapper.MapNotification(event, "tid_test1")

	assert.Equal(t, "http://www.ft.com/thing/ThingChangeType/DELETE", n.Type, "It is an DELETE notification")
	assert.Nil(t, err, "The mapping should not return an error")
}

func TestMapToDeleteNotification_ContentTypeHeader(t *testing.T) {
	t.Parallel()
	id, _ := uuid.NewV4()
	event := ContentMessage{
		ContentURI:        "http://list-transformer-pr-uk-up.svc.ft.com:8080/list/blah/" + id.String(),
		LastModified:      "2016-11-02T10:54:22.234Z",
		ContentTypeHeader: "application/vnd.ft-upp-article+json",
		Payload:           map[string]interface{}{"deleted": true},
	}

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "list",
	}

	n, err := mapper.MapNotification(event, "tid_test1")

	assert.Equal(t, "http://www.ft.com/thing/ThingChangeType/DELETE", n.Type, "It should be a DELETE notification")
	assert.Equal(t, "Article", n.SubscriptionType, "SubscriptionType should be mapped based on the message header")
	assert.Nil(t, err, "The mapping should not return an error")
}

func TestNotificationMappingFailure(t *testing.T) {
	t.Parallel()

	event := ContentMessage{
		ContentURI:   "http://list-transformer-pr-uk-up.svc.ft.com:8080/list/blah",
		LastModified: "2016-11-02T10:54:22.234Z",
		Payload:      map[string]interface{}{},
	}

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "list",
	}

	_, err := mapper.MapNotification(event, "tid_test1")

	assert.NotNil(t, err, "The mapping should fail")
}

func TestNotificationMappingFieldsNotExtractedFromPayload(t *testing.T) {
	t.Parallel()

	payload := map[string]interface{}{"foo": "bar"}
	id, _ := uuid.NewV4()
	event := ContentMessage{
		ContentURI:   "http://list-transformer-pr-uk-up.svc.ft.com:8081/list/blah/" + id.String(),
		LastModified: "2016-11-02T10:54:22.234Z",
		Payload:      payload,
	}

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "list",
	}

	n, err := mapper.MapNotification(event, "tid_test1")

	assert.Nil(t, err, "The mapping should not return an error")
	assert.Equal(t, "http://www.ft.com/thing/ThingChangeType/UPDATE", n.Type, "It is an UPDATE notification")
	assert.Empty(t, n.Title, "Title should be empty when it cannot be extracted from payload")
	assert.Equal(t, false, n.Standout.Scoop, "Scoop field should be set to false when it cannot be extracted from payload")
	assert.Equal(t, "", n.SubscriptionType, "SubscriptionType field should be empty when it cannot be extracted from payload")
}

func TestNotificationMappingMetadata(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "content",
	}

	testTID := "tid_test"

	tests := map[string]struct {
		Event    AnnotationsMessage
		HasError bool
		Expected dispatch.Notification
	}{
		"Success": {
			Event: AnnotationsMessage{
				ContentURI:   "http://annotations-rw-neo4j.svc.ft.com/annotations/d1b430b9-0ce2-4b85-9c7b-5b700e8519fe",
				LastModified: "2019-11-10T14:34:25.209Z",
				Payload:      &Annotations{ContentID: "d1b430b9-0ce2-4b85-9c7b-5b700e8519fe"},
			},
			Expected: dispatch.Notification{
				APIURL:           "test.api.ft.com/content/d1b430b9-0ce2-4b85-9c7b-5b700e8519fe",
				ID:               "http://www.ft.com/thing/d1b430b9-0ce2-4b85-9c7b-5b700e8519fe",
				Type:             dispatch.AnnotationUpdateType,
				PublishReference: testTID,
				LastModified:     "2019-11-10T14:34:25.209Z",
				SubscriptionType: dispatch.AnnotationsType,
			},
		},
		"Invalid UUID in contentURI": {
			Event: AnnotationsMessage{
				ContentURI:   "http://annotations-rw-neo4j.svc.ft.com/annotations/invalid-uuid",
				LastModified: "2019-11-10T14:34:25.209Z",
			},
			HasError: true,
		},
		"Missing payload": {
			Event: AnnotationsMessage{
				ContentURI:   "http://annotations-rw-neo4j.svc.ft.com/annotations/d1b430b9-0ce2-4b85-9c7b-5b700e8519fe",
				LastModified: "2019-11-10T14:34:25.209Z",
				Payload:      nil,
			},
			HasError: true,
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			n, err := mapper.MapMetadataNotification(test.Event, testTID)
			if test.HasError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.Expected, n)
		})
	}
}
