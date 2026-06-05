package crawler

import (
	"context"
	"sync"
	"time"
)

type waitForDurationFunc func(ctx context.Context, delay time.Duration) error

type requestLimiter struct {
	mu           sync.Mutex
	interval     time.Duration
	wait         waitForDurationFunc
	firstRequest bool
}

func newRequestLimiter(opts Options) *requestLimiter {
	return newRequestLimiterWithWait(opts, waitWithContext)
}

func newRequestLimiterWithWait(opts Options, wait waitForDurationFunc) *requestLimiter {
	return &requestLimiter{
		interval:     requestInterval(opts),
		wait:         wait,
		firstRequest: true,
	}
}

func requestInterval(opts Options) time.Duration {
	if opts.RPS > 0 {
		return time.Second / time.Duration(opts.RPS)
	}

	if opts.Delay > 0 {
		return opts.Delay
	}

	return 0
}

func (limiter *requestLimiter) Wait(ctx context.Context) error {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}

	if limiter.interval == 0 {
		return nil
	}

	if limiter.firstRequest {
		limiter.firstRequest = false
		return ctx.Err()
	}

	return limiter.wait(ctx, limiter.interval)
}

func waitWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
