# Snow API 参考文档

Snow 是一个轻量级游戏服务端应用框架，提供完整的应用生命周期管理、依赖注入、配置管理、日志、指标等能力。

---

## pkg/host

应用框架核心，提供主机生命周期管理与依赖注入构建能力。

### 类型

| 类型 | 说明 |
|------|------|
| `IHost` | 主机接口，继承 IHostedRoutine，提供 GetRoutineProvider 获取服务提供者 |
| `IBuilder` | 构建器接口，提供获取集合、提供者、配置管理器以及 Build 方法 |
| `IHostApplication` | 应用生命周期接口，提供 OnStarted/OnStopped/OnStopping 和 StopApplication 方法 |
| `IHostedRoutine` | 托管routine接口，提供 Start/Stop 方法 |
| `IHostedLifecycleRoutine` | 带生命周期事件的托管routine，在 Start/Stop 基础上提供 BeforeStart/AfterStart/BeforeStop/AfterStop |

### 函数

```go
func Run(h IHost, ctx context.Context)
```
启动 Host 并阻塞等待应用停止信号。ctx 仅用于启动阶段，启动完成后阻塞等待 OnStopping 信号。

```go
func RunWithStopContext(h IHost, startCtx, stopCtx context.Context)
```
启动 Host 并阻塞等待应用停止信号，额外允许通过 stopCtx 控制停止阶段的超时预算。

```go
func GetRoutine[T any](provider injection.IRoutineProvider) T
```
从服务提供者获取指定类型的实例。

```go
func AddSingleton[U any](builder IBuilder) *injection.RoutineDescriptor
func AddVariantSingleton[T, U any](builder IBuilder) *injection.RoutineDescriptor
func AddSingletonFactory[U any](builder IBuilder, factory func(scope injection.IRoutineScope) U) *injection.RoutineDescriptor
```
单例服务注册函数，支持泛型注册和工厂方法注册。

```go
func AddScoped[U any](builder IBuilder) *injection.RoutineDescriptor
func AddTransient[U any](builder IBuilder) *injection.RoutineDescriptor
```
作用域和瞬态服务注册函数，变体形式支持 T/U 分离注册。

```go
func AddHostedRoutine[U IHostedRoutine](builder IBuilder)
func AddHostedLifecycleRoutine[U IHostedLifecycleRoutine](builder IBuilder)
```
添加托管 routine 到主机。

```go
func AddOption[T any](builder IBuilder, path string)
func AddLogHandler[T logging.ILogHandler](builder IBuilder, factory func() T)
func AddLogFilter[T logging.ILogFilter](builder IBuilder, factory func() T)
func AddLogFormatter(builder IBuilder, name string, formatter func(logData *logging.LogData) string)
```
选项、日志处理器、过滤器和格式化器的便捷注册函数。

```go
func Inject(scope injection.IRoutineScope, instance any) bool
func NewStruct[T any]() T
```
依赖注入辅助函数，Inject 执行构造方法依赖注入，NewStruct 创建实例。

```go
func NewDefaultBuilder() *DefaultBuilder
```
创建默认构建器（在 builder 子包中）。

---

## pkg/injection

依赖注入容器，提供三种服务生命周期模式。

### 类型

| 类型 | 说明 |
|------|------|
| `IRoutineCollection` | 服务描述符集合接口 |
| `IRoutineScope` | 作用域接口，提供 scoped 实例存取和方法 |
| `IRoutineProvider` | 服务提供者接口，提供 GetRoutine/CreateScope/GetRootScope |
| `RoutineDescriptor` | 服务描述符结构体，包含 Lifetime/Key/TyKey/TyImpl/Factory 字段 |
| `RoutineLifetime` | 生命期类型 (uint8) |

### 常量

```go
const (
    Singleton RoutineLifetime = iota  // 单例，整个应用生命周期共享
    Scoped                                     // 作用域，每个作用域创建新实例
    Transient                                  // 瞬态，每次请求创建新实例
)
var DefaultKey = struct{}{}              // 默认服务键
```

### 函数

```go
func GetRoutine[T any](provider IRoutineProvider) T
func GetKeyedRoutine[T any](provider IRoutineProvider, key any) T
```
从提供者获取服务实例，后者支持按键获取。

---

## pkg/configuration

配置管理系统，支持多源配置加载、分区访问和热更新。

### 类型

| 类型 | 说明 |
|------|------|
| `IConfiguration` | 配置接口，提供 Get/TryGet/Set/GetSection/GetChildren 等方法 |
| `IConfigurationRoot` | 根配置接口，继承 IConfiguration，增加 Reload/GetProviders 方法 |
| `IConfigurationSection` | 配置分区接口，继承 IConfiguration，增加 GetKey/GetPath/GetValue/SetValue |
| `IConfigurationManager` | 配置管理器接口，继承 IConfigurationBuilder 和 IConfigurationRoot |
| `IConfigurationBuilder` | 配置构建器接口，提供 GetProperties/GetSources/AddSource/BuildConfigurationRoot |
| `IConfigurationSource` | 配置源接口，提供 BuildConfigurationProvider 方法 |
| `IConfigurationProvider` | 配置提供者接口，提供 Get/TryGet/Set/GetReloadNotifier/Load/GetChildKeys |

