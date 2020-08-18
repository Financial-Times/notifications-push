package consumer

import (
	"testing"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/kafka-client-go/kafka"
	"github.com/Financial-Times/notifications-push/v5/dispatch"
	"github.com/Financial-Times/notifications-push/v5/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMetadata(t *testing.T) {
	t.Parallel()

	l := logger.NewUPPLogger("test", "panic")

	tests := map[string]struct {
		mapper      NotificationMapper
		sendFunc    func(n dispatch.Notification)
		whitelist   []string
		msg         kafka.FTMessage
		expectError bool
	}{
		"Test fail on invalid message body": {
			msg: kafka.FTMessage{
				Headers: map[string]string{
					"X-Request-Id": "tid_test",
				},
				Body: `{`,
			},
			expectError: true,
		},
		"Test Skipping synthetic messages": {
			sendFunc: func(n dispatch.Notification) {
				t.Fatal("Send should not be called.")
			},
			msg: kafka.FTMessage{
				Headers: map[string]string{
					"X-Request-Id": "SYNTH_tid",
				},
				Body: `{}`,
			},
		},
		"Test Skipping Messages from Unsupported Origins": {
			sendFunc: func(n dispatch.Notification) {
				t.Fatal("Send should not be called.")
			},
			msg: kafka.FTMessage{
				Headers: map[string]string{
					"X-Request-Id":     "tid_test",
					"Origin-System-Id": "invalid-origins",
				},
				Body: `{}`,
			},
			whitelist: []string{"valid-origins"},
		},
		"Test Fail to map message": {
			sendFunc: func(n dispatch.Notification) {
				t.Fatal("Send should not be called.")
			},
			msg: kafka.FTMessage{
				Headers: map[string]string{
					"X-Request-Id":     "tid_test",
					"Origin-System-Id": "valid-origins",
				},
				Body: `{"uuid":"fc1d7a28-9506-323f-9558-11beb985e8f7","annotations":[]}`,
			},
			whitelist:   []string{"valid-origins"},
			expectError: true,
		},
		"Test Handle Message": {
			mapper: NotificationMapper{
				APIBaseURL: "test.api.ft.com",
				Resource:   "content",
			},
			sendFunc: func(n dispatch.Notification) {
				expected := dispatch.Notification{
					Type:             "http://www.ft.com/thing/ThingChangeType/ANNOTATIONS_UPDATE",
					ID:               "http://www.ft.com/thing/fc1d7a28-9506-323f-9558-11beb985e8f7",
					APIURL:           "test.api.ft.com/content/fc1d7a28-9506-323f-9558-11beb985e8f7",
					PublishReference: "tid_test",
					SubscriptionType: "Annotations",
					LastModified:     "2016-11-02T10:54:22.234Z",
				}
				assert.Equal(t, expected, n)
			},
			msg: kafka.FTMessage{
				Headers: map[string]string{
					"X-Request-Id":     "tid_test",
					"Origin-System-Id": "valid-origins",
				},
				Body: `{"contentUri": "http://annotations-rw-neo4j.svc.ft.com/annotations/fc1d7a28-9506-323f-9558-11beb985e8f7","lastModified": "2016-11-02T10:54:22.234Z","payload": {"uuid":"fc1d7a28-9506-323f-9558-11beb985e8f7","annotations":[{"thing":{ "id":"http://www.ft.com/thing/68678217-1d06-4600-9d43-b0e71a333c2a","predicate":"about"}}]}}`,
			},
			whitelist: []string{"valid-origins"},
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dispatcher := &mocks.Dispatcher{}
			dispatcher.On("Send", mock.AnythingOfType("dispatch.Notification")).Return().
				Run(func(args mock.Arguments) {
					arg := args.Get(0).(dispatch.Notification)
					test.sendFunc(arg)
				})
			handler := NewMetadataQueueHandler(test.whitelist, test.mapper, dispatcher, l)
			err := handler.HandleMessage(test.msg)
			if test.expectError {
				if err == nil {
					t.Fatal("expected error, but got non")
				}
			} else {
				if err != nil {
					t.Fatalf("received unexpected error %v", err)
				}
			}
		})
	}
}
