# Snow

一个轻量级、模块化的 Go 分布式服务框架，专为游戏服务器等高并发场景设计。

## 特性

- **依赖注入**：基于接口的 IoC 容器，支持 Singleton / Scoped / Transient 三种生命周期
- **配置管理**：多配置源（JSON / YAML / Memory / File），支持文件热更新
- **分布式 RPC**：TCP 二进制协议 + HTTP JSON 协议，透明代理，自动路由
- **日志系统**：分层设计（Logger → Handler → Formatter），支持控制台彩色输出、文件滚动、zstd 压缩
- **生命周期管理**：Routine 的完整生命周期（BeforeStart → Start → AfterStart → BeforeStop → Stop → AfterStop），所有阶段支持 `context.Context` 传播
- **高效调度**：goroutine 池（ants）、多 Worker 定时器池、三级时间轮
- **Promise 异步模型**：链式调用，支持 Then / Catch / Final / WithContext
- **Context 全链路传播**：RPC 超时、取消、Trace 统一由 `context.Context` 驱动，Service 停止时自动取消所有进行中的 RPC
- **版本管理**：SemVer 语义化版本解析与比较，支持 prerelease / build 元数据
- **自动服务注册**：泛型 `Register[T, U]()` 在 init() 中声明式注册，`RegisterService()` 一键挂载

## 架构概览

```
snow/
├── core/                          # 核心模块
│   ├── configuration/             # 配置系统
│   │   └── sources/               # 配置源（JSON、YAML、File、Memory）
│   ├── crontab/                   # Cron 表达式解析与调度
│   ├── debug/                     # 调试工具（堆栈信息）
│   ├── encrypt/dh/                # Diffie-Hellman 密钥交换
│   ├── host/                      # 应用宿主与生命周期管理
│   │   ├── builder/               # 默认 Host 构建器
│   │   └── internal/              # Host 内部实现
│   ├── injection/                 # 依赖注入容器
│   ├── kvs/                       # 全局键值存储
│   ├── logging/                   # 日志系统
│   │   ├── handler/               # 日志处理器
│   │   │   ├── compound/          # 复合 Handler
│   │   │   ├── console/           # 控制台 Handler
│   │   │   └── file/              # 文件 Handler（滚动 + 压缩）
│   │   └── slog/                  # 全局快捷日志
│   ├── math/                      # 数学泛型工具
│   ├── meta/                      # 元编程工具（NoCopy）
│   ├── notifier/                  # 变更通知器
│   ├── option/                    # 类型安全的选项注入
│   ├── task/                      # goroutine 池任务执行
│   ├── ticker/                    # 多 Worker 定时器池
│   ├── version/                   # 语义化版本（SemVer）管理
│   ├── xjson/                     # JSON 编解码封装（json-iterator）
│   ├── xnet/                      # 网络接口（Server、Preprocessor）
│   └── xsync/                     # 同步工具（TimeoutWaitGroup）
├── routines/                      # 内置 Routine
│   ├── ignore_input/              # 忽略标准输入（后台服务用）
│   └── node/                      # 分布式节点（RPC、消息、服务）
└── examples/                      # 示例
    ├── minimal/                   # 最小示例
    └── pingpong/                  # Ping-Pong RPC 示例
```

## 快速开始

### 安装

```bash
go get github.com/gmbytes/snow
```

### 最小示例

```go
package main

import (
    "context"
    "time"

    "github.com/gmbytes/snow/core/host"
    "github.com/gmbytes/snow/core/host/builder"
    "github.com/gmbytes/snow/core/logging/slog"
    "github.com/gmbytes/snow/core/xsync"
    "github.com/gmbytes/snow/routines/ignore_input"
)

var _ host.IHostedRoutine = (*clock)(nil)

type clock struct {
    closeChan chan struct{}
}

func (ss *clock) Start(_ context.Context, wg *xsync.TimeoutWaitGroup) {
    ss.closeChan = make(chan struct{})

    go func() {
        ticker := time.NewTicker(time.Second)
    loop:
        for {
            select {
            case <-ticker.C:
                h, m, s := time.Now().Clock()
                slog.Infof("Now => %02v:%02v:%02v", h, m, s)
            case <-ss.closeChan:
                break loop
            }
        }
    }()
}

func (ss *clock) Stop(_ context.Context, wg *xsync.TimeoutWaitGroup) {
    close(ss.closeChan)
}

func main() {
    b := builder.NewDefaultBuilder()
    host.AddHostedRoutine[*ignore_input.IgnoreInput](b)
    host.AddHostedRoutine[*clock](b)
    host.Run(b.Build())
}
```

