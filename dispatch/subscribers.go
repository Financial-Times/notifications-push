package dispatch

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"time"

	log "github.com/Financial-Times/go-logger"
	"github.com/satori/go.uuid"
)

// Subscriber represents the interface of a generic subscriber to a push stream
type Subscriber interface {
	Id() string
	send(n Notification) error
	matchesContentType(n Notification) bool
	NotificationChannel() chan string
	writeOnMsgChannel(string)
	Address() string
	Since() time.Time
	AcceptedContentType() string
}

// StandardSubscriber implements a standard subscriber
type standardSubscriber struct {
	id                  string
	notificationChannel chan string
	addr                string
	sinceTime           time.Time
	acceptedContentType string
}

// NewStandardSubscriber returns a new instance of a standard subscriber
func NewStandardSubscriber(address string, contentType string) Subscriber {
	notificationChannel := make(chan string, 16)
	return &standardSubscriber{
		id:                  uuid.NewV4().String(),
		notificationChannel: notificationChannel,
		addr:                address,
		sinceTime:           time.Now(),
		acceptedContentType: contentType,
	}
}

// Id returns the uniquely generated subscriber identifier
// Returned value is assigned during the construction phase.
func (s *standardSubscriber) Id() string {
	return s.id
}

// Address returns the IP address of the standard subscriber
func (s *standardSubscriber) Address() string {
	return s.addr
}

// AcceptedContentType returns the accepted content type for which notifications are returned
func (s *standardSubscriber) AcceptedContentType() string {
	return s.acceptedContentType
}

// Since returns the time since a subscriber have been registered
func (s *standardSubscriber) Since() time.Time {
	return s.sinceTime
}

func (s *standardSubscriber) matchesContentType(n Notification) bool {
	if strings.ToLower(s.acceptedContentType) == "all" {
		return true
	}

	if strings.Contains(n.Type, "DELETE") && n.ContentType == "" {
		return true
	}

	return strings.ToLower(s.acceptedContentType) == strings.ToLower(n.ContentType)
}

func (s *standardSubscriber) send(n Notification) error {
	notificationMsg, err := buildStandardNotificationMsg(n)
	if err != nil {
		return err
	}
	s.writeOnMsgChannel(notificationMsg)
	return nil
}

// NotificationChannel returns the channel that can be used to send
// notifications to the standard subscriber
func (s *standardSubscriber) NotificationChannel() chan string {
	return s.notificationChannel
}

func (s *standardSubscriber) writeOnMsgChannel(msg string) {
	select {
	case s.notificationChannel <- msg:
	default:
		log.WithField("subscriberId", s.id).
			WithField("subscriber", s.Address()).
			WithField("message", msg).
			Warn("Subscriber lagging behind...")
	}
}

func buildStandardNotificationMsg(n Notification) (string, error) {
	n.PublishReference = ""
	n.LastModified = ""
	n.NotificationDate = ""

	return buildNotificationMsg(n)
}

func buildNotificationMsg(n Notification) (string, error) {
	jsonNotification, err := MarshalNotificationsJSON([]Notification{n})
	if err != nil {
		return "", err
	}

	return string(jsonNotification), err
}

// MarshalNotificationsJSON returns the JSON encoding of n. For notifications, we do not use the standard function json.Marshal()
// because that will always escape special characters (<,>,&) in unicode format ("\u0026P" and similar)
func MarshalNotificationsJSON(n []Notification) ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)

	err := encoder.Encode(n)
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), err
}

// monitorSubscriber implements a Monitor subscriber
type monitorSubscriber struct {
	Subscriber
}

// NewMonitorSubscriber returns a new instance of a Monitor subscriber
func NewMonitorSubscriber(address string, contentType string) Subscriber {
	return &monitorSubscriber{NewStandardSubscriber(address, contentType)}
}

func (m *monitorSubscriber) send(n Notification) error {
	// -- set subscriberId for NPM traceability only for monitor mode subscribers
	n.SubscriberId = m.Id()
	// --

	notificationMsg, err := buildMonitorNotificationMsg(n)
	if err != nil {
		return err
	}
	m.writeOnMsgChannel(notificationMsg)
	return nil
}

func buildMonitorNotificationMsg(n Notification) (string, error) {
	return buildNotificationMsg(n)
}

// MarshalJSON returns the JSON representation of a StandardSubscriber
func (s *standardSubscriber) MarshalJSON() ([]byte, error) {
	return json.Marshal(newSubscriberPayload(s))
}

// MarshalJSON returns the JSON representation of a MonitorSubscriber
func (m *monitorSubscriber) MarshalJSON() ([]byte, error) {
	return json.Marshal(newSubscriberPayload(m))
}

// SubscriberPayload is the JSON representation of a generic subscriber
type SubscriberPayload struct {
	Id                 string `json:"id"`
	Address            string `json:"address"`
	Since              string `json:"since"`
	ConnectionDuration string `json:"connectionDuration"`
	Type               string `json:"type"`
}

func newSubscriberPayload(s Subscriber) *SubscriberPayload {
	return &SubscriberPayload{
		Id:                 s.Id(),
		Address:            s.Address(),
		Since:              s.Since().Format(time.StampMilli),
		ConnectionDuration: time.Since(s.Since()).String(),
		Type:               reflect.TypeOf(s).Elem().String(),
	}
}
