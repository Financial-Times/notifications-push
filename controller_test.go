package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestHTTPEndpoints_IntegrationTest(t *testing.T) {
	//setting up test controller
	controller := controller{newDispatcher(notificationBuilder{"http://test.api.ft.com"})}
	go controller.dispatcher.distributeEvents()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "notifications") {
			controller.notificationsPush(w, r)
			return
		}
		controller.stats(w, r)
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