### 函数

```go
func NewManager() *Manager
```
创建新的配置管理器实例。

```go
func Get[T any](root IConfiguration, path string) T
func Fill(root IConfiguration, path string, out any)
func GetBool(config IConfiguration, key string) bool
func GetInt64(config IConfiguration, key string) int64
func GetUint64(config IConfiguration, key string) uint64
func GetFloat64(config IConfiguration, key string) float64
```
泛型和基础类型获取函数，支持自动类型转换。

### 配置源

```go
type MemoryConfigurationSource    // 内存配置源
type JsonConfigurationSource      // JSON 配置源
type YamlConfigurationSource      // YAML 配置源
type EnvironmentConfigurationSource // 环境变量配置源
type FileConfigurationSource       // 文件配置源
```

---

## pkg/logging

结构化日志系统，支持 Handler/Filter 模式和日志级别过滤。

### 类型

| 类型 | 说明 |
|------|------|
| `ILogger` | 日志记录器接口，提供 Tracef/Debugf/Infof/Warnf/Errorf/Fatalf 方法 |
| `ILogHandler` | 日志处理器接口，提供 Log(data *LogData) 方法 |
| `ILogFilter` | 日志过滤器接口，提供 ShouldLog(level, name, path) bool 方法 |
| `LogData` | 日志数据结构体，包含 Time/NodeID/NodeName/Path/Name/ID/File/Level/Custom/Message 字段 |
| `DefaultLogger` | 默认日志记录器实现 |
| `LevelFilter` | 级别过滤器结构体，按最低级别过滤 |
| `Logger[T]` | 泛型日志记录器封装 |
| `LogFormatterContainer` | 日志格式化器容器 |

### 日志级别常量

```go
const (
    NONE  Level = iota
    TRACE
    DEBUG
    INFO
    WARN
    ERROR
    FATAL
)
```

### 函数

```go
func NewDefaultLogger(path string, handler ILogHandler, logDataBuilder func(data *LogData)) *DefaultLogger
```
创建默认日志记录器。

```go
func CombineFilter(filter ...ILogFilter) ILogFilter
```
组合多个过滤器，所有过滤器通过才记录。

```go
func DefaultLogFormatter(logData *LogData) string
func ColorLogFormatter(logData *LogData) string
```
默认和彩色两种日志格式化函数。

#### handler 子包

```go
type Handler struct {                    // compound 子包中的复合处理器
    proxy   []logging.ILogHandler
    filter []logging.ILogFilter
}

func NewHandler() *Handler
func (h *Handler) AddHandler(handler logging.ILogHandler)
func (h *Handler) AddFilter(filter logging.ILogFilter)

// console 子包
func NewHandler() *console.Handler

// file 子包
func NewHandler() *file.Handler
```

#### slog 子包

全局日志快捷调用：

```go
func Tracef(format string, args ...any)
func Debugf(format string, args ...any)
func Infof(format string, args ...any)
func Warnf(format string, args ...any)
func Errorf(format string, args ...any)
func Fatalf(format string, args ...any)

func BindGlobalHandler(h logging.ILogHandler)
func BindGlobalLogger(l logging.ILogger)
```

---

## pkg/metrics

指标系统，支持 Counter、Gauge、Histogram 三种指标类型。

### 类型

| 类型 | 说明 |
|------|------|
| `IMeterProvider` | 指标提供者接口，提供 Meter(name) 方法 |
| `IMeter` | 仪表接口，提供 Counter/Gauge/Histogram 方法 |
| `ICounter` | 计数器接口，提供 Add(value) 方法 |
| `IGauge` | 仪表接口，提供 Set/Add 方法 |
| `IHistogram` | 直方图接口，提供 Record(value) 方法 |
| `IMeterRegistry` | 指标注册中心接口，继承 IMeterProvider，提供 ForEach/Collect 方法 |
| `MetricData` | 指标数据结构体 |
| `MetricCollectorAdapter` | 指标收集器适配器，将 IMeterRegistry 适配为外部 collector 接口 |
| `PromCollector` | Prometheus 采集器实现 |
| `MetricType` | 指标类型枚举 |

### 常量

```go
const (
    MetricUnknown   MetricType = iota
    MetricCounter
    MetricGauge
    MetricHistogram
)

var DefaultHistogramBoundaries = []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000}
```

### 函数

```go
func NewMeterRegistry() IMeterRegistry
```
创建新的指标注册中心。

```go
func NewMetricCollectorAdapter(registry IMeterRegistry) *MetricCollectorAdapter
```
创建指标收集器适配器。

