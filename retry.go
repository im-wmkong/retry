package retry

import (
	"context"
	"time"
)

type attemptKeyType struct{}

var attemptKey = attemptKeyType{}

// Do 执行带重试的函数
func Do(
	ctx context.Context,
	fn func(ctx context.Context) error,
	opts ...Option,
) error {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	start := time.Now()
	var lastErr error

	for attempt := 1; attempt <= o.maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		// 最大耗时控制
		if o.maxElapsedTime > 0 && time.Since(start) >= o.maxElapsedTime {
			o.logger.Error(
				"retry aborted: max elapsed time exceeded",
				Field{"attempt", attempt},
				Field{"elapsed", time.Since(start)},
			)
			return lastErr
		}

		attemptCtx := context.WithValue(ctx, attemptKey, attempt)

		err := fn(attemptCtx)
		if err == nil {
			return nil
		}

		lastErr = err

		if !o.retryIf(err) || attempt == o.maxRetries {
			return err
		}

		if o.onRetry != nil {
			o.onRetry(attempt, err)
		}

		o.logger.Info(
			"retrying",
			Field{"attempt", attempt},
			Field{"error", err.Error()},
		)

		sleep := o.backoff(attempt)
		if sleep <= 0 {
			sleep = o.interval
		}

		if sleep > 0 {
			timer := time.NewTimer(sleep)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}

	return lastErr
}

// AttemptFromContext 从 context 中获取当前重试次数
func AttemptFromContext(ctx context.Context) (int, bool) {
	attempt, ok := ctx.Value(attemptKey).(int)
	return attempt, ok
}
