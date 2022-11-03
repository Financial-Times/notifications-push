package consumer

import (
	"bytes"
	"regexp"
	"testing"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/kafka-client-go/v3"
	"github.com/Financial-Times/notifications-push/v5/mocks"
	hooks "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var defaultContentURIAllowlist = regexp.MustCompile(`^http://.*-transformer-(pr|iw)-uk-.*\.svc\.ft\.com(:\d{2,5})?/(lists)/[\w-]+.*$`)
var sparkIncludedAllowlist = regexp.MustCompile(`^http://(methode|wordpress|content|upp)-(article|collection|content-placeholder|content)-(mapper|unfolder|validator)(-pr|-iw)?(-uk-.*)?\.svc\.ft\.com(:\d{2,5})?/(content|complementarycontent)/[\w-]+.*$`)

func TestSyntheticMessage(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL:     "test.api.ft.com",
		APIUrlResource: "lists",
	}
	var buf bytes.Buffer
	l := logger.NewUnstructuredLogger()
	l.SetOutput(&buf)

	dispatcher := &mocks.Dispatcher{}
	handler := NewQueueHandler(defaultContentURIAllowlist, NewSet(), nil, false, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "SYNTH_tid"},
		`{"UUID": "a uuid", "ContentURI": "http://list-transformer-pr-uk-up.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	handler.HandleMessage(msg)
	assert.NotContains(t, buf.String(), "error")

	dispatcher.AssertNotCalled(t, "Send")
}

func TestFailedCMSMessageParse(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL:     "test.api.ft.com",
		APIUrlResource: "lists",
	}
	var buf bytes.Buffer
	l := logger.NewUnstructuredLogger()
	l.SetOutput(&buf)

	dispatcher := &mocks.Dispatcher{}
	handler := NewQueueHandler(defaultContentURIAllowlist, NewSet(), nil, false, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin"}, "")

	handler.HandleMessage(msg)
	assert.Contains(t, buf.String(), "unexpected end of JSON input")

	dispatcher.AssertNotCalled(t, "Send")
}

func TestAllowlist(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL:     "test.api.ft.com",
		APIUrlResource: "lists",
	}
	var buf bytes.Buffer
	l := logger.NewUnstructuredLogger()
	l.SetOutput(&buf)

	dispatcher := &mocks.Dispatcher{}
	handler := NewQueueHandler(defaultContentURIAllowlist, NewSet(), nil, false, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin"},
		`{"ContentURI": "something which wouldn't match"}`)

	handler.HandleMessage(msg)
	assert.NotContains(t, buf.String(), "error")

	dispatcher.AssertNotCalled(t, "Send")
}

func TestSparkCCTAllowlist(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL:      "test.api.ft.com",
		UpdateEventType: "http://www.ft.com/thing/ThingChangeType/UPDATE",
		IncludeScoop:    true,
		APIUrlResource:  "content",
	}
	var buf bytes.Buffer
	l := logger.NewUnstructuredLogger()
	l.SetOutput(&buf)

	dispatcher := &mocks.Dispatcher{}
	dispatcher.On("Send", mock.AnythingOfType("dispatch.NotificationModel")).Return()

	contentTypeAllowlist := NewSet()
	contentTypeAllowlist.Add("application/vnd.ft-upp-article+json")
	handler := NewQueueHandler(sparkIncludedAllowlist, contentTypeAllowlist, nil, false, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin", "Content-Type": "application/vnd.ft-upp-article+json"},
		`{"payload": { "foo": "bar" }, "ContentURI": "http://upp-content-validator.svc.ft.com/content/f601289e-93a0-4c08-854e-fef334584079"}`)

	handler.HandleMessage(msg)
	assert.NotContains(t, buf.String(), "error")

	dispatcher.AssertExpectations(t)
}

func TestMonitoringEvents(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL:      "test.api.ft.com",
		APIUrlResource:  "content",
		UpdateEventType: "http://www.ft.com/thing/ThingChangeType/UPDATE",
	}
	var buf bytes.Buffer
	l := logger.NewUnstructuredLogger()
	l.SetOutput(&buf)

	h := hooks.NewLocal(l.Logger)
	defer h.Reset()

	dispatcher := &mocks.Dispatcher{}
	dispatcher.On("Send", mock.AnythingOfType("dispatch.NotificationModel")).Return()

	typeAllowlist := NewSet()
	typeAllowlist.Add("valid-type")
	handler := NewQueueHandler(sparkIncludedAllowlist, typeAllowlist, nil, true, mapper, dispatcher, l)
	tests := map[string]struct {
		Headers     map[string]string
		Body        string
		ExpectError bool
		Message     string
	}{
		"fail to parse message body": {
			Headers:     map[string]string{"X-Request-Id": "tid_test", "Content-Type": "valid-type"},
			Body:        ``,
			ExpectError: true,
			Message:     "Failed to unmarshall kafka message",
		},
		"skip carousel message": {
			Headers:     map[string]string{"X-Request-Id": "tid_carousel_1234567890"},
			Body:        `{"UUID": "a uuid", "ContentURI": "http://upp-article-mapper.svc.ft.com:8080/content/uuid"}`,
			ExpectError: false,
			Message:     "Skipping event: Carousel publish event.",
		},
		"skip synthetic message": {
			Headers:     map[string]string{"X-Request-Id": "SYNTH_test"},
			Body:        `{"UUID": "a uuid", "ContentURI": "http://upp-article-mapper.svc.ft.com:8080/content/uuid"}`,
			ExpectError: false,
			Message:     "Skipping event: Synthetic transaction ID.",
		},
		"skip not allowlisted content type messages": {
			Headers:     map[string]string{"X-Request-Id": "tid_test", "Content-Type": "invalid-type"},
			Body:        `{"UUID": "a uuid", "ContentURI": "http://upp-article-mapper.svc.ft.com:8080/content/uuid"}`,
			ExpectError: false,
			Message:     "Skipping event: contentType is not in the allowlist.",
		},
		"skip not allowlisted content uri messages": {
			Headers:     map[string]string{"X-Request-Id": "tid_test", "Content-Type": "application/json"},
			Body:        `{"UUID": "a uuid", "ContentURI": "http://not-in-the-allowlist"}`,
			ExpectError: false,
			Message:     "Skipping event: contentUri is not in the allowlist.",
		},
		"fail to map event to notification": {
			Headers:     map[string]string{"X-Request-Id": "tid_test", "Content-Type": "application/json"},
			Body:        `{"UUID": "a uuid", "ContentURI": "http://upp-article-mapper.svc.ft.com:8080/content/uuid"}`,
			ExpectError: true,
			Message:     "Skipping event: Cannot build notification for message.",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			msg := kafka.NewFTMessage(test.Headers, test.Body)
			handler.HandleMessage(msg)

			bufferedMsg := buf.String()
			buf.Reset()
			if test.ExpectError {
				assert.Contains(t, bufferedMsg, "error")
			} else {
				assert.NotContains(t, bufferedMsg, "error")
			}

			entry := h.LastEntry()
			val, has := entry.Data[logger.DefaultKeyMonitoringEvent]
			assert.True(t, has, "expect log to have monitor field")
			asd, _ := entry.String()
			assert.Equal(t, "true", val, "expect monitor field to be set to true, %s", asd)
			assert.Equal(t, test.Message, entry.Message)
		})
	}
}

func TestAcceptNotificationBasedOnContentType(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL:     "test.api.ft.com",
		APIUrlResource: "content",
	}
	contentTypeAllowlist := NewSet()
	contentTypeAllowlist.Add("application/vnd.ft-upp-article+json")

	var buf bytes.Buffer
	l := logger.NewUnstructuredLogger()
	l.SetOutput(&buf)

	dispatcher := &mocks.Dispatcher{}
	dispatcher.On("Send", mock.AnythingOfType("dispatch.NotificationModel")).Return()

	handler := NewQueueHandler(defaultContentURIAllowlist, contentTypeAllowlist, nil, false, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin", "Content-Type": "application/vnd.ft-upp-article+json"},
		`{"payload": { }, "ContentURI": "http://not-in-the-allowlist.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	handler.HandleMessage(msg)
	assert.NotContains(t, buf.String(), "error")
	dispatcher.AssertExpectations(t)
}

func TestAcceptNotificationBasedOnAudioContentType(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL:     "test.api.ft.com",
		APIUrlResource: "content",
	}

	var buf bytes.Buffer
	l := logger.NewUnstructuredLogger()
	l.SetOutput(&buf)

	contentTypeAllowlist := NewSet()
	contentTypeAllowlist.Add("application/vnd.ft-upp-audio+json")

	dispatcher := &mocks.Dispatcher{}
	dispatcher.On("Send", mock.AnythingOfType("dispatch.NotificationModel")).Return()

	handler := NewQueueHandler(defaultContentURIAllowlist, contentTypeAllowlist, nil, false, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin", "Content-Type": "application/vnd.ft-upp-audio+json"},
		`{"payload": { }, "ContentURI": "http://not-in-the-allowlist.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	handler.HandleMessage(msg)
	assert.NotContains(t, buf.String(), "error")
	dispatcher.AssertExpectations(t)
}

func TestDiscardNotificationBasedOnContentType(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL:     "test.api.ft.com",
		APIUrlResource: "content",
	}

	var buf bytes.Buffer
	l := logger.NewUnstructuredLogger()
	l.SetOutput(&buf)

	contentTypeAllowlist := NewSet()
	contentTypeAllowlist.Add("application/vnd.ft-upp-article+json")

	dispatcher := &mocks.Dispatcher{}
	dispatcher.On("Send", mock.AnythingOfType("dispatch.NotificationModel")).Return()

	handler := NewQueueHandler(sparkIncludedAllowlist, contentTypeAllowlist, nil, false, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin", "Content-Type": "application/vnd.ft-upp-invalid+json"},
		`{"payload": { }, "ContentURI": "http://methode-article-mapper.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	handler.HandleMessage(msg)
	assert.NotContains(t, buf.String(), "error")

	dispatcher.AssertNotCalled(t, "Send")
}

func TestAcceptNotificationBasedOnContentUriWhenContentTypeIsApplicationJson(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL:     "test.api.ft.com",
		APIUrlResource: "content",
	}

	var buf bytes.Buffer
	l := logger.NewUnstructuredLogger()
	l.SetOutput(&buf)

	contentTypeAllowlist := NewSet()
	contentTypeAllowlist.Add("application/vnd.ft-upp-article+json")

	dispatcher := &mocks.Dispatcher{}
	dispatcher.On("Send", mock.AnythingOfType("dispatch.NotificationModel")).Return()

	handler := NewQueueHandler(sparkIncludedAllowlist, contentTypeAllowlist, nil, false, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin", "Content-Type": "application/json"},
		`{"payload": { }, "ContentURI": "http://methode-article-mapper.svc.ft.com:8080/content/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	handler.HandleMessage(msg)
	assert.NotContains(t, buf.String(), "error")
	dispatcher.AssertExpectations(t)
}

func TestDiscardNotificationBasedOnContentUriWhenContentTypeIsApplicationJson(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL:     "test.api.ft.com",
		APIUrlResource: "content",
	}
	var buf bytes.Buffer
	l := logger.NewUnstructuredLogger()
	l.SetOutput(&buf)

	contentTypeAllowlist := NewSet()
	contentTypeAllowlist.Add("application/vnd.ft-upp-article+json")

	dispatcher := &mocks.Dispatcher{}
	dispatcher.On("Send", mock.AnythingOfType("dispatch.NotificationModel")).Return()

	handler := NewQueueHandler(sparkIncludedAllowlist, contentTypeAllowlist, nil, false, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin", "Content-Type": "application/json"},
		`{"payload": { }, "ContentURI": "http://not-in-the-allowlist.svc.ft.com:8080/content/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	handler.HandleMessage(msg)
	assert.NotContains(t, buf.String(), "error")

	dispatcher.AssertNotCalled(t, "Send")
}

func TestAcceptNotificationBasedOnContentUriWhenContentTypeIsMissing(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL:     "test.api.ft.com",
		APIUrlResource: "content",
	}
	var buf bytes.Buffer
	l := logger.NewUnstructuredLogger()
	l.SetOutput(&buf)

	contentTypeAllowlist := NewSet()
	contentTypeAllowlist.Add("application/vnd.ft-upp-article+json")

	dispatcher := &mocks.Dispatcher{}
	dispatcher.On("Send", mock.AnythingOfType("dispatch.NotificationModel")).Return()

	handler := NewQueueHandler(sparkIncludedAllowlist, contentTypeAllowlist, nil, false, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin", "Content-Type": "application/vnd.ft-upp-article+json"},
		`{"payload": { }, "ContentURI": "http://methode-article-mapper.svc.ft.com:8080/content/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	handler.HandleMessage(msg)
	assert.NotContains(t, buf.String(), "error")
	dispatcher.AssertExpectations(t)
}

func TestDiscardNotificationBasedOnContentUriWhenContentTypeIsMissing(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL:     "test.api.ft.com",
		APIUrlResource: "content",
	}
	var buf bytes.Buffer
	l := logger.NewUnstructuredLogger()
	l.SetOutput(&buf)

	contentTypeAllowlist := NewSet()
	contentTypeAllowlist.Add("application/vnd.ft-upp-article+json")

	dispatcher := &mocks.Dispatcher{}
	dispatcher.On("Send", mock.AnythingOfType("dispatch.NotificationModel")).Return()

	handler := NewQueueHandler(sparkIncludedAllowlist, contentTypeAllowlist, nil, false, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin"},
		`{"payload": { }, "ContentURI": "http://not-in-the-allowlist.svc.ft.com:8080/content/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	handler.HandleMessage(msg)
	assert.NotContains(t, buf.String(), "error")

	dispatcher.AssertNotCalled(t, "Send")
}

func TestFailsConversionToNotification(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL:     "test.api.ft.com",
		APIUrlResource: "list",
	}
	var buf bytes.Buffer
	l := logger.NewUnstructuredLogger()
	l.SetOutput(&buf)

	dispatcher := &mocks.Dispatcher{}

	handler := NewQueueHandler(defaultContentURIAllowlist, NewSet(), nil, false, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin"},
		`{"payload": { }, "ContentURI": "http://list-transformer-pr-uk-up.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a" + }`)

	handler.HandleMessage(msg)
	assert.Contains(t, buf.String(), "invalid character '+'")

	dispatcher.AssertNotCalled(t, "Send")
}

func TestHandleMessage(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL:      "test.api.ft.com",
		UpdateEventType: "http://www.ft.com/thing/ThingChangeType/UPDATE",
		IncludeScoop:    true,
		APIUrlResource:  "content",
	}

	var buf bytes.Buffer
	l := logger.NewUnstructuredLogger()
	l.SetOutput(&buf)

	dispatcher := &mocks.Dispatcher{}
	dispatcher.On("Send", mock.AnythingOfType("dispatch.NotificationModel")).Return()

	contentTypeAllowlist := NewSet()
	contentTypeAllowlist.Add("application/vnd.ft-upp-article+json")
	handler := NewQueueHandler(defaultContentURIAllowlist, contentTypeAllowlist, nil, false, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin", "Content-Type": "application/vnd.ft-upp-article+json"},
		`{"UUID": "55e40823-6804-4264-ac2f-b29e11bf756a", "payload": { "foo": "bar" }, "ContentURI": "http://list-transformer-pr-uk-up.svc.ft.com:8080/lists/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	handler.HandleMessage(msg)
	assert.NotContains(t, buf.String(), "error")

	dispatcher.AssertExpectations(t)
}

func TestHandleMessageMappingError(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL:     "test.api.ft.com",
		APIUrlResource: "lists",
	}
	var buf bytes.Buffer
	l := logger.NewUnstructuredLogger()
	l.SetOutput(&buf)

	contentTypeAllowlist := NewSet()
	contentTypeAllowlist.Add("application/vnd.ft-upp-article+json")

	dispatcher := &mocks.Dispatcher{}
	handler := NewQueueHandler(defaultContentURIAllowlist, contentTypeAllowlist, nil, false, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin", "Content-Type": "application/vnd.ft-upp-article+json"},
		`{"UUID": "", "payload": { }, "ContentURI": "http://list-transformer-pr-uk-up.svc.ft.com:8080/lists/blah/abc"}`)

	handler.HandleMessage(msg)
	assert.Contains(t, buf.String(), "does not contain")

	dispatcher.AssertNotCalled(t, "Send")
}

func TestDiscardStandardCarouselPublicationEvents(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL:     "test.api.ft.com",
		APIUrlResource: "lists",
	}
	l := logger.NewUnstructuredLogger()

	dispatcher := &mocks.Dispatcher{}
	handler := NewQueueHandler(defaultContentURIAllowlist, NewSet(), nil, false, mapper, dispatcher, l)

	msg1 := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_fzy2uqund8_carousel_1485954245"},
		`{"UUID": "a uuid", "ContentURI": "http://list-transformer-pr-uk-up.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	msg2 := kafka.NewFTMessage(map[string]string{"X-Request-Id": "republish_-10bd337c-66d4-48d9-ab8a-e8441fa2ec98_carousel_1493606135"},
		`{"UUID": "a uuid", "ContentURI": "http://list-transformer-pr-uk-up.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	msg3 := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_ofcysuifp0_carousel_1488384556_gentx"},
		`{"UUID": "a uuid", "ContentURI": "http://list-transformer-pr-uk-up.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a"}`,
	)
	handler.HandleMessage(msg1)
	handler.HandleMessage(msg2)
	handler.HandleMessage(msg3)

	dispatcher.AssertNotCalled(t, "Send")
}

func TestDiscardCarouselPublicationEventsWithGeneratedTransactionID(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL:     "test.api.ft.com",
		APIUrlResource: "lists",
	}
	var buf bytes.Buffer
	l := logger.NewUnstructuredLogger()
	l.SetOutput(&buf)

	dispatcher := &mocks.Dispatcher{}
	handler := NewQueueHandler(defaultContentURIAllowlist, NewSet(), nil, false, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_fzy2uqund8_carousel_1485954245_gentx"},
		`{"UUID": "a uuid", "ContentURI": "http://list-transformer-pr-uk-up.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a"}`,
	)

	handler.HandleMessage(msg)
	assert.NotContains(t, buf.String(), "error")

	dispatcher.AssertNotCalled(t, "Send")
}

func TestAcceptNotificationBasedOnE2ETransactionID(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL:     "test.api.ft.com",
		APIUrlResource: "content",
	}
	e2eTestUUIDs := []string{"e4d2885f-1140-400b-9407-921e1c7378cd"}

	var buf bytes.Buffer
	l := logger.NewUnstructuredLogger()
	l.SetOutput(&buf)

	dispatcher := &mocks.Dispatcher{}
	dispatcher.On("Send", mock.AnythingOfType("dispatch.NotificationModel")).Return()

	handler := NewQueueHandler(nil, nil, e2eTestUUIDs, false, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(
		map[string]string{"X-Request-Id": "SYNTHETIC-REQ-MONe4d2885f-1140-400b-9407-921e1c7378cd"},
		`{"payload": { }, "contentURI": "http://upp-content-validator.svc.ft.com/content/e4d2885f-1140-400b-9407-921e1c7378cd"}`,
	)

	handler.HandleMessage(msg)
	assert.NotContains(t, buf.String(), "error")
	dispatcher.AssertExpectations(t)
}