### Ping-Pong RPC 示例

```go
// main.go
func main() {
    b := builder.NewDefaultBuilder()
    host.AddHostedRoutine[*ignore_input.IgnoreInput](b)

    host.AddOption[*node.Option](b, "Node")
    host.AddOptionFactory[*node.Option](b, func() *node.Option {
        return &node.Option{
            BootName: "MyNode",
            LocalIP:  "127.0.0.1",
            Nodes: map[string]*node.ElementOption{
                "MyNode": {
                    Services: []string{"Ping", "Pong"},
                },
            },
        }
    })
    node.AddNode(b, func() *node.RegisterOption {
        return &node.RegisterOption{
            ServiceRegisterInfos: []*node.ServiceRegisterInfo{
                node.CheckedServiceRegisterInfoName[ping](1, "Ping"),
                node.CheckedServiceRegisterInfoName[pong](2, "Pong"),
            },
        }
    })

    host.Run(b.Build())
}

// pong.go - 服务端
type pong struct{ node.Service }
func (ss *pong) Start(_ any)           { ss.EnableRpc() }
func (ss *pong) Stop(_ *sync.WaitGroup) {}
func (ss *pong) AfterStop()            {}
func (ss *pong) RpcHello(ctx node.IRpcContext, msg string) {
    ss.Infof("received: %s", msg)
    ctx.Return("pong")
}

// ping.go - 客户端
type ping struct {
    node.Service
    closeChan chan struct{}
    pongProxy node.IProxy
}
func (ss *ping) Start(_ any) {
    ss.closeChan = make(chan struct{})
    ss.pongProxy = ss.CreateProxy("Pong")

    go func() {
        ticker := time.NewTicker(3 * time.Second)
    loop:
        for {
            select {
            case <-ticker.C:
                ss.Fork("rpc", func() {
                    ss.pongProxy.Call("Hello", "ping").
                        Then(func(ret string) { ss.Infof("received: %s", ret) }).
                        Done()
                })
            case <-ss.closeChan:
                break loop
            }
        }
    }()
}
func (ss *ping) Stop(_ *sync.WaitGroup) { close(ss.closeChan) }
func (ss *ping) AfterStop()             {}
```

### 自动注册模式（推荐）

使用 `Register` 在包 init() 中声明式注册服务，简化启动代码：

```go
// my_service.go
package myservice

func init() {
    node.Register[MyService, *MyService]("MyService")
}

type MyService struct { node.Service }
func (ss *MyService) Start(_ any)           { ss.EnableRpc() }
func (ss *MyService) Stop(_ *sync.WaitGroup) {}
func (ss *MyService) AfterStop()            {}

// 带配置绑定的服务注册
func init() {
    node.Register[MyService, *MyService]("MyService", func(b host.IBuilder) {
        host.AddOption[*MyConfig](b, "MyService")
    })
}
```

启动时一键注册：

```go
func main() {
    b := builder.NewDefaultBuilder()
    host.AddOption[*node.Option](b, "Node")
    node.RegisterService(b)  // 自动注册所有 init() 中 Register 的服务
    host.Run(b.Build())
}
```

## 核心模块详解

### 1. 依赖注入 (`core/injection`)

支持三种生命周期：

| 生命周期   | 说明                       |
|-----------|---------------------------|
| Singleton | 全局单例，存储在根作用域      |
| Scoped    | 作用域单例，存储在创建的作用域 |
| Transient | 瞬态，每次获取创建新实例      |

通过 `Construct` 前缀方法自动注入依赖：

```go
func (s *MyService) ConstructLogger(logger logging.ILogger) {
    s.logger = logger
}

func (s *MyService) ConstructOption(opt option.Option[MyOption]) {
    s.opt = opt
}
```

