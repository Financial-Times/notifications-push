package main

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/samuel/go-zookeeper/zk"
	"github.com/stretchr/testify/assert"
	"github.com/wvanbergen/kazoo-go"
)

var fatalErrs = []error{kazoo.ErrPartitionNotClaimed, zk.ErrNoServer}
var nonFatalErrs = []error{errors.New("foo"), errors.New("bar")}

func TestSupervise(t *testing.T) {
	t.Parallel()

	var testCases = []struct {
		description          string
		inputErrs            []error
		expectedHandlerCalls int
	}{
		{
			description:          "No errors",
			inputErrs:            []error{},
			expectedHandlerCalls: 0,
		},
		{
			description:          "Only non-fatal errors",
			inputErrs:            nonFatalErrs,
			expectedHandlerCalls: 0,
		},
		{
			description:          "Only fatal errors",
			inputErrs:            fatalErrs,
			expectedHandlerCalls: len(fatalErrs),
		},
		{
			description:          "Fatal and non-fatal errors",
			inputErrs:            merge(nonFatalErrs, fatalErrs),
			expectedHandlerCalls: len(fatalErrs),
		},
		{
			description:          "Nil error",
			inputErrs:            []error{nil, nil},
			expectedHandlerCalls: 0,
		},
	}
	for _, tc := range testCases {
		errCh := make(chan error)
		var handlerCalls int32
		fatalErrHandler := func(err error, serviceName string) {
			atomic.AddInt32(&handlerCalls, 1)
		}

		supervisor := newServiceSupervisor("notifications-push", errCh, fatalErrs, fatalErrHandler)
		go supervisor.Supervise()
		for _, err := range tc.inputErrs {
			errCh <- err
		}

		time.Sleep(250 * time.Millisecond)
		close(errCh)

		assert.Equal(t, int32(tc.expectedHandlerCalls), atomic.LoadInt32(&handlerCalls),
			fmt.Sprintf("Expected %d handler calls, got %d.", tc.expectedHandlerCalls, atomic.LoadInt32(&handlerCalls)))
	}
}

func merge(s1, s2 []error) []error {
	r := make([]error, len(s1))
	copy(r, s1)
	return append(r, s2...)
}
