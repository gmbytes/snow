# snow 测试指南

> 版本：v1.0 | 日期：2026-03-04

本文档介绍 snow 项目的测试策略、分层结构与最佳实践。

## 1. 测试分层

snow 采用清晰的测试分层，每层有明确的职责边界：

| 位置 | 类型 | 说明 |
|------|------|------|
| `pkg/*/*_test.go` | 包单元测试 | 直接构造 pkg 类型的公开 API，验证功能正确性 |
| `internal/*/*_test.go` | 内部实现测试 | 可以测试未导出的类型和函数，覆盖实现细节 |
| `test/integration/` | 集成测试 | 通过 host.NewBuilder() 验证多模块协作（未来规划）|
| `test/benchmark/` | 性能基准 | 使用 -bench 标志运行，评估关键路径性能（未来规划）|

**分层原则**：测试代码位于被测代码相同包内，共享同包访问权限，无需导出示例类型。

## 2. 命名规范

测试函数命名遵循 `TestXxx_WhenYyy_ExpectZzz` 模式，清晰表达测试场景与预期结果：

```go
func TestOption_Get_WhenNotSet_ExpectDefaultValue(t *testing.T)
func TestLogger_Info_WithFields_ExpectFormattedOutput(t *testing.T)
func TestConfiguration_GetInt_MissingKey_ExpectZero(t *testing.T)
```

**好处**：测试失败时，名称本身就是错误描述的一部分，便于快速定位问题。

## 3. 同步原则

测试中的异步操作同步是常见陷阱，snow 明确规范同步方式：

### 3.1 禁止做法

❌ **禁止使用 `time.Sleep()` 用于测试同步**

```go
// 错误的同步方式
func TestX(t *testing.T) {
    go doAsyncWork()
    time.Sleep(time.Second) // 不要这样做！
    // 假设此时工作已完成
}
```

- Sleep 时间难以确定，太短会 flaky，太长拖慢测试
- CI 环境差异可能导致本来通过的测试失败

### 3.2 推荐做法

✅ **使用以下方式进行同步**：

#### Channel 同步

```go
func TestAsyncWork_Done_ExpectCalled(t *testing.T) {
    done := make(chan struct{})
    
    go func() {
        doAsyncWork()
        close(done)
    }()
    
    select {
    case <-done:
        // 工作完成
    case <-time.After(time.Second):
        t.Fatal("timeout")
    }
}
```

#### Context Cancel

```go
func TestWithContext_Cancel_ExpectCleanup(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    
    go func() {
        doAsyncWork(ctx)
    }()
    
    cancel()
    // 验证清理逻辑被执行
}
```

#### WaitGroup

```go
func TestParallelWorkers_Complete_ExpectAllDone(t *testing.T) {
    var wg sync.WaitGroup
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            doWork()
        }()
    }
    wg.Wait()
    // 所有 worker 完成
}
```

#### Select + Timeout

```go
func TestChannel_Receive_ExpectWithinTimeout(t *testing.T) {
    resultCh := make(chan int)
    
    go func() {
        resultCh <- compute()
    }()
    
    select {
    case result := <-resultCh:
        assert.Equal(t, 42, result)
    case <-time.After(500 * time.Millisecond):
        t.Fatal("operation timeout")
    }
}
```

## 4. Mock 模式

### 4.1 日志 Handler Mock

测试日志相关功能时，使用 mutex + slice 记录日志数据进行断言：

```go
type mockHandler struct {
    mu    sync.Mutex
    logs  []logging.LogData
}

func (h *mockHandler) Handle(ctx context.Context, data logging.LogData) {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.logs = append(h.logs, data)
}

func (h *mockHandler) getLogs() []logging.LogData {
    h.mu.Lock()
    defer h.mu.Unlock()
    return slices.Clone(h.logs)
}
```

用法：

```go
func TestSomeFeature_Logging_ExpectInfoLevel(t *testing.T) {
    mock := &mockHandler{}
    
    logger := logging.NewLogger(logging.WithHandlers(mock))
    logger.Info("test message", "key", "value")
    
    logs := mock.getLogs()
    require.Len(t, logs, 1)
    require.Equal(t, logging.LevelInfo, logs[0].Level)
    require.Equal(t, "test message", logs[0].Message)
}
```

