package retry

import (
	"context"
	"fmt"
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
		// 检查 Context 是否取消
		if err := ctx.Err(); err != nil {
			return err
		}

		// 最大耗时控制
		elapsed := time.Since(start)
		if o.maxElapsedTime > 0 && elapsed >= o.maxElapsedTime {
			o.logger.Error(ctx, 
                "retry aborted: max elapsed time exceeded",
                Field{"attempt", attempt},
                Field{"elapsed", elapsed},
            )
			if lastErr != nil {
				return fmt.Errorf("retry timeout (elapsed %v): %w", elapsed, lastErr)
			}
			return fmt.Errorf("retry timeout before start: %v", elapsed)
		}

		// 注入当前尝试次数到 Context
		attemptCtx := context.WithValue(ctx, attemptKey, attempt)

		// 执行业务函数
		err := fn(attemptCtx)
		if err == nil {
			return nil
		}
		lastErr = err

		// 是否达到最大次数
		if attempt == o.maxRetries {
			o.logger.Warn(attemptCtx, 
                "retry aborted: max retries reached",
                Field{"attempt", attempt},
                Field{"error", err.Error()},
            )
			return err
		}

		// 不满足重试条件
		if !o.retryIf(err) {
			o.logger.Debug(attemptCtx, 
                "retry aborted: error not retryable",
                Field{"attempt", attempt},
                Field{"error", err.Error()},
            )
			return err
		}

		// 执行重试前回调
		if o.onRetry != nil {
			o.onRetry(attempt, err)
		}

		// 打印重试日志
		o.logger.Info(attemptCtx, 
            "retrying",
            Field{"attempt", attempt},
            Field{"error", err.Error()},
        )

		// 等待退避时间
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
