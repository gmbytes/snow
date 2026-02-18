# Snow

一个轻量级、模块化的 Go 分布式服务框架，专为游戏服务器等高并发场景设计。

## 特性

- **依赖注入**：基于接口的 IoC 容器，支持 Singleton / Scoped / Transient 三种生命周期
- **配置管理**：多配置源（JSON / YAML / Memory / File），支持文件热更新
- **分布式 RPC**：TCP 二进制协议 + HTTP JSON 协议，透明代理，自动路由
- **编解码可插拔**：TCP RPC 参数序列化通过 `ICodec` 接口抽象，默认 JSON，可替换为 MessagePack / Protobuf 等
- **日志系统**：分层设计（Logger → Handler → Formatter），支持控制台彩色输出、文件滚动、zstd 压缩、**三种背压策略**（Drop / Block / DropLow）
- **生命周期管理**：Routine 的完整生命周期（BeforeStart → Start → AfterStart → BeforeStop → Stop → AfterStop），所有阶段支持 `context.Context` 传播
- **优雅停机**：三阶段 Drain 模式（拒绝新请求 → 等待在途 → 强制退出），支持按服务依赖拓扑排序停机
- **高效调度**：goroutine 池（ants）、多 Worker 定时器池、三级时间轮
- **Promise 异步模型**：链式调用，支持 Then / Catch / Final / WithContext
- **Context 全链路传播**：RPC 超时、取消、Trace 统一由 `context.Context` 驱动，Service 停止时自动取消所有进行中的 RPC
- **统一错误模型**：结构化 `ErrorCode` + `Error` 包装，支持 `errors.Is/As`，线上日志可按错误码聚合
- **内置可观测性**：默认 Prometheus 指标采集（QPS、P95/P99、错误率、在途请求数、重连次数），自动暴露 `/metrics` 端点
- **服务发现**：`IServiceDiscovery` 接口支持 etcd / Consul 等注册中心接入，与静态配置表双模式共存；内置 `/health` 端点供探活
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
│   ├── metrics/                   # 指标采集（内置 Prometheus 实现）
│   ├── xjson/                     # JSON 编解码封装（json-iterator）
│   ├── xnet/                      # 网络接口（Server、Preprocessor）
│   └── xsync/                     # 同步工具（TimeoutWaitGroup）
├── routines/                      # 内置 Routine
│   ├── ignore_input/              # 忽略标准输入（后台服务用）
│   └── node/                      # 分布式节点（RPC、消息、服务）
└── examples/                      # 示例
    ├── minimal/                   # 最小示例
    ├── pingpong/                  # Ping-Pong RPC 示例
    └── discovery/                 # 服务发现示例
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
- 异步写入（channel 缓冲，容量可配置，默认 102400）
- 按时间 / 大小自动滚动
- 支持 zstd 压缩归档
- 文件名模板：`%Y_%M_%D_%h_%m_%i`
- 三种背压策略（`BackpressureMode`）：
  - `Drop`（默认）：channel 满时丢弃，计入丢弃计数
  - `Block`：channel 满时阻塞调用方，保证零丢失
  - `DropLow`：channel 满时仅保留 >= `DropMinLevel`（默认 WARN）的日志
- 丢弃统计：按级别原子计数，每 30s 输出汇总到 stderr；暴露 `DroppedTotal()` / `DroppedSnapshot()` API 供外部上报

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
- **请求** (`sess > 0`)：函数名 + 参数（通过 `ICodec` 序列化，默认 JSON）
- **Post** (`sess == 0`)：单向通知，无响应
- **响应** (`sess < 0`)：返回值（通过 `ICodec` 序列化）
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

`AddrUpdater` 支持运行时动态更新节点地址，适用于轻量级服务发现场景：

```go
updater := node.NewNodeAddrUpdater(nAddr, func(ch chan<- node.Addr) {
    // 从注册中心获取最新地址，写入 ch
})
updater.Start()
currentAddr := updater.GetNodeAddr()
```

#### 服务发现（`IServiceDiscovery`）

通过 `RegisterOption.ServiceDiscovery` 可接入 etcd / Consul / ZooKeeper 等注册中心，与静态配置表双模式共存：

```go
type IServiceDiscovery interface {
    Resolve(serviceName string) (INodeAddr, error)
    Deregister(nodeAddr INodeAddr, services []string)
}
```

- `CreateProxy("Name")` 优先通过 Discovery 解析，失败时回退静态表
- 停机时自动调用 `Deregister` 注销服务
- 内置 `GET /health` 端点（正常 200，Drain 中 503），供注册中心探活

未配置时行为与静态表完全一致，零 breaking change。

**完整示例**（见 `examples/discovery/`）：