### 2. 配置系统 (`core/configuration`)

支持多种配置源，后注册的优先级更高：

```go
b.ConfigurationManager().AddSource(
    sources.NewYamlConfigurationSource("config.yaml", false, true),
)
```

| 配置源    | 说明                         |
|----------|------------------------------|
| Memory   | 内存配置                      |
| JSON     | JSON 文件（支持注释）           |
| YAML     | YAML 文件                     |
| File     | 文件监听基类，支持热更新（fsnotify） |

配置绑定到结构体：

```go
type ServerConfig struct {
    Host string `snow:"host"`
    Port int    `snow:"port"`
}
```

### 3. 日志系统 (`core/logging`)

分层架构：

```
Logger → RootHandler → CompoundHandler → ConsoleHandler (彩色输出)
                                       → FileHandler    (滚动 + 压缩)
```

日志级别：`TRACE < DEBUG < INFO < WARN < ERROR < FATAL`

文件 Handler 特性：
- 异步写入（channel 缓冲）
- 按时间 / 大小自动滚动
- 支持 zstd 压缩归档
- 文件名模板：`%Y_%M_%D_%h_%m_%i`

### 4. 生命周期管理 (`core/host`)

所有生命周期方法接收 `context.Context` 和 `*xsync.TimeoutWaitGroup`：

```go
// 基础 Routine
type IHostedRoutine interface {
    Start(ctx context.Context, wg *xsync.TimeoutWaitGroup)
    Stop(ctx context.Context, wg *xsync.TimeoutWaitGroup)
}

// 完整生命周期 Routine
type IHostedLifecycleRoutine interface {
    IHostedRoutine
    BeforeStart(ctx context.Context, wg *xsync.TimeoutWaitGroup)
    AfterStart(ctx context.Context, wg *xsync.TimeoutWaitGroup)
    BeforeStop(ctx context.Context, wg *xsync.TimeoutWaitGroup)
    AfterStop(ctx context.Context, wg *xsync.TimeoutWaitGroup)
}
```

启动方式：

```go
b := builder.NewDefaultBuilder()
host.AddHostedRoutine[*MyRoutine](b)
host.Run(b.Build())  // 阻塞运行，Context 驱动停机
```

`host.Run()` 内部通过 `context.WithCancel` 管理应用生命周期，停机时 Context 自动取消，所有 Routine 响应退出。

### 5. 分布式节点 (`routines/node`)

#### 消息协议

TCP 二进制协议：

```
| 长度 4B | src 4B | dst 4B | sess 4B | trace 8B | 数据 ... |
```

消息类型：
- **请求** (`sess > 0`)：函数名 + JSON 参数
- **Post** (`sess == 0`)：单向通知，无响应
- **响应** (`sess < 0`)：JSON 返回值
- **Ping** (`dst == 0`)：连接保活

#### Service 线程模型

每个 Service 通过 Ticker 池实现**单线程消息处理**，保证线程安全：

```
消息到达 → 放入 msgBuffer → onTick 触发 → doDispatch 路由到 RPC 方法
```

Service 拥有独立的 `context.Context`（继承自 `Node.ctx`），停止时自动取消。

#### Proxy 模式

透明代理，自动判断本地 / 远程调用：

```go
proxy := s.CreateProxy("TargetService")

// 基本调用 —— 自动继承 Service 生命周期 Context，默认 30s 超时
proxy.Call("MethodName", arg1, arg2).
    Then(func(result string) { /* 处理结果 */ }).
    Done()

// 自定义超时 —— 通过 context.WithTimeout 控制
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
proxy.Call("MethodName", arg1).WithContext(ctx).
    Then(func(result string) { /* 处理结果 */ }).
    Done()
```

#### HTTP RPC

除 TCP 协议外，还支持 HTTP JSON 协议的 RPC 调用：

- 每个 Service 自动注册 HTTP 端点（路径格式：`/rpc/<ServiceName>`）
- `HttpRpc` 前缀方法可被 HTTP 请求调用
- HTTP Proxy 使用 fasthttp 客户端，默认 8s 超时
- 支持 HTTPS（TLS）

