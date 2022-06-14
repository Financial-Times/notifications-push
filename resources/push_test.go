package resources

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	logger "github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/notifications-push/v5/dispatch"
	"github.com/Financial-Times/notifications-push/v5/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSubscription(t *testing.T) {
	t.Parallel()

	l := logger.NewUPPLogger("TEST", "PANIC")

	heartbeat := time.Second * 1
	subAddress := "some-test-host"
	apiKey := "some-test-api-key"

	tests := map[string]struct {
		Request                     string
		IsMonitor                   bool
		ExpectedType                []string
		ExpectedBody                string
		ExpectedStatus              int
		ExpectStream                bool
		SubscriptionOptions         []dispatch.SubscriptionOption
		APIKeyPolicies              []string
		isListHandler               bool
		isPagesHandler              bool
		isExtractingPoliciesAllowed bool
	}{
		"Test Push Default Subscriber": {
			ExpectedType:        []string{"Article"},
			Request:             "/content/notifications-push",
			ExpectedBody:        "data: []\n\n",
			ExpectedStatus:      http.StatusOK,
			ExpectStream:        true,
			SubscriptionOptions: []dispatch.SubscriptionOption{},
			APIKeyPolicies:      []string{},
		},
		"Test Push All Subscriber": {
			ExpectedType:        []string{"Article", "ContentPackage", "Audio"},
			Request:             "/content/notifications-push?type=All",
			ExpectedBody:        "data: []\n\n",
			ExpectedStatus:      http.StatusOK,
			ExpectStream:        true,
			SubscriptionOptions: []dispatch.SubscriptionOption{},
			APIKeyPolicies:      []string{},
		},
		"Test Push all lowercase Subscriber": {
			ExpectedType:        []string{"Article", "ContentPackage", "Audio"},
			Request:             "/content/notifications-push?type=all",
			ExpectedBody:        "data: []\n\n",
			ExpectedStatus:      http.StatusOK,
			ExpectStream:        true,
			SubscriptionOptions: []dispatch.SubscriptionOption{},
			APIKeyPolicies:      []string{},
		},
		"Test Push Standard Subscriber": {
			ExpectedType:        []string{"Audio"},
			Request:             "/content/notifications-push?type=Audio",
			ExpectedBody:        "data: []\n\n",
			ExpectedStatus:      http.StatusOK,
			ExpectStream:        true,
			SubscriptionOptions: []dispatch.SubscriptionOption{},
			APIKeyPolicies:      []string{},
		},
		"Test Push Monitor Subscriber": {
			ExpectedType:        []string{"Article"},
			Request:             "/content/notifications-push?monitor=true",
			IsMonitor:           true,
			ExpectedBody:        "data: []\n\n",
			ExpectedStatus:      http.StatusOK,
			ExpectStream:        true,
			SubscriptionOptions: []dispatch.SubscriptionOption{},
			APIKeyPolicies:      []string{},
		},
		"Test require create events Push Standard Subscriber": {
			ExpectedType:                []string{"Audio"},
			Request:                     "/content/notifications-push?type=Audio",
			ExpectedBody:                "data: []\n\n",
			ExpectedStatus:              http.StatusOK,
			ExpectStream:                true,
			SubscriptionOptions:         []dispatch.SubscriptionOption{dispatch.CreateEventOption},
			APIKeyPolicies:              []string{advancedNotificationsXPolicy},
			isExtractingPoliciesAllowed: true,
		},
		"Test require create events Push Standard Subscriber policy check disabled": {
			ExpectedType:                []string{"Audio"},
			Request:                     "/content/notifications-push?type=Audio",
			ExpectedBody:                "data: []\n\n",
			ExpectedStatus:              http.StatusOK,
			ExpectStream:                true,
			SubscriptionOptions:         []dispatch.SubscriptionOption{},
			APIKeyPolicies:              []string{advancedNotificationsXPolicy},
			isExtractingPoliciesAllowed: false,
		},
		"Test require create events Push Monitor Subscriber": {
			ExpectedType:                []string{"Article"},
			Request:                     "/content/notifications-push?monitor=true",
			IsMonitor:                   true,
			ExpectedBody:                "data: []\n\n",
			ExpectedStatus:              http.StatusOK,
			ExpectStream:                true,
			SubscriptionOptions:         []dispatch.SubscriptionOption{dispatch.CreateEventOption},
			APIKeyPolicies:              []string{advancedNotificationsXPolicy},
			isExtractingPoliciesAllowed: true,
		},
		"Test Push Invalid Subscription": {
			Request:        "/content/notifications-push?type=Invalid",
			ExpectedBody:   "specified type (Invalid) is unsupported\n",
			ExpectedStatus: http.StatusBadRequest,
		},
		"Test push subscriber for lists": {
			isListHandler:       true,
			ExpectedType:        []string{"List"},
			Request:             "/lists/notifications-push",
			ExpectedBody:        "data: []\n\n",
			ExpectedStatus:      http.StatusOK,
			ExpectStream:        true,
			SubscriptionOptions: []dispatch.SubscriptionOption{},
			APIKeyPolicies:      []string{},
		},
		"Test Type is not allowed for Lists push": {
			isListHandler:  true,
			Request:        "/lists/notifications-push?type=Article",
			ExpectedBody:   "specified type (Article) is unsupported\n",
			ExpectedStatus: http.StatusBadRequest,
		},
		"Test push subscriber for pages": {
			isPagesHandler:      true,
			ExpectedType:        []string{"Page"},
			Request:             "/pages/notifications-push",
			ExpectedBody:        "data: []\n\n",
			ExpectedStatus:      http.StatusOK,
			ExpectStream:        true,
			SubscriptionOptions: []dispatch.SubscriptionOption{},
			APIKeyPolicies:      []string{},
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			p := &mocks.KeyProcessor{}
			d := &mocks.Dispatcher{}
			r := mocks.NewShutdownReg()
			r.On("RegisterOnShutdown", mock.Anything).Return()
			defer r.Shutdown()
			handler := NewSubHandler(d, p, r, heartbeat, l, []string{"Article", "ContentPackage", "Audio"},
				[]string{"Annotations", "Article", "ContentPackage", "Audio", "All", "LiveBlogPackage", "LiveBlogPost", "Content"}, "Article", test.isExtractingPoliciesAllowed)

			listHandler := NewSubHandler(d, p, r, heartbeat, l, []string{}, []string{}, "List", false)
			pageHandler := NewSubHandler(d, p, r, heartbeat, l, []string{}, []string{}, "Page", false)

			ctx, cancel := context.WithCancel(context.Background())

			p.On("Validate", mock.Anything, apiKey).Return(nil)
			if test.isExtractingPoliciesAllowed {
				p.On("GetPolicies", mock.Anything, apiKey).Return(test.APIKeyPolicies, nil)
			}

			if test.ExpectStream {
				sub, _ := dispatch.NewStandardSubscriber(subAddress, test.ExpectedType, test.SubscriptionOptions)
				d.On("Subscribe", subAddress, test.ExpectedType, test.IsMonitor, test.SubscriptionOptions).Run(func(args mock.Arguments) {
					go func() {
						<-time.After(time.Millisecond * 10)
						cancel()
					}()
				}).Return(sub)
				d.On("Unsubscribe", mock.Anything).Return()
			} else {
				defer cancel()
			}

			resp := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, test.Request, nil)
			req = req.WithContext(ctx)
			req.Header.Set(APIKeyHeaderField, apiKey)
			req.Header.Set(ClientAdrKey, subAddress)

			if test.isListHandler {
				listHandler.HandleSubscription(resp, req)
			} else if test.isPagesHandler {
				pageHandler.HandleSubscription(resp, req)
			} else {
				handler.HandleSubscription(resp, req)
			}

			if test.ExpectStream {
				assertHeaders(t, resp.Header())
			}

			reader := bufio.NewReader(resp.Body)
			body, _ := reader.ReadString(byte(0))

			assert.Equal(t, test.ExpectedBody, body)

			assert.Equal(t, test.ExpectedStatus, resp.Code)
			d.AssertExpectations(t)
			p.AssertExpectations(t)
		})
	}
}

