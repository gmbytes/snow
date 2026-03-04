# snow 架构设计文档

> 版本：v1.0 | 日期：2026-03-04

snow 是一个基于 Go 语言的轻量级服务框架，提供了依赖注入、配置管理、日志、可观测性等基础能力，适用于构建微服务或游戏后端服务。

## 1. 模块结构概览

项目采用 `pkg/` + `internal/` 分层结构，明确区分公开 API 与内部实现：

```
snow/
├── pkg/                    # 稳定公开 API，对外承诺兼容性
│   ├── host/              # 生命周期编排：IHost、IBuilder、Add*/Run
│   │   └── builder/       # NewDefaultBuilder 默认构建器
│   ├── injection/         # 依赖注入：IRoutineCollection、IRoutineProvider、GetRoutine[T]
│   ├── logging/           # 日志核心：ILogger、ILogHandler、LogData、DefaultLogger
│   │   ├── slog/          # 全局日志 API（Infof/Errorf 等）
│   │   └── handler/       # Handler 实现
│   │       ├── compound/  # 复合 handler
│   │       ├── console/   # 控制台 handler
│   │       └── file/      # 文件 handler（含背压策略）
│   ├── configuration/     # 配置接口体系
│   │   └── sources/       # Yaml/Json/Memory/File 配置源
│   ├── notifier/          # 配置变更通知：INotifier
│   ├── option/            # 选项模式：Option[T] 泛型
│   ├── xnet/              # 网络预处理：IPreprocessor
│   │   └── transport/     # WebSocket 连接/监听器
│   ├── xsync/            # 同步原语：TimeoutWaitGroup
│   ├── metrics/          # 可观测性：PromCollector
│   ├── version/          # 版本解析：Version、CurrentVersion
│   └── task/             # 任务调度：Task 管理
│
├── internal/             # 内部实现，不承诺稳定性
│   ├── host/             # Host 内部实现（RoutineScope、Provider、Container）
│   ├── crontab/          # Cron 表达式解析
│   ├── debug/            # 堆栈信息收集
│   ├── encrypt/dh/      # DH 密钥交换
│   ├── kvs/             # 全局键值存储
│   ├── math/            # 泛型数学工具（Clamp、Abs）
│   ├── meta/            # NoCopy 标记类型
│   ├── ticker/          # 时间轮事件循环
│   ├── xhttp/          # HTTP 薄封装
│   └── xjson/          # JSON 薄封装
│
├── routine/             # 框架服务层
│   ├── node/            # RPC 节点框架（代理、上下文、编解码）
│   └── ignore_input/    # 忽略输入服务示例
│
└── examples/            # 示例工程
```

## 2. 依赖约束规则

snow 采用严格的层级依赖约束，保证公共 API 的稳定性：

| 来源 | 目标 | 允许 |
|------|------|------|
| `pkg/*` | `pkg/*` | ✅ 允许 |
| `pkg/*` | `internal/*` | ❌ 除 `pkg/host/builder` 需要引用 `internal/host` 实现外，其他禁止 |
| `internal/*` | `pkg/*` | ✅ 允许 |
| `internal/*` | `internal/*` | ✅ 允许（同一模块内） |
| `routine/*` | `pkg/*` | ✅ 允许 |
| `routine/*` | `internal/*` | ✅ 允许 |
| 外部消费者 | `pkg/*` | ✅ 允许 |
| 外部消费者 | `routines/*` | ✅ 允许 |
| 外部消费者 | `internal/*` | ❌ 禁止 |

**设计意图**：`pkg/` 是对外承诺兼容性的稳定层，`internal/` 是实现细节可能随时变化。外部项目（如 server）只能依赖 `pkg/` 和 `routines/`，不能依赖 `internal/`。

## 3. 核心设计模式

### 3.1 依赖注入（DI）

snow 提供完整的依赖注入容器，支持三种生命周期：

```go
// 定义服务
type MyService struct {
    Dep *Dependency
}

// 注册为单例
builder.AddSingleton(func() *MyService {
    return &MyService{Dep: GetRoutine[*Dependency]()}
})

// 获取服务（编译期安全）
svc := GetRoutine[*MyService](provider)
```

- **IRoutineCollection**：服务注册集合，支持 AddSingleton、AddScoped、AddTransient
- **IRoutineProvider**：服务提供者，从集合中解析实例
- **GetRoutine[T]**：泛型获取函数，自动类型推断

### 3.2 选项模式（Options）

配置项使用 Options 模式，支持默认值、热更新回调：

```go
opt := option.NewOption[string](
    "gs.name",
    option.WithDefault("unknown"),
    option.OnChanged(func(old, new string) {
        log.Info("gs name changed", "old", old, "new", new)
    }),
)

// 获取当前值
name := opt.Get()

// 设置值（触发 OnChanged）
opt.Set("game-gs-01")
```

- 每个 Option 绑定唯一配置路径
- 支持热更新：值变化时触发回调
- 类型安全：泛型消除类型断言

### 3.3 生命周期编排

服务实现两种接口以参与应用生命周期：

```go
// 基本生命周期
type IHostedRoutine interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}

// 扩展生命周期（可选）
type IHostedLifecycleRoutine interface {
    BeforeStart(ctx context.Context) error
    AfterStart(ctx context.Context) error
    BeforeStop(ctx context.Context) error
    AfterStop(ctx context.Context) error
}
```

