package retry

import "context"

// Logger 抽象日志接口，避免绑定具体日志库
// 可以用 zap / logrus / slog 适配
type Logger interface {
	Debug(ctx context.Context, msg string, fields ...Field)
	Info(ctx context.Context, msg string, fields ...Field)
	Warn(ctx context.Context, msg string, fields ...Field)
	Error(ctx context.Context, msg string, fields ...Field)
}

// Field 结构化日志字段
type Field struct {
	Key   string
	Value any
}

// 简单默认实现（可选）
// 不设置 Logger 时不会输出任何日志
type noopLogger struct{}

func (n *noopLogger) Debug(context.Context, string, ...Field) {}
func (n *noopLogger) Info(context.Context, string, ...Field)  {}
func (n *noopLogger) Warn(context.Context, string, ...Field)  {}
func (n *noopLogger) Error(context.Context, string, ...Field) {}
