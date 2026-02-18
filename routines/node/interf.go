package node

import "context"

type IPromise interface {
	Then(f any) IPromise
	Catch(f func(error)) IPromise
	Final(f func()) IPromise
	// WithContext 绑定显式 Context，覆盖默认的 Service 生命周期 Context。
	// 用于自定义超时（context.WithTimeout）、上游取消传播、或附加 trace 信息。
	// 未调用时，框架自动使用发起方 Service 的 Context 作为父级。
	WithContext(ctx context.Context) IPromise
	Done()
}

type IProxy interface {
	Call(name string, args ...any) IPromise
	GetNodeAddr() INodeAddr
	Avail() bool
}

type ITimeWheelHandle interface {
	Stop()
}

type IRpcContext interface {
	// Context 返回本次 RPC 调用关联的 context.Context，
	// 可用于派生下游调用的超时/取消，或传递 trace 信息。
	Context() context.Context
	GetRemoteNodeAddr() INodeAddr
	GetRemoteServiceAddr() int32
	Catch(f func(error)) IRpcContext
	Return(args ...any)
	Error(error)
}

type INodeAddr interface {
	IsLocalhost() bool
	GetIPString() string
	String() string
}

type IMetricCollector interface {
	// Gauge 仪表，设置值
	Gauge(name string, val int64)
	// Counter 计数器，累加值
	Counter(name string, val uint64)
	// Histogram 直方图，累加，但值为浮点数，可为正负
	Histogram(name string, val float64)
}

// ICodec RPC 消息体编解码接口，用于 TCP 二进制协议的参数序列化。
// HTTP RPC 始终使用 JSON（因 Content-Type 语义绑定），不受此接口影响。
type ICodec interface {
	// Marshal 将对象编码为字节串。
	Marshal(v any) ([]byte, error)
	// Unmarshal 将字节串解码到对象。
	Unmarshal(data []byte, v any) error
	// Name 返回编解码器名称（如 "json"、"msgpack"），用于日志与调试。
	Name() string
}

// IServiceDiscovery 服务发现接口，用于运行时动态解析服务名到节点地址。
// 未配置时框架回退到静态配置表（Config.Nodes），二者可共存。
type IServiceDiscovery interface {
	// Resolve 根据服务名解析目标节点地址。
	// 返回 AddrInvalid 或 error 时框架回退到静态表查找。
	Resolve(serviceName string) (INodeAddr, error)
	// Deregister 停机时注销自身（由 Node.Stop 调用）。传入当前节点地址和服务列表。
	Deregister(nodeAddr INodeAddr, services []string)
}