### 4.2 文件系统 Fake

需要模拟文件系统操作时，使用结构体字段注入错误或计数，避免复杂 fake FS：

```go
type fakeFileWriter struct {
    WriteFunc func(p []byte) (n int, err error)
    CallCount int
}

func (f *fakeFileWriter) Write(p []byte) (n int, err error) {
    f.CallCount++
    if f.WriteFunc != nil {
        return f.WriteFunc(p)
    }
    return len(p), nil
}
```

这样可以在测试中精确控制返回错误或统计调用次数。

### 4.3 临时目录

避免 mock 文件系统，尽可能使用真实临时目录：

```go
func TestFileHandler_Rotation_ExpectNewFile(t *testing.T) {
    tmpDir := t.TempDir()
    
    handler := file.NewFileHandler(file.WithDirectory(tmpDir))
    // 测试日志轮转
}
```

- `t.TempDir()` 自动创建临时目录，测试结束后自动清理
- 测试的是真实文件系统行为，更可靠

## 5. 覆盖率目标

snow 追求合理的测试覆盖率，以下为目标：

| 模块 | 目标覆盖率 | 当前状态 |
|------|------------|----------|
| **整体** | 70%+ | - |
| **logging** | 85%+ | 91.6% |
| **configuration** | 85%+ | 已有 8 个测试文件 |

核心模块（logging、configuration）是框架基础设施，覆盖率应当更高，以保证整个系统的稳定性。

查看当前覆盖率：

```bash
make coverage
```

这会生成 coverage.out 并打印汇总信息。

## 6. 运行命令

项目提供统一的测试 Makefile：

```bash
# 全量测试（随机顺序，禁止缓存）
make test

# 竞态检测（强烈推荐每次合入前运行）
make test-race

# 覆盖率报告
make coverage

# 性能基准测试
make bench

# 完整 CI 检查（lint + test + test-race）
make ci
```

### 命令说明

| 命令 | 作用 |
|------|------|
| `make test` | 运行所有测试，启用 -shuffle=on 打乱顺序，-count=1 禁用缓存 |
| `make test-race` | 使用 -race 检测数据竞争，超时设为 3 分钟 |
| `make coverage` | 以 atomic 模式统计覆盖率，生成 coverage.out |
| `make bench` | 运行所有 Benchmark，-run=^$$ 确保只跑基准 |
| `make ci` | 依次执行 lint、test、test-race，全面质量检查 |

## 7. 测试技巧

### 7.1 表驱动测试

对于有多组输入输出的场景，使用表驱动写法减少重复：

```go
func TestOption_String_VariousInputs(t *testing.T) {
    tests := []struct {
        name     string
        value    string
        expected string
    }{
        {"empty string", "", ""},
        {"normal string", "hello", "hello"},
        {"unicode", "你好", "你好"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            opt := option.NewOption[string]("test", option.WithDefault(tt.value))
            got := opt.Get()
            require.Equal(t, tt.expected, got)
        })
    }
}
```

### 7.2 Subtests 组织

使用 t.Run 划分嵌套场景：

```go
func TestConfiguration_Nested(t *testing.T) {
    t.Run("get existing key", func(t *testing.T) { /* ... */ })
    t.Run("get missing key", func(t *testing.T) { /* ... */ })
    t.Run("watch change", func(t *testing.T) { /* ... */ })
}
```

- 单独运行子测试：`-run TestConfiguration_Nested/get_existing_key`
- 子测试并行：t.Parallel()

### 7.3 断言库

推荐使用 stretchr/testify：

```go
import (
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/assert"
)
```

- require：断言失败立即终止测试
- assert：断言失败继续执行

通常使用 require，只有在需要收集多个失败时才用 assert。

## 8. 相关资源

- [架构文档](ARCHITECTURE.md)
- [迁移文档：从 core/ 到 pkg/internal/](MIGRATION_PKG_INTERNAL.md)