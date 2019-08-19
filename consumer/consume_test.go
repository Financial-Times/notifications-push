package consumer

import (
	"regexp"
	"testing"

	"github.com/Financial-Times/go-logger"
	"github.com/Financial-Times/kafka-client-go/kafka"
	"github.com/Financial-Times/notifications-push/test/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var defaultContentUriWhitelist = regexp.MustCompile(`^http://.*-transformer-(pr|iw)-uk-.*\.svc\.ft\.com(:\d{2,5})?/(lists)/[\w-]+.*$`)
var sparkIncludedWhiteList = regexp.MustCompile("^http://(methode|wordpress|content|upp)-(article|collection|content-placeholder|content)-(mapper|unfolder|validator)(-pr|-iw)?(-uk-.*)?\\.svc\\.ft\\.com(:\\d{2,5})?/(content|complementarycontent)/[\\w-]+.*$")

func init() {
	logger.InitDefaultLogger("test-notifications-push")
}

func TestSyntheticMessage(t *testing.T) {
	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "lists",
	}

	dispatcher := new(mocks.MockDispatcher)
	handler := NewMessageQueueHandler(defaultContentUriWhitelist, NewSet(), mapper, dispatcher)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "SYNTH_tid"},
		`{"UUID": "a uuid", "ContentURI": "http://list-transformer-pr-uk-up.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	handler.HandleMessage(msg)
	dispatcher.AssertNotCalled(t, "Send")
}

func TestFailedCMSMessageParse(t *testing.T) {
	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "lists",
	}

	dispatcher := new(mocks.MockDispatcher)
	handler := NewMessageQueueHandler(defaultContentUriWhitelist, NewSet(), mapper, dispatcher)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin"}, "")

	handler.HandleMessage(msg)
	dispatcher.AssertNotCalled(t, "Send")
}

func TestWhitelist(t *testing.T) {
	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "lists",
	}

	dispatcher := new(mocks.MockDispatcher)
	handler := NewMessageQueueHandler(defaultContentUriWhitelist, NewSet(), mapper, dispatcher)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin"},
		`{"ContentURI": "something which wouldn't match"}`)

	handler.HandleMessage(msg)
	dispatcher.AssertNotCalled(t, "Send")
}

func TestSparkCCTWhitelist(t *testing.T) {
	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "content",
	}

	dispatcher := new(mocks.MockDispatcher)
	dispatcher.On("Send", mock.AnythingOfType("[]dispatch.Notification")).Return()

	handler := NewMessageQueueHandler(sparkIncludedWhiteList, NewSet(), mapper, dispatcher)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin"},
		`{"contentURI": "http://upp-content-validator.svc.ft.com/content/f601289e-93a0-4c08-854e-fef334584079"}`)

	err := handler.HandleMessage(msg)
	assert.NoError(t, err)
	dispatcher.AssertExpectations(t)
}

func TestAcceptNotificationBasedOnContentType(t *testing.T) {
	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "content",
	}
	contentTypeWhitelist := NewSet()
	contentTypeWhitelist.Add("application/vnd.ft-upp-article+json")

	dispatcher := new(mocks.MockDispatcher)
	dispatcher.On("Send", mock.AnythingOfType("[]dispatch.Notification")).Return()

	handler := NewMessageQueueHandler(defaultContentUriWhitelist, contentTypeWhitelist, mapper, dispatcher)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin", "Content-Type": "application/vnd.ft-upp-article+json; version=1.0; charset=utf-8"},
		`{"ContentURI": "http://not-in-the-whitelist.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	err := handler.HandleMessage(msg)
	assert.NoError(t, err)
	dispatcher.AssertExpectations(t)
}

func TestAcceptNotificationBasedOnAudioContentType(t *testing.T) {
	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "content",
	}
	contentTypeWhitelist := NewSet()
	contentTypeWhitelist.Add("application/vnd.ft-upp-audio+json")

	dispatcher := new(mocks.MockDispatcher)
	dispatcher.On("Send", mock.AnythingOfType("[]dispatch.Notification")).Return()

	handler := NewMessageQueueHandler(defaultContentUriWhitelist, contentTypeWhitelist, mapper, dispatcher)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin", "Content-Type": "application/vnd.ft-upp-audio+json"},
		`{"ContentURI": "http://not-in-the-whitelist.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	err := handler.HandleMessage(msg)
	assert.NoError(t, err)
	dispatcher.AssertExpectations(t)
}

func TestDiscardNotificationBasedOnContentType(t *testing.T) {
	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "content",
	}
	contentTypeWhitelist := NewSet()
	contentTypeWhitelist.Add("application/vnd.ft-upp-article+json")

	dispatcher := new(mocks.MockDispatcher)
	dispatcher.On("Send", mock.AnythingOfType("[]dispatch.Notification")).Return()

	handler := NewMessageQueueHandler(sparkIncludedWhiteList, contentTypeWhitelist, mapper, dispatcher)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin", "Content-Type": "application/vnd.ft-upp-invalid+json"},
		`{"ContentURI": "http://methode-article-mapper.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	handler.HandleMessage(msg)
	dispatcher.AssertNotCalled(t, "Send")
}