func TestPassKeyAsParameter(t *testing.T) {
	t.Parallel()

	l := logger.NewUPPLogger("TEST", "PANIC")

	ctx, cancel := context.WithCancel(context.Background())

	keyAPI := "some-test-api-key"
	heartbeat := time.Second * 1

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/content/notifications-push", nil)
	req = req.WithContext(ctx)
	q := req.URL.Query()
	q.Add(apiKeyQueryParam, keyAPI)
	req.URL.RawQuery = q.Encode()

	p := &mocks.KeyProcessor{}
	p.On("Validate", mock.Anything, keyAPI).Return(nil)
	p.On("GetPolicies", mock.Anything, keyAPI).Return([]string{}, nil)

	sub, _ := dispatch.NewStandardSubscriber(req.RemoteAddr, []string{"Article"}, []dispatch.SubscriptionOption{})
	d := &mocks.Dispatcher{}
	d.On("Subscribe", req.RemoteAddr, []string{"Article"}, false, []dispatch.SubscriptionOption{}).Run(func(args mock.Arguments) {
		go func() {
			<-time.After(time.Millisecond * 10)
			cancel()
		}()
	}).Return(sub)
	d.On("Unsubscribe", mock.AnythingOfType("*dispatch.StandardSubscriber")).Return()
	r := mocks.NewShutdownReg()
	r.On("RegisterOnShutdown", mock.Anything).Return()
	defer r.Shutdown()

	handler := NewSubHandler(d, p, r, heartbeat, l, []string{"Article", "ContentPackage", "Audio"},
		[]string{"Annotations", "Article", "ContentPackage", "Audio", "All", "LiveBlogPackage", "LiveBlogPost", "Content"}, "Article", true)

	handler.HandleSubscription(resp, req)

	assertHeaders(t, resp.Header())

	reader := bufio.NewReader(resp.Body)
	body, _ := reader.ReadString(byte(0)) // read to EOF

	assert.Equal(t, "data: []\n\n", body)

	assert.Equal(t, http.StatusOK, resp.Code, "Should be OK")
	d.AssertExpectations(t)
	p.AssertExpectations(t)
}

