# retry

一个功能强大的Go语言重试助手包，支持多种重试策略和灵活的配置选项。

## 特性

- 支持上下文取消
- 可配置最大重试次数
- 可配置最大总耗时
- 支持固定间隔重试
- 支持自定义退避策略
- 支持条件重试（基于错误类型）
- 支持重试前回调（用于监控和日志）
- 支持自定义日志记录器
- 支持从上下文获取当前重试次数
- 无第三方依赖

## 安装

```bash
go get github.com/im-wmkong/retry
```

## 快速开始

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "math/rand"
    "time"

    "github.com/im-wmkong/retry"
)

func main() {
    ctx := context.Background()
    // 初始化随机数种子
    rand.Seed(time.Now().UnixNano())

    err := retry.Do(ctx, func(ctx context.Context) error {
        attempt, ok := retry.AttemptFromContext(ctx)
        if !ok {
            return errors.New("无法获取重试次数")
        }
        fmt.Printf("尝试次数: %d\n", attempt)
        
        // 模拟一个有概率失败的操作（70%概率失败）
        if rand.Float32() < 0.7 {
            return errors.New("临时错误：服务暂时不可用")
        }
        
        return nil
    }, retry.WithMaxRetries(5), retry.WithInterval(1*time.Second))

    if err != nil {
        fmt.Printf("操作失败: %v\n", err)
    } else {
        fmt.Println("操作成功")
    }
}
```

## 配置选项

retry包提供了多种配置选项，可以通过函数式选项模式进行配置：

| 选项 | 描述 | 默认值 |
|------|------|--------|
| WithMaxRetries | 设置最大重试次数（>=1） | 1 |
| WithMaxElapsedTime | 设置最大允许的总重试耗时 | 0（无限制） |
| WithInterval | 设置固定重试间隔 | 0（无间隔） |
| WithBackoff | 设置自定义退避策略（优先级高于Interval） | 始终返回0 |
| WithRetryIf | 设置是否对特定错误重试 | 对所有非nil错误重试 |
| WithOnRetry | 设置每次重试前的回调 | nil |
| WithLogger | 设置日志实现 | noopLogger（不输出日志） |

## 示例

### 1. 使用固定间隔重试

```go
err := retry.Do(ctx, func(ctx context.Context) error {
    attempt, ok := retry.AttemptFromContext(ctx)
    if !ok {
        return errors.New("无法获取重试次数")
    }
    fmt.Printf("第 %d 次尝试\n", attempt)
    // 可能失败的操作
    return someOperation()
}, retry.WithMaxRetries(3), retry.WithInterval(500*time.Millisecond))
```

### 2. 使用指数退避策略

```go
err := retry.Do(ctx, func(ctx context.Context) error {
    attempt, ok := retry.AttemptFromContext(ctx)
    if !ok {
        return errors.New("无法获取重试次数")
    }
    fmt.Printf("第 %d 次尝试\n", attempt)
    // 可能失败的操作
    return someOperation()
}, retry.WithMaxRetries(5), retry.WithBackoff(func(attempt int) time.Duration {
    // 指数退避：10ms, 20ms, 40ms, 80ms, 160ms
    return time.Duration(10*attempt) * time.Millisecond
}))
```

### 3. 基于错误类型重试

```go
var (
    ErrTemporary = errors.New("临时错误")
    ErrPermanent = errors.New("永久错误")
    ErrAttemptNotFound = errors.New("无法获取重试次数")
)

err := retry.Do(ctx, func(ctx context.Context) error {
    attempt, ok := retry.AttemptFromContext(ctx)
    if !ok {
        return ErrAttemptNotFound
    }
    fmt.Printf("第 %d 次尝试\n", attempt)
    // 可能失败的操作
    return someOperation()
}, retry.WithMaxRetries(3), retry.WithRetryIf(func(err error) bool {
    // 只对临时错误重试
    return err == ErrTemporary
}))
```

### 4. 使用监控回调

```go
err := retry.Do(ctx, func(ctx context.Context) error {
    // 可能失败的操作
    return someOperation()
}, retry.WithMaxRetries(5), retry.WithOnRetry(func(attempt int, err error) {
    fmt.Printf("第 %d 次重试，错误: %v\n", attempt, err)
}))
```

### 5. 使用自定义日志记录器

```go
type CustomLogger struct{}