启动顺序：BeforeStart → Start → AfterStart  
停止顺序：BeforeStop → Stop → AfterStop

### 3.4 日志管道

日志系统采用 Handler 链模式：

```
Logger.Info("msg", "key", "value")
        ↓
ILogHandler.Handle(context.Context, LogData)
        ↓
CompoundHandler.Dispatch (依次调用子 Handler)
        ↓
ConsoleHandler + FileHandler
```

- **ILogHandler**：日志处理接口，所有 Handler 实现此接口
- **CompoundHandler**：组合多个 Handler，依次分发
- **ConsoleHandler**：控制台输出，颜色分级
- **FileHandler**：文件输出，支持日志轮转、背压策略

### 3.5 配置分层

配置系统提供多层抽象：

```
IConfigurationSource (数据来源)
        ↓
IConfigurationProvider (提供配置查询)
        ↓
IConfigurationRoot (根节点，可遍历 Section)
        ↓
IConfigurationSection (配置节，Key-Value 访问)
```

典型用法：

```go
// 从 Builder 注册配置源
builder.AddConfigurationSource(yamlSource)

// 从 Host 获取配置根节点
root := host.Configuration().GetConfigurationRoot()

// 按路径获取值
section := root.GetSection("database")
ip := section.Get("host").String("")
port := section.Get("port").Int(3306)
```

## 4. 并发不变量

snow 在设计中严格遵守以下并发安全原则：

### 4.1 Ticker 单线程模型

时间轮（Ticker）为每个 Service 创建独立的 goroutine，但保证回调在同一 goroutine 中串行执行：

```go
// 每个 Service 的 Tick 在同一个 goroutine 中执行
// 无需加锁，服务内部状态天然安全
tk := ticker.NewTicker(serviceName)
tk.Start(func(event interface{}) {
    // 此回调在单 goroutine 中，不会并发
    handleEvent(event)
})
```

### 4.2 DI 容器单例初始化

使用 CAS 自旋锁保证单例只初始化一次：

```go
func (d *RoutineDescriptor) InitOnce(provider IRoutineProvider) {
    if !atomic.CompareAndSwapUint32(&d.initialized, 0, 1) {
        return // 已初始化
    }
    d.instance = d.constructor(provider)
}
```

### 4.3 文件日志异步写入

FileHandler 使用 channel + 后台 writer goroutine 实现异步写入：

```go
// 生产端：发送日志到 channel（非阻塞）
select {
case ch <- logData:
default:
// channel 满，执行背压策略
}

// 消费端：后台 goroutine 轮询写入
go func() {
    for data := range ch {
        writer.Write(data)
    }
}()
```

### 4.4 背压策略

当日志产生速率超过文件写入速率时，FileHandler 提供三种策略：

| 策略 | 行为 |
|------|------|
| **Drop** | 丢弃低优先级日志，保证高优先级日志写入 |
| **Block** | 阻塞生产者，等待写入完成（可能导致请求延迟） |
| **DropLow** | 只保留 Debug 级别之外的日志 |

### 4.5 slog 全局日志

全局 logger 使用 atomic.Pointer 保证并发安全：

```go
var globalLogger atomic.Pointer[ILogger]

func SetLogger(logger ILogger) {
    globalLogger.Store(&logger)
}

func Info(msg string, args ...interface{}) {
    if p := globalLogger.Load(); p != nil {
        (*p).Info(msg, args...)
    }
}
```

## 5. 启动流程

snow 应用的标准启动流程如下：

```
┌─────────────────────────────────────────────┐
│ 1. NewDefaultBuilder()                     │
│    ├─ 注册默认日志 Handler              │
│    ├─ 注册全局 Logger                   │
│    └─ 注册基础 DI 容器                   │
└────────────────┬────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────┐
│ 2. 用户配置阶段                           │
│    ├─ builder.RegisterService(...)      │
│    ├─ builder.AddConfigurationSource(..) │
│    └─ builder.AddOption(...)            │
└────────────────┬────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────┐
│ 3. builder.Build() → Host                │
│    └─ 实例化所有单例                     │
└────────────────┬────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────┐
│ 4. host.Run(ctx)                         │
│    ├─ 所有 HostedRoutine.BeforeStart   │
│    ├─ 所有 HostedRoutine.Start         │
│    ├─ 所有 HostedRoutine.AfterStart    │
│    └─ 等待系统信号（SIGINT/SIGTERM）    │
└────────────────┬────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────┐
│ 5. 收到退出信号                           │
│    ├─ 所有 HostedRoutine.BeforeStop    │
│    ├─ 所有 HostedRoutine.Stop          │
│    └─ 所有 HostedRoutine.AfterStop     │
└─────────────────────────────────────────────┘
```

完整示例：

```go
func main() {
    // 1. 创建默认构建器
    builder := host.NewDefaultBuilder()

    // 2. 注册服务
    builder.AddSingleton(func() *GameServer {
        return &GameServer{}
    })

    // 3. 添加配置源
    yamlSrc := sources.NewYamlConfigurationSource("config.yaml")
    builder.AddConfigurationSource(yamlSrc)

    // 4. 构建主机
    h, err := builder.Build()
    if err != nil {
        panic(err)
    }

    // 5. 运行（阻塞直到收到退出信号）
    h.Run(context.Background())
}
```

## 6. 相关资源

- [迁移文档：从 core/ 到 pkg/internal/](MIGRATION_PKG_INTERNAL.md)
- [测试文档](TESTING.md)