# Snow 优化方案与落地路线图

> 本文基于 聚焦 Snow 的可演进优化方案。目标是在保持 Snow「轻量、开箱即用」优势的同时，补齐治理能力与规模化短板。

## 1. 优化目标

- **稳定性目标**：降低异常中断和停机风险，提升可恢复能力。
- **性能目标**：减少高频路径的 GC 与反射开销，提升吞吐与延迟稳定性。
- **可观测性目标**：具备指标、日志、追踪三位一体的排障能力。
- **架构目标**：降低 `routines/node` 与 `core/host` 的耦合，提升模块可替换性。

## 2. 优先级总览

| 优先级 | 主题 | 产出 |
|--------|------|------|
| **P0** | 可靠性与治理基线 | context 传播、错误模型、优雅停机、基础指标 |
| **P1** | 性能与运行效率 | 序列化可插拔、对象池化、反射热点优化、日志背压 |
| **P2** | 架构演进 | Node 分层解耦、服务发现、测试与文档工具化 |

## 3. P0：可靠性与治理基线（优先立即推进）

### 3.1 RPC 全链路 `context.Context` 传播

**现状**
- `IRpcContext` 缺少标准 Context 访问，超时/取消/追踪难统一。

**改进建议**
- 在 `IRpcContext` 增加 `Context() context.Context`。
- Proxy.Call 支持传入父 Context，默认继承调用方上下文。
- 超时统一从 Context 派生，避免多套 timeout 机制并存。

**验收标准**
- 任一 RPC 可由上游取消并快速退出。
- 超时由 Context 控制后，日志/指标可关联同一 trace。

### 3.2 统一错误模型（错误码 + 包装）

**当前进展（已落地：基础版）**
- 已在 `routines/node` 增加统一错误模型：`ErrorCode` + `Error` 包装（支持 `errors.Is/As`）。
- 已定义核心错误码：`ErrTimeout`、`ErrServiceNotFound`、`ErrCodec`、`ErrTransport`、`ErrCancelled`、`ErrInvalidArgument`、`ErrInternal`。
- 已接入关键路径：
  - `proxy.go`：请求超时、取消、服务不存在等统一为带错误码的错误。
  - `proxy_http.go`：HTTP 传输/编解码错误统一包装。
  - `message.go`：错误序列化改为结构化 `code + msg`，远端可还原错误码。
  - `config.go` / `node.go`：启动阶段关键错误由返回 `error` 处理，减少对 `panic` 的依赖。
  - `handle.go`：连接断开、远端超时等统一错误码。

**兼容策略**
- RPC 仍保持 `ctx.Error(err)` 作为兼容层；推荐新代码使用显式 `error` 并附带 `ErrorCode`。

**验收结果（当前）**
- 线上日志/告警已具备按错误码聚合条件（已支持独立 `error_code` 字段，无需从错误字符串解析）。
- 主要异常路径已不依赖 `panic` 才能返回错误（初始化与 RPC 关键链路已覆盖）。

**后续增强（待办）**
- 将 `panic/recover` 保护块的输出也统一映射为结构化错误码。

### 3.3 优雅停机（Drain 模式）

**当前进展（已落地：基础版）**
- 已实现三阶段停机流程：
  - `拒绝新请求`：`Node.Stop()` 先进入 drain 模式，关闭监听并拒绝新的 TCP/HTTP 请求。
  - `等待进行中请求`：按可配置超时等待在途请求与会话清空。
  - `强制退出`：超时后输出剩余明细并继续执行原有 stop 流程。
- 已新增 drain 相关配置：
  - `StopDrainTimeoutSec`：等待在途请求最大秒数（默认 8）。
  - `StopDrainPollMs`：轮询间隔毫秒（默认 100）。
- 已新增在途统计与明细输出：
  - Service 级 in-flight request 计数。
  - remote handle pending session 计数。
  - drain 超时时日志输出 `remaining_requests / pending_sessions / per-service detail`。

**兼容策略**
- 不修改现有 Service 业务接口，Drain 逻辑由 Node/Handle 层托管。
- 支持依赖拓扑停机；若依赖图存在环路则自动回退到逆序停服，避免影响既有行为。

**验收结果（当前）**
- 停机期间可显式拒绝新请求，避免“边停机边接入”。
- 超时强退前可输出剩余请求/会话明细，满足可观测要求。
- 已支持按 `ServiceDependencies` 解析停机顺序（依赖方先停，被依赖方后停）。
- HTTP Drain 拒绝响应已结构化输出 `code + msg`，可直接用于聚合与检索。