```go
// discovery.go — 实现 IServiceDiscovery 接口
type MapDiscovery struct {
    mu       sync.RWMutex
    registry map[string]node.INodeAddr
}

func NewMapDiscovery() *MapDiscovery {
    return &MapDiscovery{registry: make(map[string]node.INodeAddr)}
}

// Register 注册服务地址（模拟注册中心写入）
func (d *MapDiscovery) Register(serviceName, host string, port int) error {
    addr, err := node.NewNodeAddr(host, port)
    if err != nil {
        return err
    }
    d.mu.Lock()
    d.registry[serviceName] = addr
    d.mu.Unlock()
    return nil
}

// Resolve 实现 node.IServiceDiscovery
func (d *MapDiscovery) Resolve(serviceName string) (node.INodeAddr, error) {
    d.mu.RLock()
    defer d.mu.RUnlock()
    if addr, ok := d.registry[serviceName]; ok {
        return addr, nil
    }
    return nil, fmt.Errorf("service %q not found", serviceName)
}

// Deregister 实现 node.IServiceDiscovery，停机时由框架自动调用
func (d *MapDiscovery) Deregister(nodeAddr node.INodeAddr, services []string) {
    d.mu.Lock()
    defer d.mu.Unlock()
    for _, name := range services {
        delete(d.registry, name)
    }
}
```

```go
// main.go — 注入 ServiceDiscovery
func main() {
    disc := NewMapDiscovery()
    disc.Register("Pong", "127.0.0.1", 8000)

    b := builder.NewDefaultBuilder()
    host.AddHostedRoutine[*ignore_input.IgnoreInput](b)
    host.AddOption[*node.Option](b, "Node")
    host.AddOptionFactory[*node.Option](b, func() *node.Option {
        return &node.Option{
            BootName: "MyNode",
            LocalIP:  "127.0.0.1",
            Nodes: map[string]*node.ElementOption{
                "MyNode": {
                    Port: 8000, HttpPort: 8080,
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
            ServiceDiscovery: disc, // 注入服务发现
        }
    })

    host.Run(b.Build())
}
```

```go
// ping.go — 调用方代码无需任何改动
func (ss *ping) Start(_ any) {
    // CreateProxy 内部自动走 Discovery.Resolve("Pong")，
    // 失败时回退静态表。对业务完全透明。
    ss.pongProxy = ss.CreateProxy("Pong")
    // ... 同普通 Ping-Pong 示例
}
```

启动后：
- `disc.Resolve("Pong")` 返回 `127.0.0.1:8000`，`CreateProxy` 据此路由
- `GET http://127.0.0.1:8080/health` 返回 `{"status":"ok"}`
- 停机时框架自动调用 `disc.Deregister()`，从注册表中摘除服务

生产环境只需将 `MapDiscovery` 替换为 etcd / Consul 实现即可，业务代码零变更。

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

### 8. 编解码可插拔 (`ICodec`)

TCP RPC 的参数序列化通过 `ICodec` 接口抽象，默认使用 JSON（`JsonCodec`，基于 `xjson`）。用户可通过 `RegisterOption.Codec` 替换为任意二进制编解码器：

```go
type ICodec interface {
    Marshal(v any) ([]byte, error)
    Unmarshal(data []byte, v any) error
    Name() string
}
```

```go
node.AddNode(b, func() *node.RegisterOption {
    return &node.RegisterOption{
        Codec: MyMsgPackCodec{},  // 替换为 MessagePack
        // ...
    }
})
```

- 未配置时自动注入 `JsonCodec{}`，行为与历史版本一致
- HTTP RPC 始终使用 JSON（因 HTTP Content-Type 语义绑定），不受 `ICodec` 影响

### 9. 统一错误模型

框架定义了结构化错误码体系（`ErrorCode` + `Error` 包装），支持 `errors.Is/As`：

| 错误码 | 含义 |
|--------|------|
| `ErrTimeout` | 请求超时 |
| `ErrServiceNotFound` | 服务未找到 |
| `ErrCodec` | 编解码错误 |
| `ErrTransport` | 传输层错误 |
| `ErrCancelled` | 请求被取消 |
| `ErrInvalidArgument` | 参数非法 |
| `ErrInternal` | 内部错误 |

远端 RPC 错误序列化为 `code + msg` 结构，接收端可还原错误码。日志自动提取 `error_code` 字段用于聚合。

### 10. 优雅停机（Drain 模式）

`Node.Stop()` 执行三阶段停机：

1. **拒绝新请求**：进入 drain 模式，关闭 TCP/HTTP 监听
2. **等待在途请求**：按 `StopDrainTimeoutSec`（默认 8s）轮询等待在途 RPC 和会话清空
3. **强制退出**：超时后输出剩余明细并继续 stop 流程

支持按 `ServiceDependencies` 声明依赖拓扑，框架自动计算停机顺序（依赖方先停，被依赖方后停）。

### 11. 指标与可观测性

启用 Node 且未自定义 `RegisterOption.MetricCollector` 时，框架自动注入内置 Prometheus 采集器，并在 Node 的 HTTP 端口暴露 **`GET /metrics`**。

核心指标：

