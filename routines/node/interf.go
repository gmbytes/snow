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
