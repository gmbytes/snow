package node

import (
	"context"
	"reflect"
	"strings"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// otelTestSvc 最小化测试服务，用于 OTel 集成测试。
type otelTestSvc struct {
	Service
}

func (s *otelTestSvc) RpcPing(ctx IRpcContext) {
	ctx.Return()
}

// newOtelTestService 构建一个最小化 Service，预注册 RpcPing 方法，
// 并将 node.regOpt.Tracer 设置为给定值。
func newOtelTestService(regOpt *RegisterOption) *otelTestSvc {
	svc := &otelTestSvc{}
	svc.methodMap = make(map[string]reflect.Value)
	svc.realSrv = svc

	svcType := reflect.TypeOf(svc)
	for i := 0; i < svcType.NumMethod(); i++ {
		m := svcType.Method(i)
		if strings.HasPrefix(m.Name, "Rpc") {
			svc.methodMap[m.Name] = m.Func
		}
	}

	n := &Node{
		logger: nopLogger{},
		regOpt: regOpt,
	}
	svc.node = n
	svc.name = "TestService"
	svc.ctx = context.Background()

	return svc
}

// buildPingRequest 构造一条本地 RpcPing 请求消息（无序列化，args 直接传递）。
func buildPingRequest() *message {
	m := &message{
		fName: "RpcPing",
		sess:  1,
		src:   1,
		args:  []reflect.Value{},
	}
	return m
}

// TestOtelTracer_NodeTracer 验证 NodeTracer 返回非 nil Tracer。
func TestOtelTracer_NodeTracer(t *testing.T) {
	tr := NodeTracer()
	if tr == nil {
		t.Fatal("NodeTracer() returned nil")
	}
}

// TestOtelTracer_ServerSpan 验证当 RegisterOption.Tracer 不为 nil 时，
// doDispatch 会为每次 RPC 创建一个 gs-side span。
func TestOtelTracer_ServerSpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp),
	)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tr := tp.Tracer(otelTracerName)
	regOpt := &RegisterOption{Tracer: tr}
	svc := newOtelTestService(regOpt)

	mReq := buildPingRequest()
	mReq.nAddr = AddrLocal

	svc.doDispatch(mReq)

	spans := exp.GetSpans()
	if len(spans) == 0 {
		t.Fatal("期望创建 OTel span，但没有 span 被记录")
	}

	span := spans[0]
	wantName := "rpc/TestService/RpcPing"
	if span.Name != wantName {
		t.Fatalf("span 名称错误: got=%q want=%q", span.Name, wantName)
	}
}

// TestOtelTracer_Disabled 验证当 RegisterOption.Tracer 为 nil 时，
// doDispatch 不产生任何 span（零开销路径）。
func TestOtelTracer_Disabled(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp),
	)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	regOpt := &RegisterOption{Tracer: nil}
	svc := newOtelTestService(regOpt)

	mReq := buildPingRequest()
	mReq.nAddr = AddrLocal

	svc.doDispatch(mReq)

	spans := exp.GetSpans()
	if len(spans) != 0 {
		t.Fatalf("Tracer 为 nil 时不应创建 span，但记录了 %d 个 span", len(spans))
	}
}