| 指标 | Prometheus 名 | 说明 |
|------|---------------|------|
| RPC QPS | `snow_counter_total{name="[ServiceRpc] ..."}` | 各服务 RPC 调用总数 |
| 请求时延 | `snow_duration_seconds{name="[ServiceRequest] ..."}` | P50/P95/P99 聚合 |
| 错误率 | `snow_counter_total{name="[ServiceError] ..."}` | 按服务统计 |
| 在途请求数 | `snow_gauge{name="[ServiceInFlight] ..."}` | 实时会话压力 |
| 重连次数 | `snow_counter_total{name="[NodeReconnect...] ..."}` | 连接成功/失败 |

用户可通过 `RegisterOption.MetricCollector` 传入自定义实现替换 Prometheus。

完整 Prometheus 抓取配置与 Grafana 查询示例见 [snow_optimization.md - 3.4](snow_optimization.md)。

### 12. JSON 工具 (`core/xjson`)

基于 `json-iterator/go` 的高性能 JSON 封装，框架内部统一使用：

```go
data, err := xjson.Marshal(obj)
err := xjson.Unmarshal(data, &obj)
str, err := xjson.MarshalToString(obj)
err := xjson.UnmarshalFromString(str, &obj)
```

### 13. Crontab (`core/crontab`)

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
    MaxSize: 104857600        # 100MB
    MaxLogChanLength: 102400  # 日志 channel 缓冲区容量
    BackpressureMode: 0       # 0=Drop(默认) 1=Block 2=DropLow
    DropMinLevel: 4           # DropLow 模式下保留的最低级别（4=WARN）

Node:
  LocalIP: "127.0.0.1"
  BootName: MyNode
  HttpKeepAliveSeconds: 60
  HttpTimeoutSeconds: 30
  StopDrainTimeoutSec: 8     # 优雅停机等待在途请求超时（秒）
  StopDrainPollMs: 100       # 停机轮询间隔（毫秒）
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

| 依赖                           | 用途               |
|---------------------------------|--------------------|
| panjf2000/ants/v2              | Goroutine 池        |
| valyala/fasthttp               | 高性能 HTTP 服务器    |
| prometheus/client_golang       | Prometheus 指标采集   |
| fsnotify/fsnotify              | 文件系统事件监听      |
| json-iterator/go               | 高性能 JSON 编解码    |
| klauspost/compress             | zstd 压缩            |
| gopkg.in/yaml.v3               | YAML 解析            |
| go-strip-json-comments         | JSON 注释移除         |
| arl/assertgo                   | 断言工具             |
| stretchr/testify               | 测试框架             |

## License

详见 [LICENSE](LICENSE) 文件。

---

## 优化建议与进展

> 完整设计文档与实施细节见 [snow_optimization.md](snow_optimization.md)

### 已实现

| # | 主题 | 状态 | 说明 |
|---|------|------|------|
| 1 | Context 全链路传播 | **已实现** | `IRpcContext.Context()`、`IPromise.WithContext()`、三级回退、Service 生命周期 Context |
| 2 | 自动服务注册 | **已实现** | `Register[T,U]()` + `RegisterService(b)` 泛型声明式注册 |
| 3 | 统一错误模型 | **已实现** | 结构化 `ErrorCode` + `Error` 包装，关键路径已覆盖 |
| 4 | 优雅停机（Drain） | **已实现** | 三阶段停机 + 依赖拓扑排序 + 在途统计明细输出 |
| 5 | 基础可观测性 | **已实现** | 内置 Prometheus 采集 + `/metrics` 自动挂载 + 统一日志字段 |
| 6 | 编解码可插拔（ICodec） | **已实现** | TCP RPC 序列化接口化，默认 JSON，可替换 |
| 7 | 日志写入背压 | **已实现** | 三种背压策略 + 丢弃计数 + 周期告警 |
| 8 | 服务发现与动态路由 | **已实现** | `IServiceDiscovery` 接口 + 静态表双模式 + `/health` 探活 + 停机自动注销 |

### 待推进

| # | 主题 | 优先级 | 说明 |
|---|------|--------|------|
| 9 | 高频对象池化 | P1 | `message`/`rpcContext`/`promise`/`[]byte` 等 `sync.Pool` 池化 |
| 10 | 反射热点优化 | P1 | `reflect.Value.Call` 构建 dispatch 缓存，降低 CPU 开销 |
| 11 | 链路追踪（OpenTelemetry） | P2 | `trace_id` 跨节点传播 + Span 上报 |
| 12 | 连接管理增强 | P2 | 指数退避重连、连接池、心跳超时检测 |
| 13 | 配置验证 | P2 | 配置项类型/范围校验，启动期必填校验 |
| 14 | RPC 文档生成 | P2 | 自动扫描生成 API 文档、HTTP RPC OpenAPI |
| 15 | 测试支持 | P2 | Mock 工具 + 集成测试框架 + 单元测试环境 |
