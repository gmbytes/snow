# Snow

一个轻量级、模块化的 Go 分布式服务框架，专为游戏服务器等高并发场景设计。

## 特性

- **依赖注入**：基于接口的 IoC 容器，支持 Singleton / Scoped / Transient 三种生命周期
- **配置管理**：多配置源（JSON / YAML / Memory / File），支持文件热更新
- **分布式 RPC**：TCP 二进制协议 + HTTP JSON 协议，透明代理，自动路由
- **日志系统**：分层设计（Logger → Handler → Formatter），支持控制台彩色输出、文件滚动、zstd 压缩
- **生命周期管理**：Routine 的完整生命周期（BeforeStart → Start → AfterStart → BeforeStop → Stop → AfterStop）
- **高效调度**：goroutine 池（ants）、多 Worker 定时器池、三级时间轮
- **Promise 异步模型**：链式调用，支持 Then / Catch / Final / Timeout

## 架构概览

```
snow/
├── core/                          # 核心模块
│   ├── configuration/             # 配置系统
│   │   └── sources/               # 配置源（JSON、YAML、File、Memory）
│   ├── constraints/               # Go 泛型类型约束
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
│   ├── maps/                      # Map 泛型工具
│   ├── math/                      # 数学泛型工具
│   ├── meta/                      # 元编程工具（NoCopy）
│   ├── net/                       # 网络预处理接口
│   ├── notifier/                  # 变更通知器
│   ├── option/                    # 类型安全的选项注入
│   ├── sync/                      # 同步工具（TimeoutWaitGroup）
│   ├── task/                      # goroutine 池任务执行
│   └── ticker/                    # 多 Worker 定时器池
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
    "github.com/gmbytes/snow/core/host"
    "github.com/gmbytes/snow/core/host/builder"
    "github.com/gmbytes/snow/core/logging/slog"
    "github.com/gmbytes/snow/core/xsync"
    "github.com/gmbytes/snow/routines/ignore_input"
)

type clock struct {
    host.HostedRoutine
    logger  logging.ILogger
    stopWg  sync.TimeoutWaitGroup
}

func (s *clock) ConstructLogger(logger logging.ILogger) {
    s.logger = logger
}

func (s *clock) Start() {
    s.stopWg.Add(1)
    task.Execute(func() {
        defer s.stopWg.Done()
        ticker := time.NewTicker(time.Second)
        defer ticker.Stop()
        for {
            select {
            case <-ticker.C:
                slog.Infof("current time: %v", time.Now())
            case <-s.stopWg.C():
                return
            }
        }
    })
}

func (s *clock) Stop() {
    s.stopWg.Done()
}

func main() {
    b := builder.NewDefaultBuilder()
    host.AddHostedRoutine[ignore_input.IgnoreInput](b)
    host.AddHostedRoutine[clock](b)
    b.Build().Run()
}
```

### Ping-Pong RPC 示例

```go
// main.go
func main() {
    b := builder.NewDefaultBuilder()
    host.AddHostedRoutine[ignore_input.IgnoreInput](b)
    host.AddHostedRoutine[node.Node](b)

    node.CheckedServiceRegisterInfo[ping, *ping](PingKind).
        SetName("Ping").SetConstructor(func() *ping { return &ping{} })
    node.CheckedServiceRegisterInfo[pong, *pong](PongKind).
        SetName("Pong").SetConstructor(func() *pong { return &pong{} })

    b.Build().Run()
}

// pong.go - 服务端
type pong struct{ node.Service }
func (s *pong) Start(arg []byte) { s.EnableRpc() }
func (s *pong) RpcHello(ctx node.IRpcContext, name string) {
    ctx.Return("pong")
}

// ping.go - 客户端
type ping struct{ node.Service }
func (s *ping) Start(arg []byte) {
    proxy := s.CreateProxy("Pong")
    s.CreateTickItem(3*time.Second, func() {
        proxy.Call("Hello", "test").
            Then(func(reply string) { slog.Infof("reply: %s", reply) }).
            Catch(func(err error) { slog.Errorf("error: %v", err) }).
            Done()
    })
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

### 4. 分布式节点 (`routines/node`)

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

#### Proxy 模式

透明代理，自动判断本地 / 远程调用：

```go
proxy := s.CreateProxy("TargetService")
proxy.Call("MethodName", arg1, arg2).
    Then(func(result string) { /* 处理结果 */ }).
    Timeout(5 * time.Second).
    Done()
```

### 5. 时间轮 (`routines/node/timewheel`)

三级时间轮（小时 / 分钟 / 秒），O(1) 插入和触发：

```go
// 循环定时（每 3 秒）
handle := s.CreateTickItem(3*time.Second, func() { /* ... */ })

// 单次延迟（5 秒后）
handle := s.CreateAfterItem(5*time.Second, func() { /* ... */ })

