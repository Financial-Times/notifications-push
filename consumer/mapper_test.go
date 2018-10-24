package consumer

import (
	"testing"

	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
)

func TestMapToUpdateNotification(t *testing.T) {
	standout := map[string]interface{}{"scoop": true}
	payload := map[string]interface{}{"title": "This is a title", "standout": standout, "type": "Article"}

	event := PublicationEvent{
		ContentURI:   "http://list-transformer-pr-uk-up.svc.ft.com:8081/list/blah/" + uuid.NewV4().String(),
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
	assert.Equal(t, "Article", n.ContentType, "ContentType field should be mapped correctly")
}

func TestMapToUpdateNotification_ForContentWithVersion3UUID(t *testing.T) {
	payload := struct{ Foo string }{"bar"}

	event := PublicationEvent{
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
	event := PublicationEvent{
		ContentURI:   "http://list-transformer-pr-uk-up.svc.ft.com:8080/list/blah/" + uuid.NewV4().String(),
		LastModified: "2016-11-02T10:54:22.234Z",
		Payload:      "",
	}

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "list",
	}

	n, err := mapper.MapNotification(event, "tid_test1")

	assert.Equal(t, "http://www.ft.com/thing/ThingChangeType/DELETE", n.Type, "It is an UPDATE notification")
	assert.Nil(t, err, "The mapping should not return an error")
}

func TestMapToDeleteNotification_ContentTypeHeader(t *testing.T) {
	event := PublicationEvent{
		ContentURI:        "http://list-transformer-pr-uk-up.svc.ft.com:8080/list/blah/" + uuid.NewV4().String(),
		LastModified:      "2016-11-02T10:54:22.234Z",
		ContentTypeHeader: "application/vnd.ft-upp-article+json",
		Payload:           "",
	}

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "list",
	}

	n, err := mapper.MapNotification(event, "tid_test1")

	assert.Equal(t, "http://www.ft.com/thing/ThingChangeType/DELETE", n.Type, "It should be a DELETE notification")
	assert.Equal(t, "Article", n.ContentType, "ContentType should be mapped based on the message header")
	assert.Nil(t, err, "The mapping should not return an error")
}

func TestNotificationMappingFailure(t *testing.T) {
	event := PublicationEvent{
		ContentURI:   "http://list-transformer-pr-uk-up.svc.ft.com:8080/list/blah",
		LastModified: "2016-11-02T10:54:22.234Z",
		Payload:      "",
	}

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "list",
	}

	_, err := mapper.MapNotification(event, "tid_test1")

	assert.NotNil(t, err, "The mapping should fail")
}

func TestNotificationMappingFieldsNotExtractedFromPayload(t *testing.T) {
	payload := map[string]interface{}{"foo": "bar"}

	event := PublicationEvent{
		ContentURI:   "http://list-transformer-pr-uk-up.svc.ft.com:8081/list/blah/" + uuid.NewV4().String(),
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
	assert.Equal(t, "", n.ContentType, "ContentType field should be empty when it cannot be extracted from payload")
}
