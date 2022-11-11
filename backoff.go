package form3api

import (
	"context"
	"math"
	"math/rand"
	"time"
)

const (
	minBackOffMs    = 100
	backOffJitterMs = 100
)

type timer interface {
	Tick() <-chan time.Time
	Stop() bool
}

type timeTimer struct {
	*time.Timer
}

func (t *timeTimer) Tick() <-chan time.Time {
	return t.Timer.C
}

func (t *timeTimer) Stop() bool {
	return t.Timer.Stop()
}

func newTimer(d time.Duration) timer {
	return &timeTimer{
		Timer: time.NewTimer(d),
	}
}

type ctxNewTimer struct{}

type newTimerFunc func(time.Duration) timer

func withNewTimer(ctx context.Context, f newTimerFunc) context.Context {
	return context.WithValue(ctx, ctxNewTimer{}, f)
}

func mustGetNewTimer(ctx context.Context) newTimerFunc {
	if value := ctx.Value(ctxNewTimer{}); value != nil {
		return value.(newTimerFunc)
	}
	return newTimer
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	delay := mustGetNewTimer(ctx)(duration)
	select {
	case <-delay.Tick():
		return nil
	case <-ctx.Done():
		// It is crucial to also stop the timer to avoid leaks.
		if !delay.Stop() {
			<-delay.Tick()
		}
		return ctx.Err()
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func backOff(ctx context.Context, n uint) error {
	d := int(math.Round(math.Pow(1.5, float64(n)) * 500.0))
	d = max(minBackOffMs, d+(rand.Intn(backOffJitterMs)-backOffJitterMs/2))
	return sleepContext(ctx, time.Duration(d)*time.Millisecond)
}
