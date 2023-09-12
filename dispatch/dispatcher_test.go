package dispatch

import (
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/Financial-Times/notifications-push/v5/access"
	hooks "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Financial-Times/go-logger/v2"
)

const (
	typeArticle       = "Article"
	annotationSubType = "Annotations"
	opaFile           = "../opa_modules/central_banking.rego"
)

var contentSubscribeTypes = []string{"Article", "ContentPackage", "Audio"}

var delay = 2 * time.Second
var historySize = 10

var n1 = NotificationModel{
	APIURL:           "http://api.ft.com/content/7998974a-1e97-11e6-b286-cddde55ca122",
	ID:               "http://www.ft.com/thing/7998974a-1e97-11e6-b286-cddde55ca122",
	Type:             "http://www.ft.com/thing/ThingChangeType/UPDATE",
	PublishReference: "tid_test1",
	LastModified:     "2016-11-02T10:54:22.234Z",
	SubscriptionType: "ContentPackage",
}

var n2 = NotificationModel{
	APIURL:           "http://api.ft.com/content/7998974a-1e97-11e6-b286-cddde55ca122",
	ID:               "http://www.ft.com/thing/7998974a-1e97-11e6-b286-cddde55ca122",
	Type:             "http://www.ft.com/thing/ThingChangeType/DELETE",
	PublishReference: "tid_test2",
	LastModified:     "2016-11-02T10:55:24.244Z",
	SubscriptionType: "Article",
}

var annNotif = NotificationModel{
	APIURL:           "http://api.ft.com/content/7998974a-1e97-11e6-b286-cddde55ca122",
	ID:               "http://www.ft.com/thing/7998974a-1e97-11e6-b286-cddde55ca122",
	Type:             "http://www.ft.com/thing/ThingChangeType/ANNOTATIONS_UPDATE",
	PublishReference: "tid_test3",
	SubscriptionType: "Annotations",
}

var e2eTestNotification = NotificationModel{
	APIURL:           "http://api.ft.com/content/e4d2885f-1140-400b-9407-921e1c7378cd",
	ID:               "http://www.ft.com/thing/e4d2885f-1140-400b-9407-921e1c7378cd",
	Type:             "http://www.ft.com/thing/ThingChangeType/UPDATE",
	PublishReference: "SYNTHETIC-REQ-MONe4d2885f-1140-400b-9407-921e1c7378cd",
	LastModified:     "2016-11-02T10:54:22.234Z",
	IsE2ETest:        true,
}

var createNotification = NotificationModel{
	APIURL:           "http://api.ft.com/content/e4d2885f-1140-400b-9407-921e1c7378cd",
	ID:               "http://www.ft.com/thing/e4d2885f-1140-400b-9407-921e1c7378cd",
	Type:             "http://www.ft.com/thing/ThingChangeType/CREATE",
	PublishReference: "SYNTHETIC-REQ-MONe4d2885f-1140-400b-9407-921e1c7378cd",
	LastModified:     "2016-11-02T10:54:22.234Z",
	SubscriptionType: "Article",
}

// Subscribers who did not explicitly opt in for Create notifications will actually get Update notification
var modifiedCreateNotification = NotificationModel{
	APIURL:           "http://api.ft.com/content/e4d2885f-1140-400b-9407-921e1c7378cd",
	ID:               "http://www.ft.com/thing/e4d2885f-1140-400b-9407-921e1c7378cd",
	Type:             "http://www.ft.com/thing/ThingChangeType/UPDATE",
	PublishReference: "SYNTHETIC-REQ-MONe4d2885f-1140-400b-9407-921e1c7378cd",
	LastModified:     "2016-11-02T10:54:22.234Z",
	SubscriptionType: "Article",
}

var zeroTime = time.Time{}