// 取消
handle.Stop()
```

### 6. Crontab (`core/crontab`)

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
  Nodes:
    - Name: MyNode
      Order: 1
      Addr: "127.0.0.1:8000"
      Services:
        - Kind: 1
          Name: Ping
        - Kind: 2
          Name: Pong
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

## License

详见 [LICENSE](LICENSE) 文件。

---

## 优化建议

以下是基于代码分析提出的框架优化建议：

### 一、架构层面

#### 1. 引入 Context 传播机制

当前 RPC 调用缺乏标准的 `context.Context` 传播。建议在 `IRpcContext` 中集成 `context.Context`，支持跨服务的超时传播、取消信号传递和链路追踪：

```go
type IRpcContext interface {
    Context() context.Context  // 新增
    Return(args ...any)
    Error(err error)
}
```

#### 2. 错误处理标准化

当前错误处理主要依赖 `panic/recover`，建议：
- 定义统一的错误码体系（如 `ErrServiceNotFound`、`ErrTimeout`、`ErrCodec`）
- RPC 方法支持返回 `error` 作为标准返回值，而非仅通过 `ctx.Error()` 
- 减少 panic 的使用，用返回值替代

#### 3. 服务发现与注册中心

当前节点地址是静态配置的，建议引入服务发现机制：
- 支持 etcd / Consul / ZooKeeper 等注册中心
- 支持动态节点加入和退出
- 支持健康检查和自动摘除

### 二、性能层面

#### 4. 消息序列化优化

当前使用 JSON 序列化 RPC 参数，对于高频调用场景开销较大。建议：
- 支持 Protocol Buffers 或 MessagePack 等二进制序列化
- 提供序列化器接口，允许用户自定义：

```go
type ICodec interface {
    Marshal(v any) ([]byte, error)
    Unmarshal(data []byte, v any) error
}
```

#### 5. 对象池化

以下对象频繁创建和销毁，建议使用 `sync.Pool` 池化：
- `message` 结构体
- `rpcContext` 结构体
- `[]byte` 缓冲区（handle.go 中的读写缓冲）
- `promise` 结构体

#### 6. 减少反射使用

当前 RPC 分发大量使用 `reflect.Value.Call()`，性能开销显著。建议：
- 启动时通过代码生成或泛型生成类型安全的调用包装
- 对高频方法使用类型断言替代反射调用

#### 7. 文件日志优化

`file_handler.go` 中 channel 缓冲区大小固定为 512，高并发时可能成为瓶颈。建议：
- 支持配置 channel 缓冲区大小
- 考虑使用 ring buffer 替代 channel
- 添加写入背压机制，避免日志丢失

### 三、可靠性层面

#### 8. 连接管理增强

`remoteHandle` 的重连机制较为简单，建议：
- 实现指数退避重连策略
- 添加连接池支持，避免单点连接瓶颈
- 支持连接的优雅关闭（drain 模式）
- 增加心跳超时检测（当前只有发送端 ping，缺少接收端超时判定）

#### 9. 会话管理改进

当前会话超时检查间隔为固定 10 秒（`handle.go`），建议：
- 使用时间轮管理会话超时，提高精度和效率
- 支持每个 RPC 调用自定义超时时间
- 添加会话数量限制，防止内存泄漏

#### 10. 优雅停机

当前停机使用固定超时（5s/8s），建议：
- 支持等待进行中的 RPC 调用完成
- 支持 drain 模式（停止接受新请求，处理完已有请求后关闭）
- 停止顺序应支持依赖排序（依赖的服务后停止）

### 四、可观测性层面

#### 11. 指标收集完善

当前定义了 `IMetricCollector` 接口但缺乏内置实现。建议：
- 提供 Prometheus metrics 内置实现
- 添加关键指标：RPC QPS、延迟分布、错误率、连接数、消息队列长度
- 支持 pprof endpoint

#### 12. 链路追踪

当前消息中有 `trace` 字段但未充分利用。建议：
- 集成 OpenTelemetry
- 自动传播 trace context
- 支持 span 的创建和上报

### 五、开发体验层面

#### 13. 配置验证

当前配置系统缺乏校验机制，建议：
- 支持配置项的类型校验和范围校验
- 启动时校验必填配置项
- 配置变更时校验新值合法性

#### 14. RPC 接口文档自动生成

当前 RPC 方法通过 `Rpc` / `HttpRpc` 前缀约定，建议：
- 提供工具自动扫描并生成 API 文档
- 支持 HTTP RPC 的 Swagger / OpenAPI 文档生成

#### 15. 测试支持

当前框架缺乏测试辅助工具，建议：
- 提供 mock 工具（mock IProxy、mock IRpcContext）
- 提供集成测试框架（快速启动 / 停止测试节点）
- 为 Service 提供独立的单元测试运行环境
