package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

// MockLogger 用于测试的日志记录器
type MockLogger struct {
	infoCalls []struct {
		msg    string
		fields []Field
	}
	errorCalls []struct {
		msg    string
		fields []Field
	}
}

func (m *MockLogger) Info(msg string, fields ...Field) {
	m.infoCalls = append(m.infoCalls, struct {
		msg    string
		fields []Field
	}{msg: msg, fields: fields})
}

func (m *MockLogger) Error(msg string, fields ...Field) {
	m.errorCalls = append(m.errorCalls, struct {
		msg    string
		fields []Field
	}{msg: msg, fields: fields})
}

// 检查是否有指定消息的Info调用
func (m *MockLogger) HasInfo(msg string) bool {
	for _, call := range m.infoCalls {
		if call.msg == msg {
			return true
		}
	}
	return false
}

// 检查是否有指定消息的Error调用
func (m *MockLogger) HasError(msg string) bool {
	for _, call := range m.errorCalls {
		if call.msg == msg {
			return true
		}
	}
	return false
}

// 获取Info调用次数
func (m *MockLogger) InfoCallCount() int {
	return len(m.infoCalls)
}

// 获取Error调用次数
func (m *MockLogger) ErrorCallCount() int {
	return len(m.errorCalls)
}

// TestDo_SuccessFirstAttempt 测试第一次尝试就成功
func TestDo_SuccessFirstAttempt(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	err := Do(ctx, func(ctx context.Context) error {
		attempts++
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

// TestDo_RetryUntilSuccess 测试重试直到成功
func TestDo_RetryUntilSuccess(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	err := Do(ctx, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary error")
		}
		return nil
	}, WithMaxRetries(5))

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

// TestDo_MaxRetriesExceeded 测试超过最大重试次数
func TestDo_MaxRetriesExceeded(t *testing.T) {
	ctx := context.Background()
	attempts := 0
	expectedErr := errors.New("persistent error")

	err := Do(ctx, func(ctx context.Context) error {
		attempts++
		return expectedErr
	}, WithMaxRetries(3))

	if err == nil {
		t.Error("expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

// TestDo_ContextCanceled 测试上下文取消
func TestDo_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0

	// 启动一个goroutine在稍后取消上下文
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := Do(ctx, func(ctx context.Context) error {
		attempts++
		// 模拟长时间运行
		time.Sleep(100 * time.Millisecond)
		return errors.New("temporary error")
	}, WithMaxRetries(5), WithInterval(10*time.Millisecond))

	if err == nil {
		t.Error("expected error, got nil")
	}
	if err != context.Canceled {
		t.Errorf("expected error %v, got %v", context.Canceled, err)
	}
	if attempts < 1 {
		t.Errorf("expected at least 1 attempt, got %d", attempts)
	}
	if attempts >= 5 {
		t.Errorf("expected less than 5 attempts, got %d", attempts)
	}
}

// TestDo_MaxElapsedTimeExceeded 测试超过最大耗时
func TestDo_MaxElapsedTimeExceeded(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	err := Do(ctx, func(ctx context.Context) error {
		attempts++
		time.Sleep(30 * time.Millisecond)
		return errors.New("temporary error")
	}, WithMaxRetries(5), WithMaxElapsedTime(100*time.Millisecond), WithInterval(10*time.Millisecond))

	if err == nil {
		t.Error("expected error, got nil")
	}
	if attempts < 3 {
		t.Errorf("expected at least 3 attempts, got %d", attempts)
	}
	if attempts >= 5 {
		t.Errorf("expected less than 5 attempts, got %d", attempts)
	}
}

// TestDo_RetryIfCondition 测试重试条件
func TestDo_RetryIfCondition(t *testing.T) {
	ctx := context.Background()
	attempts := 0
	shouldRetryErr := errors.New("should retry")
	shouldNotRetryErr := errors.New("should not retry")

	err := Do(ctx, func(ctx context.Context) error {
		attempts++
		if attempts == 1 {
			return shouldRetryErr
		}
		return shouldNotRetryErr
	}, WithMaxRetries(3), WithRetryIf(func(err error) bool {
		return err == shouldRetryErr
	}))

	if err == nil {
		t.Error("expected error, got nil")
	}
	if err != shouldNotRetryErr {
		t.Errorf("expected error %v, got %v", shouldNotRetryErr, err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

// TestDo_BackoffStrategy 测试退避策略
func TestDo_BackoffStrategy(t *testing.T) {
	ctx := context.Background()
	attempts := 0
	var backoffTimes []time.Duration
	var backoffStart time.Time

	// 自定义退避策略：每次重试间隔增加10ms
	backoffFunc := func(attempt int) time.Duration {
		return time.Duration(attempt*10) * time.Millisecond
	}

	err := Do(ctx, func(ctx context.Context) error {
		attempts++
		// 记录退避结束时间（除了第一次尝试）
		if attempts > 1 {
			backoffEnd := time.Now()
			backoffTimes = append(backoffTimes, backoffEnd.Sub(backoffStart))
		}

		if attempts < 4 {
			return errors.New("temporary error")
		}
		return nil
	}, WithMaxRetries(5), WithBackoff(backoffFunc), WithOnRetry(func(attempt int, err error) {
		// 在重试前记录退避开始时间
		backoffStart = time.Now()
	}))

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attempts != 4 {
		t.Errorf("expected 4 attempts, got %d", attempts)
	}
	if len(backoffTimes) != 3 {
		t.Errorf("expected 3 backoff times, got %d", len(backoffTimes))
	}

	// 检查退避时间是否符合预期（允许一定误差，因为有函数执行开销）
	for i, elapsed := range backoffTimes {
		expectedMin := time.Duration((i+1)*10) * time.Millisecond
		// 允许1ms的误差
		if elapsed < expectedMin-1*time.Millisecond {
			t.Errorf("backoff time %d should be at least %v, got %v", i, expectedMin, elapsed)
		}
		// 退避时间不应该太长
		if elapsed > expectedMin+10*time.Millisecond {
			t.Errorf("backoff time %d should be at most %v, got %v", i, expectedMin+10*time.Millisecond, elapsed)
		}
	}
}

// TestDo_OnRetryCallback 测试重试回调
func TestDo_OnRetryCallback(t *testing.T) {
	ctx := context.Background()
	attempts := 0
	var retryCallbacks []struct {
		attempt int
		err     error
	}

	expectedErr := errors.New("temporary error")

	err := Do(ctx, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return expectedErr
		}
		return nil
	}, WithMaxRetries(5), WithOnRetry(func(attempt int, err error) {
		retryCallbacks = append(retryCallbacks, struct {
			attempt int
			err     error
		}{attempt: attempt, err: err})
	}))

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
	if len(retryCallbacks) != 2 {
		t.Errorf("expected 2 retry callbacks, got %d", len(retryCallbacks))
	}

	// 检查回调参数
	if retryCallbacks[0].attempt != 1 {
		t.Errorf("expected first callback attempt 1, got %d", retryCallbacks[0].attempt)
	}
	if retryCallbacks[0].err != expectedErr {
		t.Errorf("expected first callback error %v, got %v", expectedErr, retryCallbacks[0].err)
	}
	if retryCallbacks[1].attempt != 2 {
		t.Errorf("expected second callback attempt 2, got %d", retryCallbacks[1].attempt)
	}
	if retryCallbacks[1].err != expectedErr {
		t.Errorf("expected second callback error %v, got %v", expectedErr, retryCallbacks[1].err)
	}
}

// TestDo_Logger 测试日志功能
func TestDo_Logger(t *testing.T) {
	ctx := context.Background()
	attempts := 0
	mockLogger := &MockLogger{}

	err := Do(ctx, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary error")
		}
		return nil
	}, WithMaxRetries(5), WithLogger(mockLogger))

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
	if mockLogger.InfoCallCount() != 2 {
		t.Errorf("expected 2 info calls, got %d", mockLogger.InfoCallCount())
	}
	if !mockLogger.HasInfo("retrying") {
		t.Error("expected 'retrying' info message")
	}
	if mockLogger.ErrorCallCount() != 0 {
		t.Errorf("expected 0 error calls, got %d", mockLogger.ErrorCallCount())
	}
}

// TestDo_MaxElapsedTimeWithLogger 测试超过最大耗时并记录日志
func TestDo_MaxElapsedTimeWithLogger(t *testing.T) {
	ctx := context.Background()
	attempts := 0
	mockLogger := &MockLogger{}

	err := Do(ctx, func(ctx context.Context) error {
		attempts++
		time.Sleep(30 * time.Millisecond)
		return errors.New("temporary error")
	}, WithMaxRetries(5), WithMaxElapsedTime(100*time.Millisecond), WithInterval(10*time.Millisecond), WithLogger(mockLogger))

	if err == nil {
		t.Error("expected error, got nil")
	}
	if !mockLogger.HasInfo("retrying") {
		t.Error("expected 'retrying' info message")
	}
	if !mockLogger.HasError("retry aborted: max elapsed time exceeded") {
		t.Error("expected 'retry aborted: max elapsed time exceeded' error message")
	}
}

// TestOptions_DefaultValues 测试默认选项
func TestOptions_DefaultValues(t *testing.T) {
	o := defaultOptions()
	if o.maxRetries != 1 {
		t.Errorf("expected maxRetries 1, got %d", o.maxRetries)
	}
	if o.maxElapsedTime != 0 {
		t.Errorf("expected maxElapsedTime 0, got %v", o.maxElapsedTime)
	}
	if o.interval != 0 {
		t.Errorf("expected interval 0, got %v", o.interval)
	}
	if o.backoff == nil {
		t.Error("expected backoff not nil")
	}
	if o.retryIf == nil {
		t.Error("expected retryIf not nil")
	}
	if o.onRetry != nil {
		t.Error("expected onRetry nil")
	}
	if _, ok := o.logger.(*noopLogger); !ok {
		t.Errorf("expected logger to be *noopLogger, got %T", o.logger)
	}
}

// TestOptions_CustomValues 测试自定义选项
func TestOptions_CustomValues(t *testing.T) {
	customBackoff := func(attempt int) time.Duration {
		return time.Duration(attempt) * time.Second
	}
	customRetryIf := func(err error) bool {
		return err != nil
	}
	customOnRetry := func(attempt int, err error) {}
	customLogger := &MockLogger{}

	o := defaultOptions()
	WithMaxRetries(5)(o)
	WithMaxElapsedTime(30 * time.Second)(o)
	WithInterval(1 * time.Second)(o)
	WithBackoff(customBackoff)(o)
	WithRetryIf(customRetryIf)(o)
	WithOnRetry(customOnRetry)(o)
	WithLogger(customLogger)(o)

	if o.maxRetries != 5 {
		t.Errorf("expected maxRetries 5, got %d", o.maxRetries)
	}
	if o.maxElapsedTime != 30*time.Second {
		t.Errorf("expected maxElapsedTime 30s, got %v", o.maxElapsedTime)
	}
	if o.interval != 1*time.Second {
		t.Errorf("expected interval 1s, got %v", o.interval)
	}
	if o.backoff == nil {
		t.Error("expected backoff not nil")
	}
	if o.retryIf == nil {
		t.Error("expected retryIf not nil")
	}
	if o.onRetry == nil {
		t.Error("expected onRetry not nil")
	}
	if o.logger != customLogger {
		t.Errorf("expected logger to be customLogger, got %T", o.logger)
	}
}

// TestWithMaxRetries_InvalidValue 测试无效的最大重试次数
func TestWithMaxRetries_InvalidValue(t *testing.T) {
	o := defaultOptions()
	originalMaxRetries := o.maxRetries

	// 测试负值
	WithMaxRetries(-1)(o)
	if o.maxRetries != originalMaxRetries {
		t.Errorf("expected maxRetries to remain %d, got %d", originalMaxRetries, o.maxRetries)
	}

	// 测试零值
	WithMaxRetries(0)(o)
	if o.maxRetries != originalMaxRetries {
		t.Errorf("expected maxRetries to remain %d, got %d", originalMaxRetries, o.maxRetries)
	}
}

// TestWithMaxElapsedTime_InvalidValue 测试无效的最大耗时
func TestWithMaxElapsedTime_InvalidValue(t *testing.T) {
	o := defaultOptions()
	originalMaxElapsedTime := o.maxElapsedTime

	// 测试负值
	WithMaxElapsedTime(-1 * time.Second)(o)
	if o.maxElapsedTime != originalMaxElapsedTime {
		t.Errorf("expected maxElapsedTime to remain %v, got %v", originalMaxElapsedTime, o.maxElapsedTime)
	}
}
