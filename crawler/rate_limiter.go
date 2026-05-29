package crawler

import (
	"context"
	"time"
)

type waitForDurationFunc func(ctx context.Context, delay time.Duration) error

type requestLimiter struct {
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
	if err := ctx.Err(); err != nil {
		return err
	}

	if limiter == nil || limiter.interval <= 0 {
		return nil
	}

	if limiter.firstRequest {
		limiter.firstRequest = false
		return nil
	}

	return limiter.wait(ctx, limiter.interval)
}

func waitWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