func TestInvalidKey(t *testing.T) {
	t.Parallel()

	l := logger.NewUPPLogger("TEST", "PANIC")

	keyAPI := "some-test-api-key"
	heartbeat := time.Second * 1

	p := &mocks.KeyProcessor{}
	p.On("Validate", mock.Anything, keyAPI).Return(NewKeyErr("failed key", http.StatusForbidden, ""))

	d := &mocks.Dispatcher{}
	r := mocks.NewShutdownReg()

	handler := NewSubHandler(d, p, r, heartbeat, l, []string{"Article", "ContentPackage", "Audio"},
		[]string{"Annotations", "Article", "ContentPackage", "Audio", "All", "LiveBlogPackage", "LiveBlogPost", "Content"}, "Article", false)

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/content/notifications-push", nil)
	req.Header.Set(APIKeyHeaderField, keyAPI)

	handler.HandleSubscription(resp, req)

	body, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, http.StatusForbidden, resp.Code, "Expect error for invalid key")
	assert.Equal(t, "failed key\n", string(body))

	p.AssertExpectations(t)
}

func TestHeartbeat(t *testing.T) {
	t.Parallel()

	l := logger.NewUPPLogger("TEST", "PANIC")

	ctx, cancel := context.WithCancel(context.Background())

	subAddress := "some-test-host"
	keyAPI := "some-test-api-key"
	heartbeat := time.Second * 1
	heartbeatMsg := "data: []\n\n"

	p := &mocks.KeyProcessor{}
	p.On("Validate", mock.Anything, keyAPI).Return(nil)
	p.On("GetPolicies", mock.Anything, keyAPI).Return([]string{}, nil)

	sub, _ := dispatch.NewStandardSubscriber(subAddress, []string{"Article"}, []dispatch.SubscriptionOption{})
	d := &mocks.Dispatcher{}
	d.On("Subscribe", subAddress, []string{"Article"}, false, []dispatch.SubscriptionOption{}).Return(sub)
	d.On("Unsubscribe", mock.AnythingOfType("*dispatch.StandardSubscriber")).Return()
	r := mocks.NewShutdownReg()
	r.On("RegisterOnShutdown", mock.Anything).Return()
	defer r.Shutdown()

	handler := NewSubHandler(d, p, r, heartbeat, l, []string{"Article", "ContentPackage", "Audio"},
		[]string{"Annotations", "Article", "ContentPackage", "Audio", "All", "LiveBlogPackage", "LiveBlogPost", "Content"}, "Article", true)

	req, _ := http.NewRequest(http.MethodGet, "/content/notifications-push", nil)
	req = req.WithContext(ctx)
	req.Header.Set(APIKeyHeaderField, keyAPI)
	req.Header.Set(ClientAdrKey, subAddress)

	pipe := newPipedResponse()
	defer pipe.Close()

	go func() {
		handler.HandleSubscription(pipe, req)
	}()

	start := time.Now()
	msg, _ := pipe.readString()
	assert.Equal(t, heartbeatMsg, msg, "Read incoming heartbeat")
	delay := time.Since(start)
	assert.True(t, delay+(time.Millisecond*100) < heartbeat, "The fist heartbeat should not wait for hearbeat delay")

	start = time.Now()
	msg, _ = pipe.readString()
	delay = time.Since(start)

	assert.InEpsilon(t, heartbeat.Nanoseconds(), delay.Nanoseconds(), 0.05, "The second heartbeat delay should be send within 0.05 relative error")
	assert.Equal(t, heartbeatMsg, msg, "The second heartbeat message is correct")

	start = start.Add(heartbeat)
	msg, _ = pipe.readString()
	delay = time.Since(start)

	assert.InEpsilon(t, heartbeat.Nanoseconds(), delay.Nanoseconds(), 0.05, "The third heartbeat delay should be send within 0.05 relative error")
	assert.Equal(t, heartbeatMsg, msg, "The third heartbeat message is correct")

	cancel()
	// wait for handler to close the connection
	<-time.After(time.Millisecond * 5)

	assert.Equal(t, http.StatusOK, pipe.Code, "Should be OK")
	d.AssertExpectations(t)
	p.AssertExpectations(t)
}

