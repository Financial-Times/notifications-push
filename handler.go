package main

import (
	"bufio"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const evPrefix = "data: "

type handler struct {
	dispatcher         *eventDispatcher
	notificationsCache queue
}

type stats struct {
	NrOfSubscribers int          `json:"nrOfSubscribers"`
	Subscribers     []subscriber `json:"subscribers"`
}

func (h handler) notificationsPush(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	cn, ok := w.(http.CloseNotifier)
	if !ok {
		http.Error(w, "Cannot stream.", http.StatusInternalServerError)
		return
	}

	bw := bufio.NewWriter(w)

	events := make(chan string, 16)
	subscriberEvent := subscriberEvent{
		ch: events,
		subscriber: subscriber{
			Addr:  getClientAddr(r),
			Since: time.Now(),
		},
	}
	h.dispatcher.addSubscriber <- subscriberEvent
	defer func() {
		h.dispatcher.removeSubscriber <- subscriberEvent
	}()

	for {
		select {
		case <-cn.CloseNotify():
			return
		case event := <-events:
			_, err := bw.WriteString(evPrefix + event + "\n\n")
			if err != nil {
				infoLogger.Printf("[%v]", err)
				return
			}
			err = bw.Flush()
			if err != nil {
				infoLogger.Printf("[%v]", err)
				return
			}
			flusher := w.(http.Flusher)
			flusher.Flush()
		}
	}
}

func (h handler) notifications(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "application/json")

	var errMsgPrefix = "Serving /notifications request:"

	bytes, err := json.Marshal(h.notificationsCache.items())
	if err != nil {
		warnLogger.Printf(errMsgPrefix, "[%v]", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	_, err = w.Write(bytes)
	if err != nil {
		warnLogger.Printf(errMsgPrefix, "[%v]", err)
		http.Error(w, "", http.StatusInternalServerError)
	}
}

func (h handler) stats(w http.ResponseWriter, r *http.Request) {
	subscribers := []subscriber{}
	for _, s := range h.dispatcher.subscribers {
		subscribers = append(subscribers, s)
	}
	stats := stats{
		NrOfSubscribers: len(h.dispatcher.subscribers),
		Subscribers:     subscribers,
	}
	bytes, err := json.Marshal(stats)
	if err != nil {
		warnLogger.Printf("[%v]", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-type", "application/json")
	w.Write(bytes)
}

func getClientAddr(r *http.Request) string {
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		addr := strings.Split(xForwardedFor, ",")
		return addr[0]
	}
	return r.RemoteAddr
}