#### Context 全链路传播

RPC 的超时与取消统一由 `context.Context` 驱动，无多套 timeout 机制并存：

```
Node.ctx (根 Context，WithCancel)
    └─ Service.ctx (继承 Node.ctx)
         └─ proxy.doCall 默认父级
              └─ message.ctx 传递到被调方（本地调用）
                   └─ rpcContext.ctx (WithCancel 派生)
                        └─ IRpcContext.Context() 暴露给 Handler
                             └─ handler 可传递给下游 RPC / DB / HTTP
```

**三级 Context 回退**：`WithContext(ctx)` 显式传入 → `Service.ctx` 生命周期 → `context.Background()`

**调用方**无需改动，自动获得 context 能力：
```go
// 日常调用：自动继承 Service context，Service 停止时所有 RPC 立即取消
proxy.Call("Hello", "ping").Then(func(ret string) { ... }).Done()
```

**被调方** Handler 签名不变，可选使用 `ctx.Context()`：
```go
func (s *pong) RpcHello(ctx node.IRpcContext, msg string) {
    // ctx.Context() 可传给 DB、HTTP、下游 RPC 等需要 context 的操作
    result, err := db.QueryContext(ctx.Context(), "SELECT ...")
    ctx.Return(result)
}
```

**链路传播**：上游取消 → 下游自动取消：
```go
func (s *myService) RpcProcess(rpcCtx node.IRpcContext, data string) {
    s.downstream.Call("Work", data).
        WithContext(rpcCtx.Context()).  // 继承上游 context
        Then(func(r string) { rpcCtx.Return(r) }).
        Catch(func(err error) { rpcCtx.Error(err) }).
        Done()
}
```

#### 服务注册

两种注册方式：

**方式一：手动注册**（适合快速原型）
```go
node.AddNode(b, func() *node.RegisterOption {
    return &node.RegisterOption{
        ServiceRegisterInfos: []*node.ServiceRegisterInfo{
            node.CheckedServiceRegisterInfoName[ping](1, "Ping"),
            node.CheckedServiceRegisterInfoName[pong](2, "Pong"),
        },
    }
})
```

**方式二：自动注册**（推荐，Kind 自动分配）
```go
// 各服务包 init() 中声明
func init() {
    node.Register[MyService, *MyService]("MyService")
}

// main.go 中一键注册
node.RegisterService(b)
```

`RegisterService` 自动收集所有 `Register` 调用，执行各服务的 setup 回调，并调用 `AddNode` 完成注册。

#### 停机依赖顺序（`ServiceDependencies`）

当启用 Drain 停机时，可在 `RegisterOption` 中声明服务依赖，框架会按依赖关系计算停机顺序（依赖方先停，被依赖方后停）。

```go
node.AddNode(b, func() *node.RegisterOption {
    return &node.RegisterOption{
        ServiceRegisterInfos: []*node.ServiceRegisterInfo{
            node.CheckedServiceRegisterInfoName[gateway](1, "Gateway"),
            node.CheckedServiceRegisterInfoName[world](2, "World"),
            node.CheckedServiceRegisterInfoName[db](3, "DB"),
        },
        // A: [B] 表示 A 依赖 B（停机时 A 会先于 B 停止）
        ServiceDependencies: map[string][]string{
            "Gateway": {"World"},
            "World":   {"DB"},
        },
    }
})
```

若依赖图存在环路，框架会自动回退为历史行为（逆序停机）并输出告警日志。

#### 动态地址管理

`AddrUpdater` 支持运行时动态更新节点地址，适用于服务发现场景：

```go
updater := node.NewNodeAddrUpdater(nAddr, func(ch chan<- node.Addr) {
    // 从注册中心获取最新地址，写入 ch
})
updater.Start()
currentAddr := updater.GetNodeAddr()
```

### 6. 时间轮 (`routines/node/timewheel`)

三级时间轮（小时 / 分钟 / 秒），O(1) 插入和触发：

```go
// 循环定时（每 3 秒）
handle := s.CreateTickItem(3*time.Second, func() { /* ... */ })

// 单次延迟（5 秒后）
handle := s.CreateAfterItem(5*time.Second, func() { /* ... */ })

// 取消
handle.Stop()
```

