package resources

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/notifications-push/v5/dispatch"
	"github.com/Financial-Times/notifications-push/v5/mocks"
)

func TestStats(t *testing.T) {
	t.Parallel()

	l := logger.NewUPPLogger("test", "panic")
	d := &mocks.Dispatcher{}
	d.On("Subscribers").Return([]dispatch.Subscriber{})

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/stats", nil)
	if err != nil {
		t.Fatal(err)
	}

	Stats(d, l)(w, req)

	assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "Should be json")
	assert.Equal(t, `{"nrOfSubscribers":0,"subscribers":[]}`, w.Body.String(), "Should be empty array")
	assert.Equal(t, 200, w.Code, "Should be OK")

	d.AssertExpectations(t)
}
