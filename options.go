package retry

import "time"

// Option functional option
type Option func(*options)

type options struct {
	maxRetries     int
	maxElapsedTime time.Duration
	interval       time.Duration
	backoff        func(attempt int) time.Duration
	retryIf        func(err error) bool
	onRetry        func(attempt int, err error)
	logger         Logger
}

func defaultOptions() *options {
	return &options{
		maxRetries:     1,
		maxElapsedTime: 0,
		interval:       0,
		backoff: func(int) time.Duration {
			return 0
		},
		retryIf: func(err error) bool {
			return err != nil
		},
		onRetry: nil,
		logger:  &noopLogger{},
	}
}

// WithMaxRetries 设置最大尝试次数（>=1）
func WithMaxRetries(n int) Option {
	return func(o *options) {
		if n > 0 {
			o.maxRetries = n
		}
	}
}

// WithMaxElapsedTime 设置最大允许的总重试耗时
func WithMaxElapsedTime(d time.Duration) Option {
	return func(o *options) {
		if d > 0 {
			o.maxElapsedTime = d
		}
	}
}

// WithInterval 固定重试间隔
func WithInterval(d time.Duration) Option {
	return func(o *options) {
		o.interval = d
	}
}

// WithBackoff 自定义退避策略（优先级高于 Interval）
func WithBackoff(fn func(attempt int) time.Duration) Option {
	return func(o *options) {
		if fn != nil {
			o.backoff = fn
		}
	}
}

// WithRetryIf 是否对该错误重试
func WithRetryIf(fn func(err error) bool) Option {
	return func(o *options) {
		if fn != nil {
			o.retryIf = fn
		}
	}
}

// WithOnRetry 每次重试前的回调（常用于 metrics / log）
func WithOnRetry(fn func(attempt int, err error)) Option {
	return func(o *options) {
		o.onRetry = fn
	}
}

// WithLogger 设置日志实现
func WithLogger(l Logger) Option {
	return func(o *options) {
		if l != nil {
			o.logger = l
		}
	}
}