**后续增强（待办）**
- 增加依赖拓扑配置合法性校验（启动期检测缺失依赖与环路详情）。
- 将 HTTP 错误结构统一升级为 `error_code + message + trace_id`。

### 3.4 基础可观测性落地

**当前进展（已落地）**

**一、内置 Prometheus 指标采集（可替换）**

- 新增 `core/metrics/PromCollector`，实现 `IMetricCollector` 接口（`Gauge` / `Counter` / `Histogram`）。
- 底层映射为三个 Prometheus 指标族，以 label `name` 区分逻辑指标：

| Prometheus 指标名 | 类型 | 说明 |
|-------------------|------|------|
| `snow_gauge{name}` | Gauge | 瞬时值（在途请求数等） |
| `snow_counter_total{name}` | Counter | 累计值（RPC 调用次数、错误次数、重连次数等） |
| `snow_duration_seconds{name}` | Histogram | 时延分布（自动纳秒→秒转换），支持 P50/P95/P99 聚合 |

- `Node.Construct` 中：若用户未提供 `RegisterOption.MetricCollector`，自动注入 `metrics.NewPromCollector("snow")`。
- 用户可通过 `RegisterOption.MetricCollector` 传入自定义实现来替换默认行为。

**二、最小指标集**

| 指标维度 | 采集点 | Prometheus label `name` 示例 |
|----------|--------|------------------------------|
| QPS（RPC 调用总数） | `Service.doDispatch` 入口 | `[ServiceRpc] <ServiceName>` |
| 请求时延（P95/P99） | `Service.doDispatch` request 完成时 | `[ServiceRequest] <ServiceName>::<Method>` |
| Post 时延 | `Service.doDispatch` post 完成时 | `[ServicePost] <ServiceName>::<Method>` |
| 主线程函数执行时延 | `Service.onTick` 中每次函数调用 | `[ServiceFunc] <ServiceName>::<Tag>` |
| 错误率 | `rpcContext.Error()` 触发时 | `[ServiceError] <ServiceName>` |
| 在途请求数（会话压力） | `doDispatch` in-flight 增减时 | `[ServiceInFlight] <ServiceName>` |
| 重连次数 | `nodeGetMessageSender` 连接成功/失败时 | `[NodeReconnectSuccess] <Addr>` / `[NodeReconnectFailed] <Addr>` |

**三、指标暴露（`/metrics` 端点）**

- `PromCollector` 实现了 `FastHTTPHandler() fasthttp.RequestHandler`。
- `Node.postInitOptions` 中自动检测 `MetricCollector` 是否提供该方法，若有则挂载 `GET /metrics` 到 Node HTTP 服务器。
- 启动后即可通过 `http://<NodeHost>:<HttpPort>/metrics` 被 Prometheus 抓取，无需额外代码。

**四、统一日志字段**

已在 RPC 核心错误日志中补齐结构化字段（key=value 格式）：

| 字段 | 来源 | 出现位置 |
|------|------|----------|
| `trace_id` | `message.trace` | `doDispatch` panic recover、函数名解析失败 |
| `service` | `Service.name` | 同上 |
| `method` | RPC 函数名 | 同上（有方法名时） |
| `peer` | `message.nAddr`（远端地址） | 同上 |
| `error_code` | `logging.ExtractErrorCode` 自动提取 | 所有日志行（DefaultLogFormatter 已内置输出 `error_code=<code>`） |

**五、Prometheus 抓取配置示例**

```yaml
# prometheus.yml
scrape_configs:
  - job_name: snow
    metrics_path: /metrics
    static_configs:
      - targets: ['127.0.0.1:8080']   # Node 的 HttpPort
    scrape_interval: 15s
```

**六、Grafana 常用查询**

| 用途 | PromQL |
|------|--------|
| 服务 RPC QPS | `rate(snow_counter_total{name=~"\\[ServiceRpc\\].*"}[1m])` |
| 请求时延 P95 | `histogram_quantile(0.95, rate(snow_duration_seconds_bucket{name=~"\\[ServiceRequest\\].*"}[5m]))` |
| 请求时延 P99 | `histogram_quantile(0.99, rate(snow_duration_seconds_bucket{name=~"\\[ServiceRequest\\].*"}[5m]))` |
| 错误率 | `rate(snow_counter_total{name=~"\\[ServiceError\\].*"}[5m]) / rate(snow_counter_total{name=~"\\[ServiceRpc\\].*"}[5m])` |
| 在途请求数 | `snow_gauge{name=~"\\[ServiceInFlight\\].*"}` |
| 重连次数（1h） | `increase(snow_counter_total{name=~"\\[NodeReconnect(Success\|Failed)\\].*"}[1h])` |

**兼容策略**