### 7. 版本管理 (`core/version`)

SemVer 语义化版本解析、比较与兼容性检查：

```go
ver, ok := version.BuildVersion("1.2.3-beta+build.123")
// ver.Major=1, ver.Minor=2, ver.Hotfix=3, ver.Prerelease="beta", ver.Build="build.123"

current := version.CurrentVersion()  // 编译期注入的版本号
buildTime := version.BuildTime()     // 构建时间

// 兼容性检查（Major.Minor 相同即兼容）
ver1.Compatible(ver2)

// 版本比较
ver1.GreaterThan(ver2)
```

数据库版本管理（用于数据迁移）：

```go
dbVer := version.GetAppCurrentDBVersion()
```

### 8. JSON 工具 (`core/xjson`)

基于 `json-iterator/go` 的高性能 JSON 封装，框架内部统一使用：

```go
data, err := xjson.Marshal(obj)
err := xjson.Unmarshal(data, &obj)
str, err := xjson.MarshalToString(obj)
err := xjson.UnmarshalFromString(str, &obj)
```

### 9. Crontab (`core/crontab`)

支持标准 Cron 表达式和宏：

```go
expr, _ := crontab.Parse("*/5 * * * *")        // 每 5 分钟
expr, _ := crontab.Parse("0 0 * * * * *")       // 每小时（含秒字段）
expr, _ := crontab.Parse("@daily")              // 每天
nextTime := expr.Normalize(time.Now())          // 下次触发时间
```

## 配置文件示例

```yaml
# config.yaml
Host:
  StartWaitTimeoutSeconds: 10
  StopWaitTimeoutSeconds: 15

Logging:
  Console:
    Level: DEBUG
  File:
    Level: INFO
    Path: "./logs"
    MaxSize: 104857600  # 100MB

Node:
  LocalIP: "127.0.0.1"
  BootName: MyNode
  HttpKeepAliveSeconds: 60
  HttpTimeoutSeconds: 30
  Nodes:
    MyNode:
      Order: 1
      Host: "127.0.0.1"
      Port: 8000
      HttpPort: 8080
      Services:
        - Ping
        - Pong
```

## 依赖

| 依赖                        | 用途               |
|-----------------------------|--------------------|
| panjf2000/ants/v2           | Goroutine 池        |
| valyala/fasthttp            | 高性能 HTTP 服务器    |
| fsnotify/fsnotify           | 文件系统事件监听      |
| json-iterator/go            | 高性能 JSON 编解码    |
| klauspost/compress          | zstd 压缩            |
| gopkg.in/yaml.v3            | YAML 解析            |
| go-strip-json-comments      | JSON 注释移除         |
| arl/assertgo                | 断言工具             |
| stretchr/testify            | 测试框架             |

## License

详见 [LICENSE](LICENSE) 文件。

---

## 优化建议

以下是基于代码分析提出的框架优化建议：

### 一、架构层面

#### 1. ~~引入 Context 传播机制~~ (已实现)

已完成 RPC 全链路 `context.Context` 传播：

- `IRpcContext` 新增 `Context() context.Context`，Handler 可获取关联 Context
- `IPromise` 新增 `WithContext(ctx)` 用于显式绑定 Context（自定义超时 / 上游取消传播）
- `Service` 拥有生命周期 Context（从 `Node.ctx` 派生），停止时自动取消所有进行中的 RPC
- 超时统一由 `context.WithTimeout` 控制，移除原有 `Timeout()` / `srv.After()` / `http.Client.Timeout` 多套机制
- 调用方三级回退：`WithContext(ctx)` → `Service.ctx` → `context.Background()`
- 本地 RPC 通过 `message.ctx` 传递 Context，远程 RPC 使用 `context.Background()` 兜底

#### 2. ~~自动服务注册~~ (已实现)

已完成泛型自动注册机制：

- `Register[T, U]()` 在包 init() 中声明式注册，Kind 自动分配
- `RegisterService(b)` 一键收集并注册所有服务
- 支持可选 setup 回调（用于绑定 Option 配置等构建期操作）
- `GetRegisteredService()` 获取已注册服务名列表

