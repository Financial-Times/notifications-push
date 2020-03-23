package dispatch

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Financial-Times/go-logger"
	logTest "github.com/Financial-Times/go-logger/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	contentTypeFilter = "All"
	typeArticle       = "Article"
	annotationSubType = "Annotations"
)

var delay = 2 * time.Second
var historySize = 10

var n1 = Notification{
	APIURL:           "http://api.ft.com/content/7998974a-1e97-11e6-b286-cddde55ca122",
	ID:               "http://www.ft.com/thing/7998974a-1e97-11e6-b286-cddde55ca122",
	Type:             "http://www.ft.com/thing/ThingChangeType/UPDATE",
	PublishReference: "tid_test1",
	LastModified:     "2016-11-02T10:54:22.234Z",
	SubscriptionType: "ContentPackage",
}

var n2 = Notification{
	APIURL:           "http://api.ft.com/content/7998974a-1e97-11e6-b286-cddde55ca122",
	ID:               "http://www.ft.com/thing/7998974a-1e97-11e6-b286-cddde55ca122",
	Type:             "http://www.ft.com/thing/ThingChangeType/DELETE",
	PublishReference: "tid_test2",
	LastModified:     "2016-11-02T10:55:24.244Z",
}

var annNotif = Notification{
	APIURL:           "http://api.ft.com/content/7998974a-1e97-11e6-b286-cddde55ca122",
	ID:               "http://www.ft.com/thing/7998974a-1e97-11e6-b286-cddde55ca122",
	Type:             "http://www.ft.com/thing/ThingChangeType/ANNOTATIONS_UPDATE",
	PublishReference: "tid_test3",
	SubscriptionType: "Annotations",
}

var zeroTime = time.Time{}

func init() {
	logger.InitLogger("notifications-push", "PANIC")
}

func TestShouldDispatchNotificationsToMultipleSubscribers(t *testing.T) {
	h := NewHistory(historySize)
	d := NewDispatcher(delay, h)

	m := d.Subscribe("192.168.1.2", contentTypeFilter, true)
	s := d.Subscribe("192.168.1.3", contentTypeFilter, false)

	go d.Start()
	defer d.Stop()

	notBefore := time.Now()
	d.Send(n1)
	// sleep for ensuring that notifications come in the order they are send.
	<-time.After(time.Millisecond * 20)
	d.Send(n2)

	actualN1StdMsg := <-s.Notifications()
	verifyNotificationResponse(t, n1, zeroTime, zeroTime, actualN1StdMsg)

	actualN2StdMsg := <-s.Notifications()
	verifyNotificationResponse(t, n2, zeroTime, zeroTime, actualN2StdMsg)

	actualN1MonitorMsg := <-m.Notifications()
	verifyNotificationResponse(t, n1, notBefore, time.Now(), actualN1MonitorMsg)

	actualN2MonitorMsg := <-m.Notifications()
	verifyNotificationResponse(t, n2, notBefore, time.Now(), actualN2MonitorMsg)
}

