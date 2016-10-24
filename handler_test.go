package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestGetClientAddr_XForwardedHeadersPopulated(t *testing.T) {
	testHeaders := http.Header{}
	testHeaders["X-Forwarded-For"] = []string{"1.2.3.4", "5.6.7.8"}
	testRequest := &http.Request{
		Header: testHeaders,
	}

	addr := getClientAddr(testRequest)

	if addr != "1.2.3.4" {
		t.Errorf("Expected: [1.2.3.4]. Actual: [%v]", addr)
	}
}

func TestGetClientAddr_XForwardedHeadersMissing(t *testing.T) {
	testRequest := &http.Request{
		RemoteAddr: "10.10.10.10:10101",
	}

	addr := getClientAddr(testRequest)

	if addr != "10.10.10.10:10101" {
		t.Errorf("Expected: [10.10.10.10:10101]. Actual: [%v]", addr)
	}
}

func TestIntegration_NotificationsPushRequestsServed_NrOfClientsReflectedOnStatsEndpoint(t *testing.T) {
	//setting up test controller
	queue := newUnique(1)
	h := newHandler("content", newDispatcher(), &queue, "http://test.api.ft.com")
	go h.dispatcher.distributeEvents()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "notifications") {
			h.notificationsPush(w, r)
			return
		}
		h.stats(w, r)
	}))
	defer func() {
		ts.Close()
	}()
	nrOfRequests := 10
	for i := 0; i < nrOfRequests; i++ {
		go testRequest(ts.URL + "/notifications")
	}
	time.Sleep(time.Second)
	resp, err := http.Get(ts.URL + "/stats")
	if err != nil {
		t.Error(err)
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	var stats map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&stats)
	if err != nil {
		t.Error(err)
	}
	actualNrOfReqs, ok := stats["nrOfSubscribers"].(float64)
	if !ok || int(actualNrOfReqs) != nrOfRequests {
		t.Errorf("Expected: [%v]. Found: [%v]", nrOfRequests, int(actualNrOfReqs))
	}
}

func TestNotifications_NotificationsInCacheMatchReponseNotifications(t *testing.T) {
	not0 := notificationUPP{
		APIURL:           "http://localhost:8080/content/16ecb25e-3c63-11e6-8716-a4a71e8140b0",
		ID:               "http://www.ft.com/thing/16ecb25e-3c63-11e6-8716-a4a71e8140b0",
		Type:             "http://www.ft.com/thing/ThingChangeType/UPDATE",
		PublishReference: "test1",
		LastModified:     "2016-06-27T14:56:00.988Z",
	}
	not1 := notificationUPP{
		APIURL:           "http://localhost:8080/content/26ecb25e-3c63-11e6-8716-a4a71e8140b0",
		ID:               "http://www.ft.com/thing/26ecb25e-3c63-11e6-8716-a4a71e8140b0",
		Type:             "http://www.ft.com/thing/ThingChangeType/DELETE",
		PublishReference: "test2",
		LastModified:     "2016-06-27T14:57:00.988Z",
	}
	notificationConcreteStructs := []notificationUPP{not0, not1}
	page := notificationsPageUpp{
		RequestURL:    "http://test.api.ft.com/content/notifications",
		Notifications: notificationConcreteStructs,
		Links: []link{link{
			Href: "http://test.api.ft.com/__notifications-push/content/notifications?empty=true",
			Rel:  "next",
		}},
	}

	cache := newUnique(2)
	h := newHandler("content", nil, &cache, "http://test.api.ft.com")
	cache.enqueue(&not0)
	cache.enqueue(&not1)
	req, err := http.NewRequest("GET", "http://localhost:8080/content/notifications", nil)
	if err != nil {
		t.Errorf("[%v]", err)
	}
	w := httptest.NewRecorder()
	h.notifications(w, req)

	expected, err := json.Marshal(page)
	if err != nil {
		t.Errorf("[%v]", err)
	}
	expectedS := string(expected)
	actual := w.Body.String()
	if !reflect.DeepEqual(expectedS, actual) {
		t.Errorf("Expected: [%v]. Actual: [%v]", expectedS, actual)
	}
}

func TestNotifications_EmptyNextPageIsEmpty(t *testing.T) {
	page := notificationsPageUpp{
		RequestURL:    "http://localhost:8080/__notifications-push/content/notifications?empty=true",
		Notifications: []notificationUPP{},
		Links: []link{link{
			Href: "http://localhost:8080/__notifications-push/content/notifications?empty=true",
			Rel:  "next",
		}},
	}
	cache := newUnique(10)
	h := newHandler("content", nil, &cache, "http://localhost:8080")
	req, err := http.NewRequest("GET", "http://localhost:8080/__notifications-push/content/notifications?empty=true", nil)
	if err != nil {
		t.Errorf("[%v]", err)
	}
	w := httptest.NewRecorder()
	h.notifications(w, req)

	expected, err := json.Marshal(page)
	if err != nil {
		t.Errorf("[%v]", err)
	}
	expectedS := string(expected)
	actual := w.Body.String()
	if !reflect.DeepEqual(expectedS, actual) {
		t.Errorf("Expected: [%v]. Actual: [%v]", expectedS, actual)
	}
}

func testRequest(url string) {
	resp, err := http.Get(url)
	if err != nil {
		warnLogger.Println(err)
	}
	defer func() {
		time.Sleep(time.Second * 2)
		err = resp.Body.Close()
		if err != nil {
			warnLogger.Println(err.Error())
		}
	}()
}
