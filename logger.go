package retry

// Logger 抽象日志接口，避免绑定具体日志库
// 你可以用 zap / logrus / slog 适配
type Logger interface {
	Info(msg string, fields ...Field)
	Error(msg string, fields ...Field)
}

// Field 结构化日志字段
type Field struct {
	Key   string
	Value any
}

// 简单默认实现（可选）
// 不设置 Logger 时不会输出任何日志
type noopLogger struct{}

func (n *noopLogger) Info(string, ...Field)  {}
func (n *noopLogger) Error(string, ...Field) {}
