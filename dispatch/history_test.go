package dispatch

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHistory(t *testing.T) {
	t.Parallel()

	history := NewHistory(2)
	lastModified := time.Now()

	history.Push(NotificationModel{
		ID:           "note1",
		LastModified: lastModified.Add(-2 * time.Second).Format(time.RFC3339Nano),
	})

	history.Push(NotificationModel{
		ID:           "note2",
		LastModified: lastModified.Add(-1 * time.Second).Format(time.RFC3339Nano),
	})

	notifications := history.Notifications()
	assert.Equal(t, 2, len(notifications), "Should be size 2")

	history.Push(NotificationModel{
		ID:           "note3",
		LastModified: lastModified.Format(time.RFC3339Nano),
	})

	notifications = history.Notifications()
	assert.Equal(t, 2, len(notifications), "Should still be size 2")

	for _, n := range notifications {
		t.Log(n.ID)
	}

	assert.Equal(t, "note3", notifications[0].ID, "Should be the last pushed notification")
}