func TestPushNotificationDelay(t *testing.T) {
	t.Parallel()

	l := logger.NewUPPLogger("TEST", "PANIC")

	ctx, cancel := context.WithCancel(context.Background())

	subAddress := "some-test-host"
	keyAPI := "some-test-api-key"
	heartbeat := time.Second * 1
	notificationDelay := time.Millisecond * 100
	heartbeatMsg := "data: []\n\n"

	p := &mocks.KeyProcessor{}
	p.On("Validate", mock.Anything, keyAPI).Return(nil)
	p.On("GetPolicies", mock.Anything, keyAPI).Return([]string{}, nil)

	sub, _ := dispatch.NewStandardSubscriber(subAddress, []string{"Article"}, []dispatch.SubscriptionOption{})
	d := &mocks.Dispatcher{}
	d.On("Subscribe", subAddress, []string{"Article"}, false, []dispatch.SubscriptionOption{}).Run(func(args mock.Arguments) {
		go func() {
			<-time.After(notificationDelay)
			err := sub.Send(dispatch.NotificationResponse{})
			assert.NoError(t, err)
		}()
	}).Return(sub)
	d.On("Unsubscribe", mock.AnythingOfType("*dispatch.StandardSubscriber")).Return()
	r := mocks.NewShutdownReg()
	r.On("RegisterOnShutdown", mock.Anything).Return()
	defer r.Shutdown()

	handler := NewSubHandler(d, p, r, heartbeat, l, []string{"Article", "ContentPackage", "Audio"},
		[]string{"Annotations", "Article", "ContentPackage", "Audio", "All", "LiveBlogPackage", "LiveBlogPost", "Content"}, "Article", true)

	req, _ := http.NewRequest(http.MethodGet, "/content/notifications-push", nil)
	req = req.WithContext(ctx)
	req.Header.Set(APIKeyHeaderField, keyAPI)
	req.Header.Set(ClientAdrKey, subAddress)

	pipe := newPipedResponse()
	defer pipe.Close()

	go func() {
		handler.HandleSubscription(pipe, req)
	}()

	start := time.Now()
	msg, _ := pipe.readString()
	assert.Equal(t, heartbeatMsg, msg, "Read incoming heartbeat")
	delay := time.Since(start)
	assert.True(t, delay+(time.Millisecond*100) < heartbeat, "The fist heartbeat should not wait for hearbeat delay")

	start = time.Now()
	msg, _ = pipe.readString()
	delay = time.Since(start)

	assert.InEpsilon(t, notificationDelay.Nanoseconds(), delay.Nanoseconds(), 0.08, "The notification is send in the correct time frame")
	assert.Equal(t, "data: [{\"apiUrl\":\"\",\"id\":\"\",\"type\":\"\"}]\n\n\n", msg, "Should get the notification")

	start = start.Add(notificationDelay)
	msg, _ = pipe.readString()
	delay = time.Since(start)

	assert.InEpsilon(t, heartbeat.Nanoseconds(), delay.Nanoseconds(), 0.05, "The third heartbeat delay should be send within 0.05 relative error")
	assert.Equal(t, heartbeatMsg, msg, "The third heartbeat message is correct")

	cancel()
	// wait for handler to close the connection
	<-time.After(time.Millisecond * 5)

	assert.Equal(t, http.StatusOK, pipe.Code, "Should be OK")
	d.AssertExpectations(t)
	p.AssertExpectations(t)
}

