package dispatch

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Financial-Times/notifications-push/v5/access"
)

func TestCreateNotificationResponse(t *testing.T) {
	tests := []struct {
		name string
		n    NotificationModel
		s    *access.NotificationSubscriptionOptions
		res  NotificationResponse
	}{
		{
			n: NotificationModel{
				Type: ContentCreateType,
			},
			s: &access.NotificationSubscriptionOptions{
				ReceiveAdvancedNotifications: true,
			},
			res: NotificationResponse{
				Type: ContentCreateType,
			},
		},
		{
			n: NotificationModel{
				Type: ContentCreateType,
			},
			s: &access.NotificationSubscriptionOptions{
				ReceiveAdvancedNotifications: false,
			},
			res: NotificationResponse{
				Type: ContentUpdateType,
			},
		},
		{
			n: NotificationModel{
				Type: ContentUpdateType,
			},
			s: &access.NotificationSubscriptionOptions{
				ReceiveAdvancedNotifications: true,
			},
			res: NotificationResponse{
				Type: ContentUpdateType,
			},
		},
		{
			n: NotificationModel{
				Type: ContentUpdateType,
			},
			s: &access.NotificationSubscriptionOptions{
				ReceiveAdvancedNotifications: false,
			},
			res: NotificationResponse{
				Type: ContentUpdateType,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res := CreateNotificationResponse(test.n, test.s)
			assert.Equal(t, test.res, res)
		})
	}
}
