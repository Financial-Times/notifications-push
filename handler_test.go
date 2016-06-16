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
	h := handler{newDispatcher(), newCircularBuffer(1)}
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
	defer resp.Body.Close()
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
	notifications := []notificationUPP{
		notificationUPP{PublishReference: "test1"},
		notificationUPP{PublishReference: "test2"},
	}
	cache := newCircularBuffer(2)
	cache.enqueue(notifications[1])
	cache.enqueue(notifications[0])

	h := handler{notificationsCache: cache}
	req, err := http.NewRequest("GET", "http://localhost:8080/notifications", nil)
	if err != nil {
		t.Errorf("[%v]", err)
	}
	w := httptest.NewRecorder()
	h.notifications(w, req)

	expected, err := json.Marshal(notifications)
	if err != nil {
		t.Errorf("[%v]", err)
	}
	actual := w.Body.Bytes()
	if reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected: [%v]. Actual: [%v]", expected, actual)
	}
}

func testRequest(url string) {
	resp, err := http.Get(url)
	if err != nil {
		warnLogger.Println(err)
	}
	defer func() {
		time.Sleep(time.Second * 2)
		resp.Body.Close()
	}()
}
