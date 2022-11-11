package form3api

import (
	"context"
	"math"
	"testing"
	"time"
)

type immediateTimer struct{}

func (t *immediateTimer) Tick() <-chan time.Time {
	c := make(chan time.Time)
	go func() { c <- time.Now() }()
	return c
}

func (t *immediateTimer) Stop() bool {
	return true
}

func TestBackOff(t *testing.T) {
	const (
		tolerance = backOffJitterMs * time.Millisecond
	)

	for _, test := range []struct {
		input    uint
		expected int
	}{
		// Values computed here:
		//     https://go.dev/play/p/mQfIKBRQSsq
		{input: 0, expected: 500},
		{input: 1, expected: 750},
		{input: 2, expected: 1125},
		{input: 3, expected: 1688},
		{input: 4, expected: 2531},
		{input: 5, expected: 3797},
		{input: 6, expected: 5695},
		{input: 7, expected: 8543},
		{input: 8, expected: 12814},
		{input: 9, expected: 19222},
	} {
		ctx := withNewTimer(context.Background(), func(d time.Duration) timer {
			expected := time.Duration(test.expected) * time.Millisecond
			if math.Abs(d.Seconds()-expected.Seconds()) > tolerance.Seconds() {
				t.Errorf("back-off delay exceeded expected %d with %d", expected, d)
			}
			return new(immediateTimer)
		})

		backOff(ctx, test.input)
	}
}
