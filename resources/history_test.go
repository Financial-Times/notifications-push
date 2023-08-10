package resources

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/notifications-push/v5/dispatch"
)

func TestHistory(t *testing.T) {
	t.Parallel()

	l := logger.NewUPPLogger("test", "panic")
	history := dispatch.NewHistory(1)

	req, err := http.NewRequest("GET", "/__history", nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	History(history, l)(w, req)

	assert.Equal(t, "application/json; charset=UTF-8", w.Header().Get("Content-Type"), "Should be json")
	assert.Equal(t, 200, w.Code, "Should be OK")
}
