package crawler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"
)

// TestRequestLimiter_WaitsBetweenRequests проверяет, что limiter выдерживает
// заданную задержку между последовательными запросами.
func TestRequestLimiter_WaitsBetweenRequests(t *testing.T) {
	var virtualNow time.Duration

	limiter := newRequestLimiterWithWait(Options{
		Delay: 200 * time.Millisecond,
	}, func(ctx context.Context, delay time.Duration) error {
		virtualNow += delay
		return nil
	})

	starts := make([]time.Duration, 0, 4)

	for range 4 {
		if err := limiter.Wait(context.Background()); err != nil {
			t.Fatalf("got wait error %v, want nil", err)
		}

		starts = append(starts, virtualNow)
	}

	for i := 1; i < len(starts); i++ {
		got := starts[i] - starts[i-1]
		want := 200 * time.Millisecond

		if got < want {
			t.Fatalf("got interval %s, want at least %s", got, want)
		}
	}
}

// TestRequestLimiter_RPSOverridesDelay проверяет, что RPS имеет приоритет
// над прямым значением Delay и задаёт интервал между запросами.
func TestRequestLimiter_RPSOverridesDelay(t *testing.T) {
	waits := make([]time.Duration, 0)

	limiter := newRequestLimiterWithWait(Options{
		Delay: time.Second,
		RPS:   5,
	}, func(ctx context.Context, delay time.Duration) error {
		waits = append(waits, delay)
		return nil
	})

	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatalf("got first wait error %v, want nil", err)
	}

	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatalf("got second wait error %v, want nil", err)
	}

	{
		got := len(waits)
		want := 1

		if got != want {
			t.Fatalf("got waits len %d, want %d", got, want)
		}
	}

	{
		got := waits[0]
		want := 200 * time.Millisecond

		if got != want {
			t.Fatalf("got wait duration %s, want %s", got, want)
		}
	}
}

// TestRequestLimiter_NoLimitDoesNotWait проверяет, что при отсутсвии Delay и RPS
// limiter не вызывает ожидание между запросами.
func TestRequestLimiter_NoLimitDoesNotWait(t *testing.T) {
	called := false

	limiter := newRequestLimiterWithWait(Options{}, func(ctx context.Context, delay time.Duration) error {
		called = true
		return nil
	})

	for range 3 {
		if err := limiter.Wait(context.Background()); err != nil {
			t.Fatalf("got wait error %v, want nil", err)
		}
	}

	{
		got := called
		want := false

		if got != want {
			t.Fatalf("got wait called %t, want %t", got, want)
		}
	}
}

// TestRequestLimiter_ContextCancelStopsWaiting проверяет, что отмена контекста
// прерывает ожидание внутри limiter.
func TestRequestLimiter_ContextCancelStopsWaiting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	waitStarted := make(chan struct{})

	limiter := newRequestLimiterWithWait(Options{
		Delay: time.Hour,
	}, func(ctx context.Context, delay time.Duration) error {
		close(waitStarted)
		<-ctx.Done()

		return ctx.Err()
	})

	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatalf("got first wait error %v, want nil", err)
	}

	errCh := make(chan error, 1)

	go func() {
		errCh <- limiter.Wait(ctx)
	}()

	<-waitStarted
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got wait error %v, want %v", err, context.Canceled)
		}
	case <-time.After(time.Second):
		t.Fatal("got hanging wait, want context cancellation to stop it")
	}
}

// TestAnalyze_LimitedAndUnlimitedSpeedProcessSamePages проверяет, что
// ограничение скорости не меняет итоговый набор обработанных страниц.
func TestAnalyze_LimitedAndUnlimitedSpeedProcessSamePages(t *testing.T) {
	const mockedAboutURL = mockedBaseURL + "/about"

	tests := []struct {
		name  string
		delay time.Duration
	}{
		{
			name:  "unlimited",
			delay: 0,
		},
		{
			name:  "limited",
			delay: time.Nanosecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{
				Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
					pages := map[string]string{
						mockedBaseURL:  `<html><body><a href="/about">About</a></body></html>`,
						mockedAboutURL: `<html><body>About</body></html>`,
					}

					body, ok := pages[rq.URL.String()]
					if !ok {
						return newTestResponse(rq, http.StatusNotFound, "404 Not Found", "not found"), nil
					}

					return newTestResponse(rq, http.StatusOK, "200 OK", body), nil
				}),
			}

			result, err := Analyze(context.Background(), Options{
				URL:        mockedBaseURL,
				Depth:      2,
				Delay:      tt.delay,
				HTTPClient: client,
			})

			if err != nil {
				t.Fatalf("got error %v, want nil", err)
			}

			var report Report
			if err := json.Unmarshal(result, &report); err != nil {
				t.Fatalf("got unmarshal error %v, want nil", err)
			}

			{
				got := len(report.Pages)
				want := 2

				if got != want {
					t.Fatalf("got pages len %d, want %d", got, want)
				}
			}

			for _, page := range report.Pages {
				got := page.Status
				want := PageStatusOK

				if got != want {
					t.Fatalf("got page %q status %q, want %q", page.URL, got, want)
				}
			}
		})
	}
}