func TestShouldDispatchNotificationsToMultipleSubscribers(t *testing.T) {
	t.Parallel()

	l := logger.NewUPPLogger("test", "panic")
	h := NewHistory(historySize)
	e, err := access.CreateEvaluator(
		"data.centralBanking.allow",
		[]string{opaFile},
	)
	assert.NoError(t, err)
	d := NewDispatcher(delay, h, e, l)

	m, _ := d.Subscribe("192.168.1.2", contentSubscribeTypes, true, &access.NotificationSubscriptionOptions{
		ReceiveAdvancedNotifications: false,
	})
	s, _ := d.Subscribe("192.168.1.3", contentSubscribeTypes, false, &access.NotificationSubscriptionOptions{
		ReceiveAdvancedNotifications: false,
	})

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
	t.Parallel()

	l := logger.NewUPPLogger("test", "panic")
	l.Out = io.Discard
	hook := hooks.NewLocal(l.Logger)
	defer hook.Reset()

	h := NewHistory(historySize)

	e, err := access.CreateEvaluator(
		"data.centralBanking.allow",
		[]string{opaFile},
	)
	assert.NoError(t, err)
	d := NewDispatcher(delay, h, e, l)

	m, _ := d.Subscribe("192.168.1.2", contentSubscribeTypes, true, &access.NotificationSubscriptionOptions{
		ReceiveAdvancedNotifications: false,
	})
	s, _ := d.Subscribe("192.168.1.3", []string{typeArticle}, false, &access.NotificationSubscriptionOptions{
		ReceiveAdvancedNotifications: false,
	})
	annSub, _ := d.Subscribe("192.168.1.4", []string{annotationSubType}, false, &access.NotificationSubscriptionOptions{
		ReceiveAdvancedNotifications: false,
	})

	go d.Start()
	defer d.Stop()

	notBefore := time.Now()
	d.Send(n1)
	// sleep for ensuring that notifications come in the order they are send.
	<-time.After(time.Millisecond * 1000)
	d.Send(n2)
	<-time.After(time.Millisecond * 1000)
	d.Send(annNotif)

	actualN2StdMsg := <-s.Notifications()
	verifyNotificationResponse(t, n2, zeroTime, zeroTime, actualN2StdMsg)

	// stops exec here ...
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

func TestShouldDispatchE2ETestNotificationsToMonitoringSubscribersOnly(t *testing.T) {
	t.Parallel()

	l := logger.NewUPPLogger("test", "panic")
	h := NewHistory(historySize)
	e, err := access.CreateEvaluator(
		"data.centralBanking.allow",
		[]string{opaFile},
	)
	assert.NoError(t, err)
	d := NewDispatcher(time.Millisecond, h, e, l)

	m, _ := d.Subscribe("192.168.1.2", contentSubscribeTypes, true, &access.NotificationSubscriptionOptions{
		ReceiveAdvancedNotifications: false,
	})
	s, _ := d.Subscribe("192.168.1.3", contentSubscribeTypes, false, &access.NotificationSubscriptionOptions{
		ReceiveAdvancedNotifications: false,
	})

	go d.Start()
	defer d.Stop()

	notBefore := time.Now()
	d.Send(e2eTestNotification)

	monitorMsg, err := waitForNotification(m.Notifications(), 10*time.Millisecond)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	verifyNotificationResponse(t, e2eTestNotification, notBefore, time.Now(), monitorMsg)

	_, err = waitForNotification(s.Notifications(), 10*time.Millisecond)
	if err == nil {
		t.Fatal("expected non nil error")
	}
}

func TestCreateNotificationIsProperlyDispatchedToSubscribers(t *testing.T) {
	t.Parallel()

	l := logger.NewUPPLogger("test", "panic")
	h := NewHistory(historySize)
	e, err := access.CreateEvaluator(
		"data.centralBanking.allow",
		[]string{opaFile},
	)
	assert.NoError(t, err)
	d := NewDispatcher(time.Millisecond, h, e, l)

	m1, _ := d.Subscribe("192.168.1.2", contentSubscribeTypes, true, &access.NotificationSubscriptionOptions{
		ReceiveAdvancedNotifications: true,
	})
	s1, _ := d.Subscribe("192.168.1.3", contentSubscribeTypes, false, &access.NotificationSubscriptionOptions{
		ReceiveAdvancedNotifications: true,
	})
	m2, _ := d.Subscribe("192.168.1.4", contentSubscribeTypes, true, &access.NotificationSubscriptionOptions{
		ReceiveAdvancedNotifications: false,
	})
	s2, _ := d.Subscribe("192.168.1.5", contentSubscribeTypes, false, &access.NotificationSubscriptionOptions{
		ReceiveAdvancedNotifications: false,
	})

	go d.Start()
	defer d.Stop()

	notBefore := time.Now()
	d.Send(createNotification)

	monitorMsg, err := waitForNotification(m1.Notifications(), 10*time.Millisecond)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	verifyNotificationResponse(t, createNotification, notBefore, time.Now(), monitorMsg)

	actualMsg, err := waitForNotification(s1.Notifications(), 10*time.Millisecond)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	verifyNotificationResponse(t, createNotification, notBefore, time.Now(), actualMsg)

	monitorMsg, err = waitForNotification(m2.Notifications(), 10*time.Millisecond)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	verifyNotificationResponse(t, modifiedCreateNotification, notBefore, time.Now(), monitorMsg)

	actualMsg, err = waitForNotification(s2.Notifications(), 10*time.Millisecond)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	verifyNotificationResponse(t, modifiedCreateNotification, notBefore, time.Now(), actualMsg)
}

func TestAddAndRemoveOfSubscribers(t *testing.T) {
	t.Parallel()

	l := logger.NewUPPLogger("test", "panic")
	h := NewHistory(historySize)

	e, err := access.CreateEvaluator(
		"data.centralBanking.allow",
		[]string{opaFile},
	)
	assert.NoError(t, err)
	d := NewDispatcher(delay, h, e, l)

	m, _ := d.Subscribe("192.168.1.2", contentSubscribeTypes, true, &access.NotificationSubscriptionOptions{
		ReceiveAdvancedNotifications: false,
	})
	m = m.(NotificationConsumer)
	s, _ := d.Subscribe("192.168.1.3", contentSubscribeTypes, false, &access.NotificationSubscriptionOptions{
		ReceiveAdvancedNotifications: false,
	})
	s = s.(NotificationConsumer)

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
	t.Parallel()

	l := logger.NewUPPLogger("test", "panic")
	h := NewHistory(historySize)

	e, err := access.CreateEvaluator(
		"data.centralBanking.allow",
		[]string{opaFile},
	)
	assert.NoError(t, err)
	d := NewDispatcher(delay, h, e, l)

	s, _ := d.Subscribe("192.168.1.3", contentSubscribeTypes, false, &access.NotificationSubscriptionOptions{
		ReceiveAdvancedNotifications: false,
	})

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
	t.Parallel()

	l := logger.NewUPPLogger("test", "panic")
	h := NewHistory(historySize)
	e, err := access.CreateEvaluator(
		"data.centralBanking.allow",
		[]string{opaFile},
	)
	assert.NoError(t, err)
	d := NewDispatcher(delay, h, e, l)

	go d.Start()
	defer d.Stop()

	notBefore := time.Now()

	d.Send(n1)
	d.Send(n2)
	d.Send(annNotif)
	time.Sleep(time.Duration(delay.Seconds()+1) * time.Second)

	notAfter := time.Now()
	verifyNotificationModel(t, annNotif, notBefore, notAfter, h.Notifications()[2])
	verifyNotificationModel(t, n1, notBefore, notAfter, h.Notifications()[1])
	verifyNotificationModel(t, n2, notBefore, notAfter, h.Notifications()[0])
	assert.Len(t, h.Notifications(), 3, "History contains 3 notifications")

	for i := 0; i < historySize; i++ {
		d.Send(n2)
	}
	time.Sleep(time.Duration(delay.Seconds()+1) * time.Second)

	assert.Len(t, h.Notifications(), historySize, "History contains 10 notifications")
	assert.NotContains(t, h.Notifications(), n1, "History does not contain old notification")
}

func TestInternalFailToSendNotifications(t *testing.T) {
	t.Parallel()

	l := logger.NewUPPLogger("test", "info")
	l.Out = io.Discard
	hook := hooks.NewLocal(l.Logger)
	defer hook.Reset()

	h := NewHistory(historySize)
	e, err := access.CreateEvaluator(
		"data.centralBanking.allow",
		[]string{opaFile},
	)
	assert.NoError(t, err)
	d := NewDispatcher(0, h, e, l)

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

func verifyNotificationResponse(t *testing.T, expected NotificationModel, notBefore time.Time, notAfter time.Time, actualMsg string) {
	actualNotifications := []NotificationResponse{}
	_ = json.Unmarshal([]byte(actualMsg), &actualNotifications)
	require.True(t, len(actualNotifications) > 0)
	actual := actualNotifications[0]

	verifyNotification(t, expected, notBefore, notAfter, actual)
}

func verifyNotification(t *testing.T, expected NotificationModel, notBefore time.Time, notAfter time.Time, actual NotificationResponse) {
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

func verifyNotificationModel(t *testing.T, expected NotificationModel, notBefore time.Time, notAfter time.Time, actual NotificationModel) {
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

func TestMatchesSubType(t *testing.T) {
	tests := []struct {
		name string
		n    NotificationModel
		s    *StandardSubscriber
		res  bool
	}{
		{
			name: "test that notification type matches if subscriber has it as subscription type",
			n: NotificationModel{
				SubscriptionType: AudioContentType,
				Type:             AudioContentType,
			},
			s: &StandardSubscriber{
				acceptedTypes: []string{ArticleContentType, AudioContentType},
			},
			res: true,
		},
		{
			name: "test that notification type does not match if subscriber does not have it as subscription type",
			n: NotificationModel{
				SubscriptionType: AudioContentType,
				Type:             AudioContentType,
			},
			s: &StandardSubscriber{
				acceptedTypes: []string{ArticleContentType, ContentPackageType},
			},
			res: false,
		},
		{
			name: "test that if notification is of type DELETE and the content type can be resolved - we should match it only if the subscriber has been subscribed for this type",
			n: NotificationModel{
				SubscriptionType: AudioContentType,
				Type:             ContentDeleteType,
			},
			s: &StandardSubscriber{
				acceptedTypes: []string{ArticleContentType, AudioContentType},
			},
			res: true,
		},
		{
			name: "test if subscriber is not subscribed for annotations and notification is of type DELETE and its content type cannot be resolved - we should match it",
			n: NotificationModel{
				SubscriptionType: "",
				Type:             ContentDeleteType,
			},
			s: &StandardSubscriber{
				acceptedTypes: []string{ArticleContentType},
			},
			res: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res := matchesSubType(test.n, test.s)
			assert.Equal(t, test.res, res)
		})
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

func (_m *MockSubscriber) Options() *access.NotificationSubscriptionOptions {
	return &access.NotificationSubscriptionOptions{
		ReceiveAdvancedNotifications: false,
	}
}

// AcceptedSubType provides a mock function with given fields:
func (_m *MockSubscriber) SubTypes() []string {
	return []string{"ContentPackage"}
}

// Address provides a mock function with given fields:
func (_m *MockSubscriber) Address() string {
	return "192.168.1.1"
}

// send provides a mock function with given fields: n
func (_m *MockSubscriber) Send(_ NotificationResponse) error {
	return fmt.Errorf("error")
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

func waitForNotification(notificationsCh <-chan string, timeout time.Duration) (string, error) {
	ticker := time.NewTicker(timeout / 10)
	defer ticker.Stop()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case <-ticker.C:
			continue
		case n := <-notificationsCh:
			return n, nil
		case <-timer.C:
			return "", fmt.Errorf("test timed out waiting for notification")
		}
	}
}