func TestAcceptNotificationBasedOnContentUriWhenContentTypeIsApplicationJson(t *testing.T) {
	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "content",
	}
	contentTypeWhitelist := NewSet()
	contentTypeWhitelist.Add("application/vnd.ft-upp-article+json")

	dispatcher := new(mocks.MockDispatcher)
	dispatcher.On("Send", mock.AnythingOfType("[]dispatch.Notification")).Return()

	handler := NewMessageQueueHandler(sparkIncludedWhiteList, contentTypeWhitelist, mapper, dispatcher)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin", "Content-Type": "application/json"},
		`{"ContentURI": "http://methode-article-mapper.svc.ft.com:8080/content/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	err := handler.HandleMessage(msg)
	assert.NoError(t, err)
	dispatcher.AssertExpectations(t)
}

func TestDiscardNotificationBasedOnContentUriWhenContentTypeIsApplicationJson(t *testing.T) {
	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "content",
	}
	contentTypeWhitelist := NewSet()
	contentTypeWhitelist.Add("application/vnd.ft-upp-article+json")

	dispatcher := new(mocks.MockDispatcher)
	dispatcher.On("Send", mock.AnythingOfType("[]dispatch.Notification")).Return()

	handler := NewMessageQueueHandler(sparkIncludedWhiteList, contentTypeWhitelist, mapper, dispatcher)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin", "Content-Type": "application/json"},
		`{"ContentURI": "http://not-in-the-whitelist.svc.ft.com:8080/content/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	handler.HandleMessage(msg)
	dispatcher.AssertNotCalled(t, "Send")
}

func TestAcceptNotificationBasedOnContentUriWhenContentTypeIsMissing(t *testing.T) {
	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "content",
	}
	contentTypeWhitelist := NewSet()
	contentTypeWhitelist.Add("application/vnd.ft-upp-article+json")

	dispatcher := new(mocks.MockDispatcher)
	dispatcher.On("Send", mock.AnythingOfType("[]dispatch.Notification")).Return()

	handler := NewMessageQueueHandler(sparkIncludedWhiteList, contentTypeWhitelist, mapper, dispatcher)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin"},
		`{"ContentURI": "http://methode-article-mapper.svc.ft.com:8080/content/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	err := handler.HandleMessage(msg)
	assert.NoError(t, err)
	dispatcher.AssertExpectations(t)
}

func TestDiscardNotificationBasedOnContentUriWhenContentTypeIsMissing(t *testing.T) {
	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "content",
	}
	contentTypeWhitelist := NewSet()
	contentTypeWhitelist.Add("application/vnd.ft-upp-article+json")

	dispatcher := new(mocks.MockDispatcher)
	dispatcher.On("Send", mock.AnythingOfType("[]dispatch.Notification")).Return()

	handler := NewMessageQueueHandler(sparkIncludedWhiteList, contentTypeWhitelist, mapper, dispatcher)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin"},
		`{"ContentURI": "http://not-in-the-whitelist.svc.ft.com:8080/content/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	handler.HandleMessage(msg)
	dispatcher.AssertNotCalled(t, "Send")
}

func TestFailsConversionToNotification(t *testing.T) {
	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "list",
	}

	dispatcher := new(mocks.MockDispatcher)

	handler := NewMessageQueueHandler(defaultContentUriWhitelist, NewSet(), mapper, dispatcher)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin"},
		`{"ContentURI": "http://list-transformer-pr-uk-up.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a" + }`)

	handler.HandleMessage(msg)
	dispatcher.AssertNotCalled(t, "Send")
}

func TestHandleMessage(t *testing.T) {
	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "lists",
	}

	dispatcher := new(mocks.MockDispatcher)
	dispatcher.On("Send", mock.AnythingOfType("[]dispatch.Notification")).Return()

	handler := NewMessageQueueHandler(defaultContentUriWhitelist, NewSet(), mapper, dispatcher)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin"},
		`{"UUID": "a uuid", "ContentURI": "http://list-transformer-pr-uk-up.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a"}`)

	handler.HandleMessage(msg)
	dispatcher.AssertExpectations(t)
}

func TestHandleMessageMappingError(t *testing.T) {
	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "lists",
	}

	dispatcher := new(mocks.MockDispatcher)
	handler := NewMessageQueueHandler(defaultContentUriWhitelist, NewSet(), mapper, dispatcher)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_summin"},
		`{"UUID": "", "ContentURI": "http://list-transformer-pr-uk-up.svc.ft.com:8080/lists/blah/abc"}`)
	err := handler.HandleMessage(msg)
	assert.NotNil(t, err, "Expected error to HandleMessage when UUID is empty")

	dispatcher.AssertNotCalled(t, "Send")
}

func TestDiscardStandardCarouselPublicationEvents(t *testing.T) {
	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "lists",
	}

	dispatcher := new(mocks.MockDispatcher)
	handler := NewMessageQueueHandler(defaultContentUriWhitelist, NewSet(), mapper, dispatcher)

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
	mapper := NotificationMapper{
		APIBaseURL: "test.api.ft.com",
		Resource:   "lists",
	}

	dispatcher := new(mocks.MockDispatcher)
	handler := NewMessageQueueHandler(defaultContentUriWhitelist, NewSet(), mapper, dispatcher)

	msg := kafka.NewFTMessage(map[string]string{"X-Request-Id": "tid_fzy2uqund8_carousel_1485954245_gentx"},
		`{"UUID": "a uuid", "ContentURI": "http://list-transformer-pr-uk-up.svc.ft.com:8080/lists/blah/55e40823-6804-4264-ac2f-b29e11bf756a"}`,
	)

	handler.HandleMessage(msg)
	dispatcher.AssertNotCalled(t, "Send")
}
