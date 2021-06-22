package dispatch

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"time"

	uuid "github.com/gofrs/uuid"
)

const notificationBuffer = 16

var ErrSubLagging = errors.New("subscriber lagging behind")

// Subscriber represents the interface of a generic subscriber to a push stream
type Subscriber interface {
	ID() string
	Notifications() <-chan string
	Address() string
	Since() time.Time
	SubTypes() []string
	isSubscribedForCreate() bool
}

type NotificationConsumer interface {
	Subscriber
	Send(n Notification) error
}

// StandardSubscriber implements a standard subscriber
type StandardSubscriber struct {
	id                      string
	notificationChannel     chan string
	addr                    string
	sinceTime               time.Time
	acceptedTypes           []string
	sendCreateNotifications bool
}

// NewStandardSubscriber returns a new instance of a standard subscriber
func NewStandardSubscriber(address string, subTypes []string, isCreateEventSubscription bool) (*StandardSubscriber, error) {
	notificationChannel := make(chan string, notificationBuffer)
	id, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	return &StandardSubscriber{
		id:                      id.String(),
		notificationChannel:     notificationChannel,
		addr:                    address,
		sinceTime:               time.Now(),
		acceptedTypes:           subTypes,
		sendCreateNotifications: isCreateEventSubscription,
	}, nil
}

// Id returns the uniquely generated subscriber identifier
// Returned value is assigned during the construction phase.
func (s *StandardSubscriber) ID() string {
	return s.id
}

// Address returns the IP address of the standard subscriber
func (s *StandardSubscriber) Address() string {
	return s.addr
}

// SubTypes returns the accepted subscription type for which notifications are returned
func (s *StandardSubscriber) SubTypes() []string {
	return s.acceptedTypes
}

// Since returns the time since a subscriber have been registered
func (s *StandardSubscriber) Since() time.Time {
	return s.sinceTime
}

// Notifications returns the channel that can provides serialized notifications send to the subscriber
func (s *StandardSubscriber) Notifications() <-chan string {
	return s.notificationChannel
}

// isSubscribedForCreate returns if the subscriber is subscribed for create events
func (s *StandardSubscriber) isSubscribedForCreate() bool {
	return s.sendCreateNotifications
}

// Send tries to send notification to the subscriber.
// It removes the monitoring fields from the notification. Serializes it as string and pushes it to the subscriber
func (s *StandardSubscriber) Send(n Notification) error {
	msg, err := buildStandardNotificationMsg(n)
	if err != nil {
		return err
	}
	select {
	case s.notificationChannel <- msg:
		return nil
	default:
		return ErrSubLagging
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
type MonitorSubscriber struct {
	id                      string
	notificationChannel     chan string
	addr                    string
	sinceTime               time.Time
	acceptedTypes           []string
	sendCreateNotifications bool
}

func (m *MonitorSubscriber) ID() string {
	return m.id
}

func (m *MonitorSubscriber) Notifications() <-chan string {
	return m.notificationChannel
}

func (m *MonitorSubscriber) Address() string {
	return m.addr
}

func (m *MonitorSubscriber) Since() time.Time {
	return m.sinceTime
}

func (m *MonitorSubscriber) SubTypes() []string {
	return m.acceptedTypes
}

func (m *MonitorSubscriber) isSubscribedForCreate() bool {
	return m.sendCreateNotifications
}

func (m *MonitorSubscriber) Send(n Notification) error {
	// -- set subscriberId for NPM traceability only for monitor mode subscribers
	n.SubscriberID = m.ID()
	msg, err := buildMonitorNotificationMsg(n)
	if err != nil {
		return err
	}
	select {
	case m.notificationChannel <- msg:
		return nil
	default:
		return ErrSubLagging
	}
}

// NewMonitorSubscriber returns a new instance of a Monitor subscriber
func NewMonitorSubscriber(address string, subTypes []string, isCreateEventSubscription bool) (*MonitorSubscriber, error) {
	notificationChannel := make(chan string, notificationBuffer)
	id, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	return &MonitorSubscriber{
		id:                      id.String(),
		notificationChannel:     notificationChannel,
		addr:                    address,
		sinceTime:               time.Now(),
		acceptedTypes:           subTypes,
		sendCreateNotifications: isCreateEventSubscription,
	}, nil
}

func buildMonitorNotificationMsg(n Notification) (string, error) {
	return buildNotificationMsg(n)
}

// MarshalJSON returns the JSON representation of a StandardSubscriber
func (s *StandardSubscriber) MarshalJSON() ([]byte, error) {
	return json.Marshal(newSubscriberPayload(s))
}

// MarshalJSON returns the JSON representation of a MonitorSubscriber
func (m *MonitorSubscriber) MarshalJSON() ([]byte, error) {
	return json.Marshal(newSubscriberPayload(m))
}

// SubscriberPayload is the JSON representation of a generic subscriber
type SubscriberPayload struct {
	ID                 string `json:"id"`
	Address            string `json:"address"`
	Since              string `json:"since"`
	ConnectionDuration string `json:"connectionDuration"`
	Type               string `json:"type"`
}

func newSubscriberPayload(s Subscriber) *SubscriberPayload {
	return &SubscriberPayload{
		ID:                 s.ID(),
		Address:            s.Address(),
		Since:              s.Since().Format(time.StampMilli),
		ConnectionDuration: time.Since(s.Since()).String(),
		Type:               reflect.TypeOf(s).Elem().String(),
	}
}