- `IMetricCollector` 接口不变，既有用户自定义实现无需修改。
- 不希望使用 Prometheus 时，可显式传入自定义实现或 `nil`（传 `nil` 时框架会自动填充默认实现；若确实不需要任何指标，可实现一个空的 `IMetricCollector`）。

**验收结果（当前）**

- 默认部署（不传 `MetricCollector`）即自动注入 Prometheus 采集，`GET /metrics` 可直接拉取。
- 通过上述 PromQL 可聚合各服务 QPS、P95/P99 时延、错误率，定位慢调用与异常服务。
- 日志行中 RPC 错误统一携带 `trace_id`、`service`、`method`、`peer`、`error_code`，可按维度检索。

**后续增强（待办）**

- 将 Histogram bucket 设为可配置（当前使用 Prometheus 默认桶 `DefBuckets`）。
- 增加 Node 级聚合指标（总连接数、消息队列长度）。
- 接入 OpenTelemetry Trace，实现 `trace_id` 跨节点传播与 Span 上报。

## 4. P1：性能与运行效率

### 4.1 编解码可插拔（ICodec）

**当前进展（已落地：基础版）**

**一、统一编解码接口**

已在 `routines/node/interf.go` 定义 `ICodec` 接口：

```go
type ICodec interface {
    Marshal(v any) ([]byte, error)
    Unmarshal(data []byte, v any) error
    Name() string
}
```

**二、默认 JSON 实现**

- 新增 `routines/node/codec.go`，提供 `JsonCodec`（基于 `xjson` / json-iterator），作为内置默认实现。
- `Node.Construct` 中：若用户未提供 `RegisterOption.Codec`，自动注入 `JsonCodec{}`。

**三、TCP RPC 已接入 ICodec**

`message.go` 中以下序列化路径已改为调用 `nodeCodec()`（运行时读取 Node 配置的 Codec 实例）：

| 调用点 | 用途 |
|--------|------|
| `marshalArgs()` | 请求/响应参数序列化 |
| `unmarshalArgs()` | 请求/响应参数反序列化 |
| `marshal()` 错误分支 | `wireError` 序列化 |
| `getError()` | `wireError` 反序列化 |

**四、HTTP RPC 保持 JSON**

HTTP RPC（`proxy_http.go` / `context_http.go` / `service.go` 中 `processHttpRpc`）始终使用 `xjson`，因 HTTP 协议语义绑定 `Content-Type: application/json`，不受 `ICodec` 配置影响。

**五、用户接入方式**

```go
// 自定义 Codec 示例：实现 ICodec 接口
type MsgPackCodec struct{}
func (MsgPackCodec) Marshal(v any) ([]byte, error)     { /* msgpack 编码 */ }
func (MsgPackCodec) Unmarshal(data []byte, v any) error { /* msgpack 解码 */ }
func (MsgPackCodec) Name() string                      { return "msgpack" }

// 注册时传入
node.AddNode(b, func() *node.RegisterOption {
    return &node.RegisterOption{
        Codec: MsgPackCodec{},
        // ...
    }
})
```

**兼容策略**

- 默认 `Codec = nil` 时自动注入 `JsonCodec{}`，行为与改造前完全一致。
- 切换 Codec 需确保集群所有节点统一升级，否则 TCP 消息体将无法互相解码。

**验收结果（当前）**

- TCP RPC 的编解码已可插拔，可通过 `RegisterOption.Codec` 一行替换为 MessagePack / Protobuf 等二进制 Codec。
- HTTP RPC 不受影响，保持 JSON 兼容性。
- 默认行为零变更，无需修改任何现有配置。

**后续增强（待办）**

- 提供官方 MessagePack / Protobuf 插件实现。
- 在基准压测中对比二进制 Codec 与 JSON 的 CPU 和延迟差异。

### 4.2 高频对象池化

**对象范围**
- `message`、`rpcContext`、`promise`、临时 `[]byte` buffer。

**改进建议**
- 使用 `sync.Pool` 管理生命周期；复用前清理状态，避免脏数据污染。
- 提供调试开关，便于排查池化相关问题。

**验收标准**
- GC 次数与内存分配速率显著下降，延迟抖动收敛。

### 4.3 反射热点优化

**现状**
- RPC 分发存在 `reflect.Value.Call` 高频开销。

**改进建议**
- 启动阶段构建 method dispatch 缓存（参数类型、调用桥）。
- 对热点方法提供类型安全包装（代码生成或泛型桥接）。

**验收标准**
- 热点 RPC 的平均耗时与 CPU 占比下降。

### 4.4 日志写入背压与缓冲治理

**当前进展（已落地）**

**一、缓冲容量配置化**

