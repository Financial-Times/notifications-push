package consumer

import (
	"testing"
	"time"

	logger "github.com/Financial-Times/go-logger"
	"github.com/Financial-Times/kafka-client-go/kafka"
	"github.com/Financial-Times/notifications-push/v4/dispatch"
	"github.com/Financial-Times/notifications-push/v4/test/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMetadata(t *testing.T) {
	logger.InitDefaultLogger("test-notifications-push")

	tests := map[string]struct {
		mapper      NotificationMapper
		sendFunc    func(notification []dispatch.Notification)
		whitelist   []string
		msg         kafka.FTMessage
		expectError bool
	}{
		"Test fail on ivalid message body": {
			msg: kafka.FTMessage{
				Headers: map[string]string{
					"X-Request-Id": "tid_test",
				},
				Body: `{`,
			},
			expectError: true,
		},
		"Test Skipping synthetic messages": {
			sendFunc: func(notification []dispatch.Notification) {
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
			sendFunc: func(notification []dispatch.Notification) {
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
		"Test Handle Message": {
			mapper: NotificationMapper{
				APIBaseURL: "test.api.ft.com",
				Resource:   "content",
			},
			sendFunc: func(notification []dispatch.Notification) {
				assert.Equal(t, 1, len(notification))
				expectedNotification := dispatch.Notification{
					Type:             "http://www.ft.com/thing/ThingChangeType/ANNOTATIONS_UPDATE",
					ID:               "http://www.ft.com/thing/fc1d7a28-9506-323f-9558-11beb985e8f7",
					APIURL:           "test.api.ft.com/content/fc1d7a28-9506-323f-9558-11beb985e8f7",
					PublishReference: "tid_test",
					ContentType:      "Annotations",
					LastModified:     time.Now().Format(time.RFC3339),
				}
				assert.Equal(t, expectedNotification, notification[0])
			},
			msg: kafka.FTMessage{
				Headers: map[string]string{
					"X-Request-Id":     "tid_test",
					"Origin-System-Id": "valid-origins",
				},
				Body: `{"uuid":"fc1d7a28-9506-323f-9558-11beb985e8f7","annotations":[]}`,
			},
			whitelist: []string{"valid-origins"},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			dispatcher := &mocks.MockDispatcher{}
			dispatcher.On("Send", mock.AnythingOfType("[]dispatch.Notification")).Return().
				Run(func(args mock.Arguments) {
					arg := args.Get(0).([]dispatch.Notification)
					test.sendFunc(arg)
				})
			handler := NewMetadataQueueHandler(test.whitelist, test.mapper, dispatcher)
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