func assertHeaders(t *testing.T, h http.Header) bool {
	pass := true
	pass = pass && assert.Equal(t, "text/event-stream; charset=UTF-8", h.Get("Content-Type"), "Should be SSE")
	pass = pass && assert.Equal(t, "no-cache, no-store, must-revalidate", h.Get("Cache-Control"))
	pass = pass && assert.Equal(t, "keep-alive", h.Get("Connection"))
	pass = pass && assert.Equal(t, "no-cache", h.Get("Pragma"))
	pass = pass && assert.Equal(t, "0", h.Get("Expires"))
	return pass
}

type pipedResponse struct {
	httptest.ResponseRecorder
	r *io.PipeReader
	w *io.PipeWriter
}

func newPipedResponse() *pipedResponse {
	r, w := io.Pipe()
	return &pipedResponse{
		ResponseRecorder: httptest.ResponseRecorder{
			HeaderMap: make(http.Header),
			Body:      new(bytes.Buffer),
			Code:      200,
		},
		r: r,
		w: w,
	}
}

func (p *pipedResponse) Write(data []byte) (int, error) {
	return p.w.Write(data)
}
func (p *pipedResponse) Read(data []byte) (int, error) {
	return p.r.Read(data)
}

func (p *pipedResponse) Close() error {
	return p.w.Close()
}

func (p *pipedResponse) readString() (string, error) {
	const bufSize = 4096
	buf := make([]byte, bufSize)
	idx, err := p.r.Read(buf)
	if err != nil {
		return "", err
	}
	return string(buf[:idx]), nil
}
