# snow pkg/internal 目录分层设计文档

> 版本：v1.0 | 日期：2026-03-04

## 1. 设计目标

将 `core/` 平铺结构迁移为 `pkg/` + `internal/` 分层：
- **pkg/**：稳定公开 API，对外承诺兼容性
- **internal/**：内部实现细节，不承诺稳定

## 2. 分类原则

1. **纯接口/抽象 → pkg/**：仅包含接口定义、类型别名、泛型辅助函数
2. **被外部直接导入 → pkg/**：server 项目直接 import 的包
3. **实现细节 → internal/**：具体实现、工具函数、第三方库薄封装
4. **特殊情况**：接口与实现混合的包，整体进 pkg/（拆分成本过高）

## 3. 逐包分类决策

### 3.1 进入 pkg/ 的包（稳定公开 API）

| 当前路径 | 目标路径 | 理由 |
|---------|---------|------|
| `core/host/` | `pkg/host/` | 核心接口 IHost、IBuilder，注册函数 AddSingleton/AddOption 等。被 server 直接导入 |
| `core/host/builder/` | `pkg/host/builder/` | NewDefaultBuilder()，server 启动入口直接使用 |
| `core/injection/` | `pkg/injection/` | 纯 DI 抽象：IRoutineCollection、IRoutineProvider、RoutineDescriptor、GetRoutine[T]()。被 host、builder 等广泛引用 |
| `core/logging/` | `pkg/logging/` | 核心接口 ILogger、ILogHandler、LogData 及 DefaultLogger。被所有模块引用 |
| `core/logging/slog/` | `pkg/logging/slog/` | 全局日志 API（Infof/Errorf 等），被 node、service 层广泛使用 |
| `core/logging/handler/` | `pkg/logging/handler/` | RootHandler（DI 容器注入桥梁），compound/console/file handler 均通过 builder 注册 |
| `core/logging/handler/compound/` | `pkg/logging/handler/compound/` | 复合 handler，host_builder 和 host_injection 直接引用 |
| `core/logging/handler/console/` | `pkg/logging/handler/console/` | 控制台 handler，builder 默认注册 |
| `core/logging/handler/file/` | `pkg/logging/handler/file/` | 文件 handler，builder 默认注册 |
| `core/configuration/` | `pkg/configuration/` | 接口体系 IConfigurationSource/Provider/Builder/Section/Root/Manager |
| `core/configuration/sources/` | `pkg/configuration/sources/` | YamlConfigurationSource 等，server 直接导入使用 |
| `core/notifier/` | `pkg/notifier/` | INotifier 接口，被 configuration 依赖 |
| `core/option/` | `pkg/option/` | Option[T] 泛型、IOptionInjector 接口，host_builder 和 logging handler 的 Construct 参数 |
| `core/xnet/` | `pkg/xnet/` | IPreprocessor 接口，server 的 pkg_processor 实现该接口 |
| `core/xnet/transport/` | `pkg/xnet/transport/` | WebSocket 连接/监听器，node 的 transport 层 |
| `core/xsync/` | `pkg/xsync/` | TimeoutWaitGroup，host 停机流程核心同步原语 |
| `core/metrics/` | `pkg/metrics/` | PromCollector（Prometheus 采集器），node 的 RegisterOption.MetricCollector 使用 |
| `core/version/` | `pkg/version/` | Version 解析、CurrentVersion()，node 启动时使用 |

### 3.2 进入 internal/ 的包（内部实现）

| 当前路径 | 目标路径 | 理由 |
|---------|---------|------|
| `core/host/internal/` | `internal/host/` | Host 内部实现（routine_provider、routine_collection、hosted_routine_container 等），已有 internal 标识 |
| `core/crontab/` | `internal/crontab/` | Cron 表达式解析，纯工具，无外部依赖方 |
| `core/debug/` | `internal/debug/` | StackInfo() 堆栈信息收集，仅 node 内部使用 |
| `core/encrypt/dh/` | `internal/encrypt/dh/` | DH 密钥交换，加密实现细节 |
| `core/kvs/` | `internal/kvs/` | 全局 KV 存储，仅 node 内部使用 |
| `core/math/` | `internal/math/` | Clamp/Abs 泛型数学工具 |
| `core/meta/` | `internal/meta/` | NoCopy 标记类型，仅 xsync 内部使用 |
| `core/task/` | `pkg/task/` | ants goroutine 池封装，被 server 项目直接使用（Execute 函数），因此归入 pkg/ |
| `core/ticker/` | `internal/ticker/` | 时间轮事件循环，仅 node service/handle 内部使用 |
| `core/xhttp/` | `internal/xhttp/` | fasthttp 薄封装，无外部使用 |
| `core/xjson/` | `internal/xjson/` | json-iterator 薄封装，仅 node codec/config 内部使用 |

### 3.3 routines/ 保持不变

| 路径 | 决策 | 理由 |
|------|------|------|
| `routines/node/` | 保持 `routines/node/` | RPC 节点框架，包含大量导出接口（IProxy、IRpcContext、IPromise 等），是框架的核心使用层。位置已在 `routines/` 下，与 `core/` 分离良好 |
| `routines/ignore_input/` | 保持 `routines/ignore_input/` | 简单 HostedRoutine 实现 |

## 4. 目标目录结构

```
snow/
├── pkg/                           # 稳定公开 API
│   ├── host/                      # IHost, IBuilder, Add*/Run 函数
│   │   └── builder/               # NewDefaultBuilder
│   ├── injection/                 # DI 抽象：IRoutineCollection/Provider, GetRoutine[T]
│   ├── logging/                   # ILogger, ILogHandler, LogData, DefaultLogger
│   │   ├── slog/                  # 全局日志 API
│   │   └── handler/               # RootHandler
│   │       ├── compound/          # 复合 handler
│   │       ├── console/           # 控制台 handler
│   │       └── file/              # 文件 handler（含背压策略）
│   ├── configuration/             # 配置接口体系
│   │   └── sources/               # Yaml/Json/Memory/File 配置源
│   ├── notifier/                  # INotifier
│   ├── option/                    # Option[T] 选项模式
│   ├── xnet/                      # IPreprocessor
│   │   └── transport/             # WebSocket 连接
│   ├── xsync/                     # TimeoutWaitGroup
│   ├── metrics/                   # PromCollector
│   └── version/                   # Version 解析
├── internal/                      # 内部实现
│   ├── host/                      # Host 内部实现（原 core/host/internal/）
│   ├── crontab/                   # Cron 解析
│   ├── debug/                     # 堆栈信息
│   ├── encrypt/dh/                # DH 加密
│   ├── kvs/                       # 全局 KV
│   ├── math/                      # 泛型数学
│   ├── meta/                      # NoCopy 标记
│   ├── task/                      # goroutine 池
│   ├── ticker/                    # 时间轮
│   ├── xhttp/                     # HTTP 封装
│   └── xjson/                     # JSON 封装
├── routines/                      # 保持不变
│   ├── node/                      # RPC 节点框架
│   └── ignore_input/              # 忽略输入服务
├── examples/                      # 保持不变
├── go.mod
└── ...
```

## 5. import 路径变更

| 旧路径 | 新路径 |
|--------|--------|
| `github.com/gmbytes/snow/core/host` | `github.com/gmbytes/snow/pkg/host` |
| `github.com/gmbytes/snow/core/host/builder` | `github.com/gmbytes/snow/pkg/host/builder` |
| `github.com/gmbytes/snow/core/host/internal` | `github.com/gmbytes/snow/internal/host` |
| `github.com/gmbytes/snow/core/injection` | `github.com/gmbytes/snow/pkg/injection` |
| `github.com/gmbytes/snow/core/logging` | `github.com/gmbytes/snow/pkg/logging` |
| `github.com/gmbytes/snow/core/logging/slog` | `github.com/gmbytes/snow/pkg/logging/slog` |
| `github.com/gmbytes/snow/core/logging/handler` | `github.com/gmbytes/snow/pkg/logging/handler` |
| `github.com/gmbytes/snow/core/logging/handler/*` | `github.com/gmbytes/snow/pkg/logging/handler/*` |
| `github.com/gmbytes/snow/core/configuration` | `github.com/gmbytes/snow/pkg/configuration` |
| `github.com/gmbytes/snow/core/configuration/sources` | `github.com/gmbytes/snow/pkg/configuration/sources` |
| `github.com/gmbytes/snow/core/notifier` | `github.com/gmbytes/snow/pkg/notifier` |
| `github.com/gmbytes/snow/core/option` | `github.com/gmbytes/snow/pkg/option` |
| `github.com/gmbytes/snow/core/xnet` | `github.com/gmbytes/snow/pkg/xnet` |
| `github.com/gmbytes/snow/core/xnet/transport` | `github.com/gmbytes/snow/pkg/xnet/transport` |
| `github.com/gmbytes/snow/core/xsync` | `github.com/gmbytes/snow/pkg/xsync` |
| `github.com/gmbytes/snow/core/metrics` | `github.com/gmbytes/snow/pkg/metrics` |
| `github.com/gmbytes/snow/core/version` | `github.com/gmbytes/snow/pkg/version` |
| `github.com/gmbytes/snow/core/crontab` | `github.com/gmbytes/snow/internal/crontab` |
| `github.com/gmbytes/snow/core/debug` | `github.com/gmbytes/snow/internal/debug` |
| `github.com/gmbytes/snow/core/encrypt/dh` | `github.com/gmbytes/snow/internal/encrypt/dh` |
| `github.com/gmbytes/snow/core/kvs` | `github.com/gmbytes/snow/internal/kvs` |
| `github.com/gmbytes/snow/core/math` | `github.com/gmbytes/snow/internal/math` |
| `github.com/gmbytes/snow/core/meta` | `github.com/gmbytes/snow/internal/meta` |
| `github.com/gmbytes/snow/core/task` | `github.com/gmbytes/snow/internal/task` |
| `github.com/gmbytes/snow/core/ticker` | `github.com/gmbytes/snow/internal/ticker` |
| `github.com/gmbytes/snow/core/xhttp` | `github.com/gmbytes/snow/internal/xhttp` |
| `github.com/gmbytes/snow/core/xjson` | `github.com/gmbytes/snow/internal/xjson` |

## 6. 迁移执行策略

### 6.1 原子提交顺序（每次提交必须 `go build ./...` 通过）

1. **创建目录结构**：mkdir pkg/ internal/ 及所有子目录
2. **迁移无依赖的 internal 包**（逐包提交）：
   - meta → internal/meta（无依赖方仅 xsync）
   - math → internal/math（无依赖方）
   - debug → internal/debug
   - xhttp → internal/xhttp
   - xjson → internal/xjson
   - kvs → internal/kvs
   - encrypt/dh → internal/encrypt/dh
3. **迁移无依赖的 pkg 包**：
   - notifier → pkg/notifier（仅被 configuration 引用）
   - injection → pkg/injection（被 host、builder 引用）
4. **迁移有依赖链的 pkg 包**：
   - logging（含所有子包）→ pkg/logging/
   - option → pkg/option
   - configuration（含 sources）→ pkg/configuration/
   - xsync → pkg/xsync
   - xnet（含 transport）→ pkg/xnet/
   - version → pkg/version
   - metrics → pkg/metrics
5. **迁移 task、ticker → internal/**（被 node 引用）
6. **迁移 crontab → internal/**
7. **迁移 host**：
   - core/host/ → pkg/host/（接口和注册函数）
   - core/host/builder/ → pkg/host/builder/
   - core/host/internal/ → internal/host/
8. **更新 server/ 的所有 import 路径**
9. **更新 examples/ 的所有 import 路径**
10. **删除空的 core/ 目录**

### 6.2 同步更新范围

每次移动包后，必须更新：
- snow 内部所有引用该包的 import 路径
- server 项目的 import 路径
- examples 的 import 路径
- 测试文件的 import 路径

### 6.3 验证检查点

每次提交后执行：
```bash
cd snow && go build ./... && go test ./... -count=1
cd ../gs && go build ./...
```

## 7. 风险与缓解

| 风险 | 缓解措施 |
|------|---------|
| host/internal → internal/host 路径冲突 | 先移动 host/internal 内容到 internal/host，再移动 host 接口到 pkg/host |
| import 路径大批量修改 | 使用 ast_grep_replace 批量替换，逐包验证 |
| server 项目编译中断 | 每步同步更新 server import，确保编译通过 |
| 测试因路径变更失败 | P0-5 日志测试已建立回归基线，迁移后重新验证 |