#### 3. 错误处理标准化

当前错误处理主要依赖 `panic/recover`，建议：
- 定义统一的错误码体系（如 `ErrServiceNotFound`、`ErrTimeout`、`ErrCodec`）
- RPC 方法支持返回 `error` 作为标准返回值，而非仅通过 `ctx.Error()` 
- 减少 panic 的使用，用返回值替代

#### 4. 服务发现与注册中心

当前节点地址是静态配置 + `AddrUpdater` 动态更新，建议进一步引入服务发现机制：
- 支持 etcd / Consul / ZooKeeper 等注册中心
- 支持动态节点加入和退出
- 支持健康检查和自动摘除

### 二、性能层面

#### 5. 消息序列化优化

当前使用 JSON 序列化 RPC 参数（通过 `xjson` 封装），对于高频调用场景开销较大。建议：
- 支持 Protocol Buffers 或 MessagePack 等二进制序列化
- 提供序列化器接口，允许用户自定义：

```go
type ICodec interface {
    Marshal(v any) ([]byte, error)
    Unmarshal(data []byte, v any) error
}
```

#### 6. 对象池化

以下对象频繁创建和销毁，建议使用 `sync.Pool` 池化：
- `message` 结构体
- `rpcContext` / `httpRpcContext` 结构体
- `[]byte` 缓冲区（handle.go 中的读写缓冲）
- `promise` 结构体

#### 7. 减少反射使用

当前 RPC 分发大量使用 `reflect.Value.Call()`，性能开销显著。建议：
- 启动时通过代码生成或泛型生成类型安全的调用包装
- 对高频方法使用类型断言替代反射调用

#### 8. 文件日志优化

`file_handler.go` 中 channel 缓冲区大小固定为 512，高并发时可能成为瓶颈。建议：
- 支持配置 channel 缓冲区大小
- 考虑使用 ring buffer 替代 channel
- 添加写入背压机制，避免日志丢失

### 三、可靠性层面

#### 9. 连接管理增强

`remoteHandle` 的重连机制较为简单，建议：
- 实现指数退避重连策略
- 添加连接池支持，避免单点连接瓶颈
- 支持连接的优雅关闭（drain 模式）
- 增加心跳超时检测（当前只有发送端 ping，缺少接收端超时判定）

#### 10. 会话管理改进

当前会话超时检查间隔为固定 10 秒（`handle.go`），建议：
- 使用时间轮管理会话超时，提高精度和效率
- 支持每个 RPC 调用自定义超时时间
- 添加会话数量限制，防止内存泄漏

#### 11. 优雅停机

当前停机使用固定超时，建议：
- 支持等待进行中的 RPC 调用完成
- 支持 drain 模式（停止接受新请求，处理完已有请求后关闭）
- 停止顺序应支持依赖排序（依赖的服务后停止）

### 四、可观测性层面

#### 12. 指标收集完善

当前定义了 `IMetricCollector` 接口但缺乏内置实现。建议：
- 提供 Prometheus metrics 内置实现
- 添加关键指标：RPC QPS、延迟分布、错误率、连接数、消息队列长度
- 支持 pprof endpoint（当前已支持通过 `ProfileListenHost` 配置开启）

#### 13. 链路追踪

当前消息中有 `trace` 字段但未充分利用。建议：
- 集成 OpenTelemetry
- 自动传播 trace context
- 支持 span 的创建和上报

### 五、开发体验层面

#### 14. 配置验证

当前配置系统缺乏校验机制，建议：
- 支持配置项的类型校验和范围校验
- 启动时校验必填配置项
- 配置变更时校验新值合法性

#### 15. RPC 接口文档自动生成

当前 RPC 方法通过 `Rpc` / `HttpRpc` 前缀约定，建议：
- 提供工具自动扫描并生成 API 文档
- 支持 HTTP RPC 的 Swagger / OpenAPI 文档生成

#### 16. 测试支持

当前框架缺乏测试辅助工具，建议：
- 提供 mock 工具（mock IProxy、mock IRpcContext）
- 提供集成测试框架（快速启动 / 停止测试节点）
- 为 Service 提供独立的单元测试运行环境