```go
func NewPromCollector(namespace string) *PromCollector
```
创建 Prometheus 采集器，带命名空间。

```go
func (p *PromCollector) FastHTTPHandler() fasthttp.RequestHandler
func (p *PromCollector) HTTPHandler() http.Handler
```
获取 HTTP 指标暴露处理器。

---

## pkg/option

选项模式实现，支持从配置或值绑定选项到结构体字段。

### 类型

| 类型 | 说明 |
|------|------|
| `Option[T]` | 泛型选项结构体，提供 Get/GetKeyed/OnChanged/OnKeyedChanged 方法 |
| `Repository` | 选项仓储结构体，管理类型到配置的绑定关系 |

### 函数

```go
func NewOptionRepository(config configuration.IConfiguration) *Repository
```
创建选项仓储，关联配置。

```go
func BindOptionPath[T any](repo *Repository, path string)
func BindKeyedOptionPath[T any](repo *Repository, key, path string)
func BindOptionValue[T any](repo *Repository, value T)
func BindKeyedOptionValue[T any](repo *Repository, key string, value T)
```
绑定选项到配置路径或直接值。

---

## pkg/xsync

同步工具，提供带超时的等待组。

### 类型

| 类型 | 说明 |
|------|------|
| `TimeoutWaitGroup` | 带超时的等待组结构体 |

### 函数

```go
func NewTimeoutWaitGroup() *TimeoutWaitGroup
```
创建新的超时等待组。

```go
func (wg *TimeoutWaitGroup) Add(n int) bool
func (wg *TimeoutWaitGroup) Done()
func (wg *TimeoutWaitGroup) Wait()
func (wg *TimeoutWaitGroup) WaitTimeout(dur time.Duration) bool
```
Add 增加计数，Done 减少计数，Wait 阻塞等待完成，WaitTimeout 支持超时退出。

---

## pkg/xnet

网络工具子包。

### 类型

| 类型 | 说明 |
|------|------|
| `IPreprocessor` | 连接预处理器接口，提供 Process(conn net.Conn) error 方法 |

#### transport 子包

TCP 和 WebSocket 监听配置与创建：

```go
type Config struct {
    TCPHost string
    TCPPort int
    WSHost  string
    WSPort  int
    WSPath  string
}

func NewListener(cfg *Config) ([]net.Listener, error)
```

---

## pkg/notifier

通知系统，轻量级的变更回调机制。

### 类型

| 类型 | 说明 |
|------|------|
| `INotifier` | 通知器接口，提供 RegisterNotifyCallback(callback func()) 方法 |

---

## pkg/task

异步任务执行，基于协程池实现。

### 函数

```go
func Execute(f func())
```
将函数提交到协程池异步执行，内部使用 ants 库维护大量协程。

---

## pkg/version

版本管理，支持语义化版本解析与比较。

### 类型

| 类型 | 说明 |
|------|------|
| `Version` | 版本结构体，包含 Major/Minor/Hotfix/Suffix/Prerelease/Build 字段 |

### 函数

```go
func CurrentVersion() *Version
func BuildVersion(verStr string) (*Version, bool)
func (v *Version) String() string
func (v *Version) Compatible(ver *Version) bool
func (v *Version) GreaterThan(ver *Version) bool
```
获取当前版本、解析版本字符串、格式化输出、版本兼容和大比较判断。

---

## 附录：包索引

| 包路径 | 功能 |
|--------|------|
| `github.com/gmbyte/snow/pkg/host` | 应用框架核心 |
| `github.com/gmbyte/snow/pkg/injection` | 依赖注入 |
| `github.com/gmbyte/snow/pkg/configuration` | 配置管理 |
| `github.com/gmbyte/snow/pkg/logging` | 日志框架 |
| `github.com/gmbyte/snow/pkg/logging/slog` | 全局日志快捷调用 |
| `github.com/gmbyte/snow/pkg/logging/handler/compound` | 复合日志处理器 |
| `github.com/gmbyte/snow/pkg/logging/handler/console` | 控制台日志处理器 |
| `github.com/gmbyte/snow/pkg/logging/handler/file` | 文件日志处理器 |
| `github.com/gmbyte/snow/pkg/metrics` | 指标系统 |
| `github.com/gmbyte/snow/pkg/option` | 选项模式 |
| `github.com/gmbyte/snow/pkg/xsync` | 同步工具 |
| `github.com/gmbyte/snow/pkg/xnet` | 网络工具 |
| `github.com/gmbyte/snow/pkg/xnet/transport` | TCP/WebSocket 传输 |
| `github.com/gmbyte/snow/pkg/notifier` | 通知系统 |
| `github.com/gmbyte/snow/pkg/task` | 任务执行 |
| `github.com/gmbyte/snow/pkg/version` | 版本管理 |
| `github.com/gmbyte/snow/pkg/task` | 任务执行 |
| `github.com/gmbyte/snow/pkg/version` | 版本管理 |