func TestShouldDispatchNotificationsToSubscribersByType(t *testing.T) {
	hook := logTest.NewTestHook("notifications-push")
	defer hook.Reset()

	h := NewHistory(historySize)
	d := NewDispatcher(delay, h)

	m := d.Subscribe("192.168.1.2", contentTypeFilter, true)
	s := d.Subscribe("192.168.1.3", typeArticle, false)
	annSub := d.Subscribe("192.168.1.4", annotationSubType, false)

	go d.Start()
	defer d.Stop()

	notBefore := time.Now()
	d.Send(n1)
	// sleep for ensuring that notifications come in the order they are send.
	<-time.After(time.Millisecond * 20)
	d.Send(n2)
	<-time.After(time.Millisecond * 20)
	d.Send(annNotif)

	actualN2StdMsg := <-s.Notifications()
	verifyNotificationResponse(t, n2, zeroTime, zeroTime, actualN2StdMsg)

	msg := <-annSub.Notifications()
	verifyNotificationResponse(t, annNotif, notBefore, time.Now(), msg)

	actualN1MonitorMsg := <-m.Notifications()
	verifyNotificationResponse(t, n1, notBefore, time.Now(), actualN1MonitorMsg)

	actualN2MonitorMsg := <-m.Notifications()
	verifyNotificationResponse(t, n2, notBefore, time.Now(), actualN2MonitorMsg)

	for _, e := range hook.AllEntries() {
		tid := e.Data["transaction_id"]
		switch e.Message {
		case "Skipping subscriber.":
			assert.Contains(t, [...]string{n1.APIURL, n2.APIURL, annNotif.APIURL}, e.Data["resource"], "skipped resource")
			assert.Contains(t, [...]string{s.Address(), m.Address(), annSub.Address()}, e.Data["subscriberAddress"], "skipped subscriber address")
		case "Processed subscribers.":
			switch tid {
			case "tid_test1":
				assert.Equal(t, 1, e.Data["sent"], "sent (%s)", tid)
				assert.Equal(t, 0, e.Data["failed"], "failed (%s)", tid)
				assert.Equal(t, 2, e.Data["skipped"], "skipped (%s)", tid)
			case "tid_test2":
				assert.Equal(t, 2, e.Data["sent"], "sent (%s)", tid)
				assert.Equal(t, 0, e.Data["failed"], "failed (%s)", tid)
				assert.Equal(t, 1, e.Data["skipped"], "skipped (%s)", tid)
			case "tid_test3":
				assert.Equal(t, 1, e.Data["sent"], "sent (%s)", tid)
				assert.Equal(t, 0, e.Data["failed"], "failed (%s)", tid)
				assert.Equal(t, 2, e.Data["skipped"], "skipped (%s)", tid)
			default:
				assert.Fail(t, "unexpected transaction_id", "%s (%s)", e.Message, tid)
			}
		default:
		}
	}
}

func TestAddAndRemoveOfSubscribers(t *testing.T) {
	h := NewHistory(historySize)
	d := NewDispatcher(delay, h)

	m := d.Subscribe("192.168.1.2", contentTypeFilter, true).(NotificationConsumer)
	s := d.Subscribe("192.168.1.3", contentTypeFilter, false).(NotificationConsumer)

	go d.Start()
	defer d.Stop()

	assert.Contains(t, d.Subscribers(), s, "Dispatcher contains standard subscriber")
	assert.Contains(t, d.Subscribers(), m, "Dispatcher contains monitor subscriber")
	assert.Equal(t, 2, len(d.Subscribers()), "Dispatcher has 2 subscribers")

	d.Unsubscribe(s)

	assert.NotContains(t, d.Subscribers(), s, "Dispatcher does not contain standard subscriber")
	assert.Contains(t, d.Subscribers(), m, "Dispatcher contains monitor subscriber")
	assert.Equal(t, 1, len(d.Subscribers()), "Dispatcher has 1 subscriber")

	d.Unsubscribe(m)

	assert.NotContains(t, d.Subscribers(), s, "Dispatcher does not contain standard subscriber")
	assert.NotContains(t, d.Subscribers(), m, "Dispatcher does not contain monitor subscriber")
	assert.Equal(t, 0, len(d.Subscribers()), "Dispatcher has no subscribers")

}

func TestDispatchDelay(t *testing.T) {
	h := NewHistory(historySize)
	d := NewDispatcher(delay, h)

	s := d.Subscribe("192.168.1.3", contentTypeFilter, false)

	go d.Start()
	defer d.Stop()

	start := time.Now()
	go d.Send(n1)

	actualN1StdMsg := <-s.Notifications()

	stop := time.Now()

	actualDelay := stop.Sub(start)

	verifyNotificationResponse(t, n1, zeroTime, zeroTime, actualN1StdMsg)
	assert.InEpsilon(t, delay.Nanoseconds(), actualDelay.Nanoseconds(), 0.05, "The delay is correct with 0.05 relative error")
}

