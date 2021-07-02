package consumer

import (
	"io/ioutil"
	"regexp"
	"testing"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/kafka-client-go/kafka"
	"github.com/Financial-Times/notifications-push/v5/mocks"
	hooks "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var defaultContentURIWhitelist = regexp.MustCompile(`^http://.*-transformer-(pr|iw)-uk-.*\.svc\.ft\.com(:\d{2,5})?/(lists)/[\w-]+.*$`)
var sparkIncludedWhiteList = regexp.MustCompile(`^http://(methode|wordpress|content|upp)-(article|collection|content-placeholder|content)-(mapper|unfolder|validator)(-pr|-iw)?(-uk-.*)?\.svc\.ft\.com(:\d{2,5})?/(content|complementarycontent)/[\w-]+.*$`)

func TestSyntheticMessage(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "lists",
	}
	l := logger.NewUPPLogger("test", "PANIC")
	dispatcher := &mocks.Dispatcher{}
	handler := NewContentQueueHandler(defaultContentURIWhitelist, NewSet(), nil, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "SYNTH_tid"},
		`{"UUID": "a uuid", "ContentURI": "http://list-transformer-pr-uk-up.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	assert.NoError(t, handler.HandleMessage(msg))

	dispatcher.AssertNotCalled(t, "Send")
}

func TestFailedCMSMessageParse(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "lists",
	}
	l := logger.NewUPPLogger("test", "PANIC")
	dispatcher := &mocks.Dispatcher{}
	handler := NewContentQueueHandler(defaultContentURIWhitelist, NewSet(), nil, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin"}, "")

	assert.Error(t, handler.HandleMessage(msg), "expect parse error")
	dispatcher.AssertNotCalled(t, "Send")
}

func TestWhitelist(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "lists",
	}
	l := logger.NewUPPLogger("test", "PANIC")
	dispatcher := &mocks.Dispatcher{}
	handler := NewContentQueueHandler(defaultContentURIWhitelist, NewSet(), nil, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin"},
		`{"ContentURI": "something which wouldn't match"}`)

	assert.NoError(t, handler.HandleMessage(msg))

	dispatcher.AssertNotCalled(t, "Send")
}

func TestSparkCCTWhitelist(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "content",
	}
	l := logger.NewUPPLogger("test", "PANIC")
	dispatcher := &mocks.Dispatcher{}
	dispatcher.On("Send", mock.AnythingOfType("dispatch.NotificationModel")).Return()

	handler := NewContentQueueHandler(sparkIncludedWhiteList, NewSet(), nil, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin"},
		`{"payload": { }, "contentURI": "http://upp-content-validator.svc.ft.com/content/f601289e-93a0-4c08-854e-fef334584079"}`)

	assert.NoError(t, handler.HandleMessage(msg))
	dispatcher.AssertExpectations(t)
}

func TestMonitoringEvents(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "content",
	}
	l := logger.NewUPPLogger("test", "info")
	l.Out = ioutil.Discard
	h := hooks.NewLocal(l.Logger)
	defer h.Reset()

	dispatcher := &mocks.Dispatcher{}
	dispatcher.On("Send", mock.AnythingOfType("dispatch.NotificationModel")).Return()

	typeWhitelist := NewSet()
	typeWhitelist.Add("valid-type")
	handler := NewContentQueueHandler(sparkIncludedWhiteList, typeWhitelist, nil, mapper, dispatcher, l)
	tests := map[string]struct {
		Headers     map[string]string
		Body        string
		ExpectError bool
		Message     string
	}{
		"fail to parse message body": {
			Headers:     map[string]string{"X-Request-Id": "tid_test"},
			Body:        "",
			ExpectError: true,
			Message:     "Skipping event.",
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
		"skip not whitelisted content type messages": {
			Headers:     map[string]string{"X-Request-Id": "tid_test", "Content-Type": "invalid-type"},
			Body:        `{"UUID": "a uuid", "ContentURI": "http://upp-article-mapper.svc.ft.com:8080/content/uuid"}`,
			ExpectError: false,
			Message:     "Skipping event: contentType is not the whitelist.",
		},
		"skip not whitelisted content uri messages": {
			Headers:     map[string]string{"X-Request-Id": "tid_test", "Content-Type": "application/json"},
			Body:        `{"UUID": "a uuid", "ContentURI": "http://not-in-the-whitelist"}`,
			ExpectError: false,
			Message:     "Skipping event: contentUri is not in the whitelist.",
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
			err := handler.HandleMessage(msg)
			if test.ExpectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			entry := h.LastEntry()
			val, has := entry.Data[logger.DefaultKeyMonitoringEvent]
			assert.True(t, has, "expect log to have monitor field")
			assert.Equal(t, "true", val, "expect monitor field to be set to true")
			assert.Equal(t, test.Message, entry.Message)
		})
	}
}

func TestAcceptNotificationBasedOnContentType(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "content",
	}
	contentTypeWhitelist := NewSet()
	contentTypeWhitelist.Add("application/vnd.ft-upp-article+json")
	l := logger.NewUPPLogger("test", "PANIC")
	dispatcher := &mocks.Dispatcher{}
	dispatcher.On("Send", mock.AnythingOfType("dispatch.NotificationModel")).Return()

	handler := NewContentQueueHandler(defaultContentURIWhitelist, contentTypeWhitelist, nil, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin", "Content-Type": "application/vnd.ft-upp-article+json; version=1.0; charset=utf-8"},
		`{"payload": { }, "ContentURI": "http://not-in-the-whitelist.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	assert.NoError(t, handler.HandleMessage(msg))
	dispatcher.AssertExpectations(t)
}

func TestAcceptNotificationBasedOnAudioContentType(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "content",
	}
	l := logger.NewUPPLogger("test", "PANIC")

	contentTypeWhitelist := NewSet()
	contentTypeWhitelist.Add("application/vnd.ft-upp-audio+json")

	dispatcher := &mocks.Dispatcher{}
	dispatcher.On("Send", mock.AnythingOfType("dispatch.NotificationModel")).Return()

	handler := NewContentQueueHandler(defaultContentURIWhitelist, contentTypeWhitelist, nil, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin", "Content-Type": "application/vnd.ft-upp-audio+json"},
		`{"payload": { }, "ContentURI": "http://not-in-the-whitelist.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	assert.NoError(t, handler.HandleMessage(msg))
	dispatcher.AssertExpectations(t)
}

func TestDiscardNotificationBasedOnContentType(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "content",
	}
	l := logger.NewUPPLogger("test", "PANIC")

	contentTypeWhitelist := NewSet()
	contentTypeWhitelist.Add("application/vnd.ft-upp-article+json")

	dispatcher := &mocks.Dispatcher{}
	dispatcher.On("Send", mock.AnythingOfType("dispatch.NotificationModel")).Return()

	handler := NewContentQueueHandler(sparkIncludedWhiteList, contentTypeWhitelist, nil, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin", "Content-Type": "application/vnd.ft-upp-invalid+json"},
		`{"payload": { }, "ContentURI": "http://methode-article-mapper.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	assert.NoError(t, handler.HandleMessage(msg))

	dispatcher.AssertNotCalled(t, "Send")
}

func TestAcceptNotificationBasedOnContentUriWhenContentTypeIsApplicationJson(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "content",
	}
	l := logger.NewUPPLogger("test", "PANIC")

	contentTypeWhitelist := NewSet()
	contentTypeWhitelist.Add("application/vnd.ft-upp-article+json")

	dispatcher := &mocks.Dispatcher{}
	dispatcher.On("Send", mock.AnythingOfType("dispatch.NotificationModel")).Return()

	handler := NewContentQueueHandler(sparkIncludedWhiteList, contentTypeWhitelist, nil, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin", "Content-Type": "application/json"},
		`{"payload": { }, "ContentURI": "http://methode-article-mapper.svc.ft.com:8080/content/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	assert.NoError(t, handler.HandleMessage(msg))
	dispatcher.AssertExpectations(t)
}

func TestDiscardNotificationBasedOnContentUriWhenContentTypeIsApplicationJson(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "content",
	}
	l := logger.NewUPPLogger("test", "PANIC")

	contentTypeWhitelist := NewSet()
	contentTypeWhitelist.Add("application/vnd.ft-upp-article+json")

	dispatcher := &mocks.Dispatcher{}
	dispatcher.On("Send", mock.AnythingOfType("dispatch.NotificationModel")).Return()

	handler := NewContentQueueHandler(sparkIncludedWhiteList, contentTypeWhitelist, nil, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin", "Content-Type": "application/json"},
		`{"payload": { }, "ContentURI": "http://not-in-the-whitelist.svc.ft.com:8080/content/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	assert.NoError(t, handler.HandleMessage(msg))

	dispatcher.AssertNotCalled(t, "Send")
}

func TestAcceptNotificationBasedOnContentUriWhenContentTypeIsMissing(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "content",
	}
	l := logger.NewUPPLogger("test", "PANIC")

	contentTypeWhitelist := NewSet()
	contentTypeWhitelist.Add("application/vnd.ft-upp-article+json")

	dispatcher := &mocks.Dispatcher{}
	dispatcher.On("Send", mock.AnythingOfType("dispatch.NotificationModel")).Return()

	handler := NewContentQueueHandler(sparkIncludedWhiteList, contentTypeWhitelist, nil, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin"},
		`{"payload": { }, "ContentURI": "http://methode-article-mapper.svc.ft.com:8080/content/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	assert.NoError(t, handler.HandleMessage(msg))
	dispatcher.AssertExpectations(t)
}

func TestDiscardNotificationBasedOnContentUriWhenContentTypeIsMissing(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "content",
	}
	l := logger.NewUPPLogger("test", "PANIC")
	contentTypeWhitelist := NewSet()
	contentTypeWhitelist.Add("application/vnd.ft-upp-article+json")

	dispatcher := &mocks.Dispatcher{}
	dispatcher.On("Send", mock.AnythingOfType("dispatch.NotificationModel")).Return()

	handler := NewContentQueueHandler(sparkIncludedWhiteList, contentTypeWhitelist, nil, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin"},
		`{"payload": { }, "ContentURI": "http://not-in-the-whitelist.svc.ft.com:8080/content/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	assert.NoError(t, handler.HandleMessage(msg))

	dispatcher.AssertNotCalled(t, "Send")
}

func TestFailsConversionToNotification(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "list",
	}
	l := logger.NewUPPLogger("test", "PANIC")
	dispatcher := &mocks.Dispatcher{}

	handler := NewContentQueueHandler(defaultContentURIWhitelist, NewSet(), nil, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin"},
		`{"payload": { }, "ContentURI": "http://list-transformer-pr-uk-up.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a" + }`)

	assert.Error(t, handler.HandleMessage(msg), "expect notification parse error")

	dispatcher.AssertNotCalled(t, "Send")
}

func TestHandleMessage(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "lists",
	}
	l := logger.NewUPPLogger("test", "PANIC")
	dispatcher := &mocks.Dispatcher{}
	dispatcher.On("Send", mock.AnythingOfType("dispatch.NotificationModel")).Return()

	handler := NewContentQueueHandler(defaultContentURIWhitelist, NewSet(), nil, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin"},
		`{"UUID": "a uuid", "payload": { }, "ContentURI": "http://list-transformer-pr-uk-up.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	assert.NoError(t, handler.HandleMessage(msg))

	dispatcher.AssertExpectations(t)
}

func TestHandleMessageMappingError(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "lists",
	}
	l := logger.NewUPPLogger("test", "PANIC")

	dispatcher := &mocks.Dispatcher{}
	handler := NewContentQueueHandler(defaultContentURIWhitelist, NewSet(), nil, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin"},
		`{"UUID": "", "payload": { }, "ContentURI": "http://list-transformer-pr-uk-up.svc.ft.com:8080/lists/blah/abc"}`)

	assert.NotNil(t, handler.HandleMessage(msg), "Expected error to HandleMessage when UUID is empty")

	dispatcher.AssertNotCalled(t, "Send")
}

func TestDiscardStandardCarouselPublicationEvents(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "lists",
	}
	l := logger.NewUPPLogger("test", "PANIC")

	dispatcher := &mocks.Dispatcher{}
	handler := NewContentQueueHandler(defaultContentURIWhitelist, NewSet(), nil, mapper, dispatcher, l)

	msg1 := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_fzy2uqund8_carousel_1485954245"},
		`{"UUID": "a uuid", "ContentURI": "http://list-transformer-pr-uk-up.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	msg2 := kafka.NewFTMessage(map[string]string{"X-Request-Id": "republish_-10bd337c-66d4-48d9-ab8a-e8441fa2ec98_carousel_1493606135"},
		`{"UUID": "a uuid", "ContentURI": "http://list-transformer-pr-uk-up.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	msg3 := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_ofcysuifp0_carousel_1488384556_gentx"},
		`{"UUID": "a uuid", "ContentURI": "http://list-transformer-pr-uk-up.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a"}`,
	)
	_ = handler.HandleMessage(msg1)
	_ = handler.HandleMessage(msg2)
	_ = handler.HandleMessage(msg3)

	dispatcher.AssertNotCalled(t, "Send")
}

func TestDiscardCarouselPublicationEventsWithGeneratedTransactionID(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "lists",
	}
	l := logger.NewUPPLogger("test", "PANIC")

	dispatcher := &mocks.Dispatcher{}
	handler := NewContentQueueHandler(defaultContentURIWhitelist, NewSet(), nil, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_fzy2uqund8_carousel_1485954245_gentx"},
		`{"UUID": "a uuid", "ContentURI": "http://list-transformer-pr-uk-up.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a"}`,
	)

	assert.NoError(t, handler.HandleMessage(msg))

	dispatcher.AssertNotCalled(t, "Send")
}

func TestAcceptNotificationBasedOnE2ETransactionID(t *testing.T) {
	t.Parallel()

	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "content",
	}
	e2eTestUUIDs := []string{"e4d2885f-1140-400b-9407-921e1c7378cd"}
	l := logger.NewUPPLogger("test", "PANIC")
	dispatcher := &mocks.Dispatcher{}
	dispatcher.On("Send", mock.AnythingOfType("dispatch.NotificationModel")).Return()

	handler := NewContentQueueHandler(nil, nil, e2eTestUUIDs, mapper, dispatcher, l)

	msg := kafka.NewFTMessage(
		map[string]string{"X-Request-Id": "SYNTHETIC-REQ-MONe4d2885f-1140-400b-9407-921e1c7378cd"},
		`{"payload": { }, "contentURI": "http://upp-content-validator.svc.ft.com/content/e4d2885f-1140-400b-9407-921e1c7378cd"}`,
	)

	assert.NoError(t, handler.HandleMessage(msg))
	dispatcher.AssertExpectations(t)
}