func (l *CustomLogger) Info(msg string, fields ...retry.Field) {
    fmt.Printf("[INFO] %s", msg)
    for _, f := range fields {
        fmt.Printf(" %s=%v", f.Key, f.Value)
    }
    fmt.Println()
}

func (l *CustomLogger) Error(msg string, fields ...retry.Field) {
    fmt.Printf("[ERROR] %s", msg)
    for _, f := range fields {
        fmt.Printf(" %s=%v", f.Key, f.Value)
    }
    fmt.Println()
}

var ErrAttemptNotFound = errors.New("无法获取重试次数")

err := retry.Do(ctx, func(ctx context.Context) error {
    attempt, ok := retry.AttemptFromContext(ctx)
    if !ok {
        return ErrAttemptNotFound
    }
    fmt.Printf("第 %d 次尝试\n", attempt)
    // 可能失败的操作
    return someOperation()
}, retry.WithMaxRetries(3), retry.WithLogger(&CustomLogger{}))
```

### 6. 上下文取消

```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

var ErrAttemptNotFound = errors.New("无法获取重试次数")

err := retry.Do(ctx, func(ctx context.Context) error {
    attempt, ok := retry.AttemptFromContext(ctx)
    if !ok {
        return ErrAttemptNotFound
    }
    fmt.Printf("第 %d 次尝试\n", attempt)
    // 长时间运行的操作
    time.Sleep(500 * time.Millisecond)
    return errors.New("临时错误")
}, retry.WithMaxRetries(10), retry.WithInterval(300*time.Millisecond))

// 当上下文超时时，重试会被取消
if err == context.DeadlineExceeded {
    fmt.Println("重试被超时取消")
}
```

## API参考

### func Do

```go
func Do(ctx context.Context, fn func(ctx context.Context) error, opts ...Option) error
```

执行带重试的函数。

- `ctx`：上下文，用于取消重试
- `fn`：要执行的函数，接受上下文参数并返回错误
- `opts`：配置选项

返回最终的错误或nil（如果成功）。

### func AttemptFromContext

```go
func AttemptFromContext(ctx context.Context) (int, bool)
```

从上下文中获取当前重试次数。

- `ctx`：上下文，由Do函数传递给回调函数

返回当前重试次数和是否成功获取到次数的布尔值。如果在Do函数的回调函数中调用，第二个返回值始终为true。

### type Option

```go
type Option func(*options)
```

函数式选项类型，用于配置重试行为。

### 选项函数

- `WithMaxRetries(n int)`：设置最大重试次数（>=1）
- `WithMaxElapsedTime(d time.Duration)`：设置最大允许的总重试耗时
- `WithInterval(d time.Duration)`：设置固定重试间隔
- `WithBackoff(fn func(attempt int) time.Duration)`：设置自定义退避策略
- `WithRetryIf(fn func(err error) bool)`：设置是否对特定错误重试
- `WithOnRetry(fn func(attempt int, err error))`：设置每次重试前的回调
- `WithLogger(l Logger)`：设置日志实现

### type Logger

```go
type Logger interface {
    Info(msg string, fields ...Field)
    Error(msg string, fields ...Field)
}
```

日志接口，用于记录重试信息。

### type Field

```go
type Field struct {
    Key   string
    Value any
}
```

结构化日志字段。

## 最佳实践

1. **始终使用上下文**：这样可以在需要时取消重试操作，避免资源浪费。

2. **合理设置最大重试次数**：根据操作的性质和失败率设置合适的重试次数，避免过多无效重试。

3. **使用退避策略**：对于网络请求等操作，使用指数退避可以减少服务器负载，提高成功率。

4. **区分临时错误和永久错误**：只对临时错误进行重试，避免对永久错误进行无意义的重试。

5. **添加监控和日志**：使用WithOnRetry回调和自定义日志记录器可以帮助监控重试行为，便于调试和优化。

6. **设置合理的超时时间**：使用WithMaxElapsedTime或上下文超时可以防止重试操作无限期执行。

7. **正确处理AttemptFromContext的返回值**：虽然在Do函数的回调中调用时总是会成功，但为了代码的健壮性，应该处理错误情况。

## 运行测试

```bash
go test -v
```

## 许可证

MIT