func TestDispatchedNotificationsInHistory(t *testing.T) {
	h := NewHistory(historySize)
	d := NewDispatcher(delay, h)

	go d.Start()
	defer d.Stop()

	notBefore := time.Now()

	d.Send(n1)
	d.Send(n2)
	d.Send(annNotif)
	time.Sleep(time.Duration(delay.Seconds()+1) * time.Second)

	notAfter := time.Now()
	verifyNotification(t, annNotif, notBefore, notAfter, h.Notifications()[2])
	verifyNotification(t, n1, notBefore, notAfter, h.Notifications()[1])
	verifyNotification(t, n2, notBefore, notAfter, h.Notifications()[0])
	assert.Len(t, h.Notifications(), 3, "History contains 3 notifications")

	for i := 0; i < historySize; i++ {
		d.Send(n2)
	}
	time.Sleep(time.Duration(delay.Seconds()+1) * time.Second)

	assert.Len(t, h.Notifications(), historySize, "History contains 10 notifications")
	assert.NotContains(t, h.Notifications(), n1, "History does not contain old notification")
}

func TestInternalFailToSendNotifications(t *testing.T) {
	hook := logTest.NewTestHook("notifications-push")
	defer hook.Reset()

	h := NewHistory(historySize)
	d := NewDispatcher(0, h)

	s1 := &MockSubscriber{}
	s2 := &MockSubscriber{}
	s3 := &MockSubscriber{}

	go d.Start()
	defer d.Stop()

	d.addSubscriber(s1)
	d.addSubscriber(s2)
	d.addSubscriber(s3)

	d.Send(n1)

	time.Sleep(time.Second)
	logger.Info("This log message is here to avoid a race condition")

	foundLog := false
	logOccurrence := 0
	for _, e := range hook.AllEntries() {
		switch e.Message {
		case "Processed subscribers. Failed to send notifications":
			assert.Equal(t, 0, e.Data["sent"], "sent")
			assert.Equal(t, 3, e.Data["failed"], "failed")
			assert.Equal(t, 0, e.Data["skipped"], "skipped")
			logOccurrence++
			foundLog = true
		default:
		}
	}
	assert.True(t, foundLog)
	assert.Equal(t, 1, logOccurrence)
}

func verifyNotificationResponse(t *testing.T, expected Notification, notBefore time.Time, notAfter time.Time, actualMsg string) {
	actualNotifications := []Notification{}
	_ = json.Unmarshal([]byte(actualMsg), &actualNotifications)
	require.True(t, len(actualNotifications) > 0)
	actual := actualNotifications[0]

	verifyNotification(t, expected, notBefore, notAfter, actual)
}

func verifyNotification(t *testing.T, expected Notification, notBefore time.Time, notAfter time.Time, actual Notification) {
	assert.Equal(t, expected.ID, actual.ID, "ID")
	assert.Equal(t, expected.Type, actual.Type, "Type")
	assert.Equal(t, expected.APIURL, actual.APIURL, "APIURL")

	if actual.LastModified != "" {
		assert.Equal(t, expected.LastModified, actual.LastModified, "LastModified")
		assert.Equal(t, expected.PublishReference, actual.PublishReference, "PublishReference")

		actualDate, _ := time.Parse(RFC3339Millis, actual.NotificationDate)
		assert.False(t, actualDate.Before(notBefore), "notificationDate is too early")
		assert.False(t, actualDate.After(notAfter), "notificationDate is too late")
	}
}

// MockSubscriber is an autogenerated mock type for the MockSubscriber type
type MockSubscriber struct {
	// _dummy property exists to prevent the compiler to apply empty struct optimizations on MockSubscriber
	// Notifications dispatcher stores subscribers as a set and expects new subscriber objects to be unique.
	// But for empty structs go compiler could decide to allocate memory for a single object
	// and just reference that memory when creating new objects of the same type.
	_dummy int //nolint:unused,structcheck
}

// AcceptedSubType provides a mock function with given fields:
func (_m *MockSubscriber) SubType() string {
	return "ContentPackage"
}

// Address provides a mock function with given fields:
func (_m *MockSubscriber) Address() string {
	return "192.168.1.1"
}

// send provides a mock function with given fields: n
func (_m *MockSubscriber) Send(n Notification) error {
	return errors.New("error")
}

// Id provides a mock function with given fields:
func (_m *MockSubscriber) ID() string {
	return "id"
}

// NotificationChannel provides a mock function with given fields:
func (_m *MockSubscriber) Notifications() <-chan string {
	return make(chan string, 16)
}

// Since provides a mock function with given fields:
func (_m *MockSubscriber) Since() time.Time {
	return time.Now()
}