- `file.Option.MaxLogChanLength` 已支持配置（默认 102400），热更新时自动重建 channel。

**二、三种背压策略**

通过 `file.Option.BackpressureMode` 配置（默认 `BackpressureDrop`，与历史行为兼容）：

| 策略 | 枚举值 | 行为 | 适用场景 |
|------|--------|------|----------|
| `BackpressureDrop` | 0（默认） | channel 满时丢弃当前日志，计入丢弃计数 | 对延迟敏感、允许丢失低级别日志 |
| `BackpressureBlock` | 1 | channel 满时阻塞调用方，直到有空间 | 不允许任何日志丢失（如审计场景） |
| `BackpressureDropLow` | 2 | channel 满时仅保留 >= `DropMinLevel` 的日志（默认 WARN），低级别丢弃 | 高峰时优先保证告警/错误日志落盘 |

配置示例（YAML）：

```yaml
Logging:
  File:
    MaxLogChanLength: 204800
    BackpressureMode: 2       # DropLow
    DropMinLevel: 4           # WARN（TRACE=1, DEBUG=2, INFO=3, WARN=4, ERROR=5）
```

**三、丢弃计数与告警**

- 每条被丢弃的日志按级别计入原子计数器 `dropStats`（零锁开销）。
- 后台协程每 30 秒检查：若该周期内有丢弃，输出汇总到 stderr，格式如：
  ```
  file log dropped 1523 entries in last 30s: DEBUG=1200 INFO=323
  ```
- 暴露 `Handler.DroppedTotal() int64` 和 `Handler.DroppedSnapshot() [6]int64` 方法，外部可接入 `IMetricCollector` 周期上报。

**兼容策略**

- 默认 `BackpressureMode = 0`（Drop），行为与改造前一致，无需修改任何现有配置。
- 原来 channel 满时输出的 `"file log channel full"` 单行 stderr 升级为带计数、带级别的周期汇总。

**验收结果（当前）**

- 高峰期日志行为可通过 `BackpressureMode` 显式选择，不再只有"静默丢弃"一种结果。
- 丢弃量可观测：stderr 周期输出 + 可编程 API 对接指标系统。
- `BackpressureBlock` 模式下保证零丢失（以调用方延迟为代价）。
- `BackpressureDropLow` 模式下高优先级日志（WARN/ERROR/FATAL）保证落盘。

## 5. P2：架构演进与工程化

### 5.1 Node 分层解耦

**目标**
- 让 `node` 的传输、路由、编解码、会话管理可替换，减少对 Host 强绑定。

**建议分层**
- `node/api`：业务可见接口（Service/Proxy/Context）
- `node/runtime`：调度与生命周期
- `node/transport`：tcp/http/memory
- `node/codec`：编解码
- `node/discovery`：可选服务发现

### 5.2 服务发现与动态路由（可选）

**建议**
- 增加静态表 + 注册中心双模式。
- 支持健康检查、剔除与自动恢复。

### 5.3 测试与文档工具化

**建议**
- 提供 Mock `IProxy`、`IRpcContext`。
- 提供测试节点启动器（单进程多节点/内存传输）。
- 提供 RPC 文档生成工具（含 HTTP RPC 的 OpenAPI 输出）。

## 6. 12 周实施路线图（建议）

| 周期 | 目标 | 关键交付 |
|------|------|----------|
| 第 1-2 周 | P0 设计冻结 | Context 方案、错误码规范、停机流程设计评审 |
| 第 3-5 周 | P0 开发联调 | Context 接入、错误模型落地、Drain 停机、基础指标 |
| 第 6 周 | P0 验收 | 压测与故障演练、回归修复 |
| 第 7-9 周 | P1 开发 | ICodec、对象池化、反射缓存、日志背压 |
| 第 10 周 | P1 验收 | 基准测试对比报告 |
| 第 11-12 周 | P2 启动 | Node 分层设计稿、PoC 与迁移方案 |

## 7. 风险与回滚策略

- **协议兼容风险**：编解码切换需支持双栈解码和灰度开关。
- **池化引入脏数据风险**：统一 reset 规范 + race 测试。
- **停机流程改造风险**：保留旧停机逻辑开关，支持一键回退。
- **链路追踪开销风险**：采样率可配，默认低采样上线。

## 8. 成功判定（可量化）

- P0 完成后：故障恢复时间下降，停机错误率下降，关键指标齐备。
- P1 完成后：基准吞吐提升、P99 降低、GC 压力下降。
- P2 启动后：核心模块边界明确，新增传输/编解码成本下降。

---

*建议将本文作为 Snow 的迭代基线文档，按版本持续补充“已完成项”和“指标实测结果”。*
