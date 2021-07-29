package mocks

import (
	"context"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	"github.com/Financial-Times/notifications-push/v5/dispatch"
	"github.com/stretchr/testify/mock"
)

type KeyProcessor struct {
	mock.Mock
}

func (m *KeyProcessor) Validate(ctx context.Context, key string) error {
	args := m.Called(ctx, key)

	if args.Get(0) != nil {
		return args.Get(0).(error)
	}
	return nil
}

func (m *KeyProcessor) GetPolicies(ctx context.Context, key string) ([]string, error) {
	args := m.Called(ctx, key)

	if args.Get(1) != nil {
		return nil, args.Get(1).(error)
	}

	return args.Get(0).([]string), nil
}

type Dispatcher struct {
	mock.Mock
}

func (m *Dispatcher) Start() {
	m.Called()
}

func (m *Dispatcher) Stop() {
	m.Called()
}

func (m *Dispatcher) Send(notification dispatch.NotificationModel) {
	m.Called(notification)
}

func (m *Dispatcher) Subscribers() []dispatch.Subscriber {
	args := m.Called()
	return args.Get(0).([]dispatch.Subscriber)
}

func (m *Dispatcher) Subscribe(address string, subTypes []string, monitoring bool, options []dispatch.SubscriptionOption) (dispatch.Subscriber, error) {
	args := m.Called(address, subTypes, monitoring, options)
	return args.Get(0).(dispatch.Subscriber), nil
}
func (m *Dispatcher) Unsubscribe(s dispatch.Subscriber) {
	m.Called(s)
}

type transport struct {
	ResponseStatusCode int
	ResponseBody       string
	Error              error
}

func ClientWithResponseCode(responseCode int) *http.Client {
	return &http.Client{
		Transport: &transport{
			ResponseStatusCode: responseCode,
		},
	}
}

func ClientWithResponseBody(responseCode int, responseBody string) *http.Client {
	return &http.Client{
		Transport: &transport{
			ResponseStatusCode: responseCode,
			ResponseBody:       responseBody,
		},
	}
}

func ClientWithError(err error) *http.Client {
	return &http.Client{
		Transport: &transport{
			Error: err,
		},
	}
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	response := &http.Response{
		Header:     make(http.Header),
		Request:    req,
		StatusCode: t.ResponseStatusCode,
	}

	response.Header.Set("Content-Type", "application/json")
	response.Body = ioutil.NopCloser(strings.NewReader(t.ResponseBody))

	if t.Error != nil {
		return nil, t.Error
	}
	return response, nil
}

type KafkaConsumer struct {
	ConnectivityCheckF func() error
}

func (c *KafkaConsumer) ConnectivityCheck() error {
	if c.ConnectivityCheckF != nil {
		return c.ConnectivityCheckF()
	}
	return errors.New("not implemented")
}

type ShutdownReg struct {
	mock.Mock
	m      *sync.Mutex
	toCall []func()
}

func NewShutdownReg() *ShutdownReg {
	return &ShutdownReg{
		m:      &sync.Mutex{},
		toCall: []func(){},
	}
}

func (r *ShutdownReg) RegisterOnShutdown(f func()) {
	r.Called(f)
	r.m.Lock()
	r.toCall = append(r.toCall, f)
	r.m.Unlock()
}

func (r *ShutdownReg) Shutdown() {
	r.m.Lock()
	for _, f := range r.toCall {
		if f != nil {
			f()
		}
	}
	r.toCall = nil
	r.m.Unlock()
}
