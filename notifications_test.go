package main

import (
	"strings"
	"testing"
)

func TestBuildNotification_MissingUUID_NotificationNil(t *testing.T) {
	nb := notificationBuilder{"http://test.api.ft.com"}
	testCmsPubEvent := cmsPublicationEvent{
		UUID: "",
	}
	n := nb.buildNotification(testCmsPubEvent)
	if n != nil {
		t.Errorf("Expected: [nil]. Actual: [%v]", n)
	}
}

func TestBuildNotification_MissingPayload_EventTypeDELETE(t *testing.T) {
	nb := notificationBuilder{"http://test.api.ft.com"}
	testCmsPubEvent := cmsPublicationEvent{
		UUID:    "foobar",
		Payload: nil,
	}
	n := nb.buildNotification(testCmsPubEvent)
	if !strings.HasSuffix(n.Type, "DELETE") {
		t.Errorf("Expected event type DELETE. Actual type URL: [%v]", n.Type)
	}
}

func TestBuildNotification_PayloadIsEmptyString_EventTypeDELETE(t *testing.T) {
	nb := notificationBuilder{"http://test.api.ft.com"}
	testCmsPubEvent := cmsPublicationEvent{
		UUID:    "foobar",
		Payload: "",
	}
	n := nb.buildNotification(testCmsPubEvent)
	if !strings.HasSuffix(n.Type, "DELETE") {
		t.Errorf("Expected event type DELETE. Actual type URL: [%v]", n.Type)
	}
}

func TestBuildNotification_PayloadIsEmptyMap_EventTypeDELETE(t *testing.T) {
	nb := notificationBuilder{"http://test.api.ft.com"}
	testCmsPubEvent := cmsPublicationEvent{
		UUID:    "foobar",
		Payload: map[string]interface{}{},
	}
	n := nb.buildNotification(testCmsPubEvent)
	if !strings.HasSuffix(n.Type, "DELETE") {
		t.Errorf("Expected event type DELETE. Actual type URL: [%v]", n.Type)
	}
}

func TestBuildNotification_HappyScenario(t *testing.T) {
	nb := notificationBuilder{"http://test.api.ft.com"}
	testCmsPubEvent := cmsPublicationEvent{
		UUID:    "baz",
		Payload: []byte(`{ "foo" : "bar" }`),
	}
	n := nb.buildNotification(testCmsPubEvent)
	if !strings.HasSuffix(n.Type, "UPDATE") {
		t.Errorf("Expected event type UPDATE. Actual type URL: [%v]", n.Type)
	}
	if !strings.HasSuffix(n.ID, "baz") {
		t.Errorf("Expected ID suffix: [baz]. Actual ID URL: [%v]", n.ID)
	}
	if !strings.HasSuffix(n.APIURL, "baz") || !strings.HasPrefix(n.APIURL, nb.APIBaseURL) {
		t.Errorf("Expected: APIURL suffix: [baz], prefix: [%v]. Actual APIURL: [%v]", nb.APIBaseURL, n.APIURL)
	}
}
