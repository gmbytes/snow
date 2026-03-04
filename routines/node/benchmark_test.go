package node

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// ──────────────────────────────────────────────────────────────────────────────
// 测试用辅助类型
// ──────────────────────────────────────────────────────────────────────────────

// benchPayload 模拟 RPC 请求中典型的结构体参数。
type benchPayload struct {
	UserID   int64  `json:"user_id"`
	Name     string `json:"name"`
	Score    int32  `json:"score"`
	IsOnline bool   `json:"is_online"`
}

// benchSvc 是最小化的测试服务，用于 Service.Entry 基准测试。
type benchSvc struct {
	Service
}

// RpcTest 模拟生产环境中典型的 RPC 处理方法签名。
func (s *benchSvc) RpcTest(ctx IRpcContext, name string, id int64) {
	// 模拟最轻量的处理：直接 Return，不引入额外分配
	ctx.Return()
}

// RpcTestPayload 带结构体参数的 RPC 方法，模拟更复杂的参数传递。
func (s *benchSvc) RpcTestPayload(ctx IRpcContext, p benchPayload) {
	ctx.Return()
}

// ──────────────────────────────────────────────────────────────────────────────
// A) 消息序列化基准测试
// ──────────────────────────────────────────────────────────────────────────────

// BenchmarkMessageMarshal_Request 测量请求消息序列化的分配基线。
// 每次迭代创建新 message，避免 marshal 写入 ss.data 后复用脏数据。
func BenchmarkMessageMarshal_Request(b *testing.B) {
	b.ReportAllocs()

	payload := benchPayload{UserID: 12345, Name: "player_001", Score: 9999, IsOnline: true}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := &message{
			fName: "RpcTestMethod",
			src:   1,
			dst:   2,
			sess:  int32(i + 1),
			trace: int64(i),
			args: []reflect.Value{
				reflect.ValueOf("player_001"),
				reflect.ValueOf(int64(12345)),
				reflect.ValueOf(payload),
			},
		}
		_, err := m.marshal()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMessageMarshal_Response 测量响应消息序列化的分配基线。
func BenchmarkMessageMarshal_Response(b *testing.B) {
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := &message{
			// fName 为空 + args 非 nil → 响应路径
			src:  2,
			dst:  1,
			sess: -int32(i + 1),
			args: []reflect.Value{
				reflect.ValueOf(true),
				reflect.ValueOf("operation_ok"),
			},
		}
		_, err := m.marshal()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMessageUnmarshal 测量从 wire bytes 反序列化消息的分配基线。
func BenchmarkMessageUnmarshal(b *testing.B) {
	b.ReportAllocs()

	// 预先序列化一条请求消息，作为 wire bytes 输入
	setup := &message{
		fName: "RpcTestMethod",
		src:   1,
		dst:   2,
		sess:  42,
		trace: 100,
		args: []reflect.Value{
			reflect.ValueOf("hello"),
			reflect.ValueOf(int64(999)),
		},
	}
	wireBytes, err := setup.marshal()
	if err != nil {
		b.Fatal(err)
	}

	// 复制一份，避免 unmarshal 修改原始 slice
	input := make([]byte, len(wireBytes))
	copy(input, wireBytes)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := &message{}
		if err := m.unmarshal(input); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMessageMarshalArgs 单独测量 marshalArgs 方法的分配基线。
func BenchmarkMessageMarshalArgs(b *testing.B) {
	b.ReportAllocs()

	m := &message{}
	args := []reflect.Value{
		reflect.ValueOf("player_001"),
		reflect.ValueOf(int64(12345)),
		reflect.ValueOf(int32(9999)),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := m.marshalArgs(args)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMessageUnmarshalArgs 单独测量 unmarshalArgs 方法的分配基线。
func BenchmarkMessageUnmarshalArgs(b *testing.B) {
	b.ReportAllocs()

	// 预先序列化 args bytes
	m := &message{}
	args := []reflect.Value{
		reflect.ValueOf("player_001"),
		reflect.ValueOf(int64(12345)),
		reflect.ValueOf(int32(9999)),
	}
	bs, err := m.marshalArgs(args)
	if err != nil {
		b.Fatal(err)
	}

	// 构造一个包含 (IRpcContext, string, int64, int32) 参数的函数类型
	// unmarshalArgs 从 argI=2 开始解析（跳过 receiver 和 ctx）
	type benchFn func(IRpcContext, string, int64, int32)
	ft := reflect.TypeOf((*benchFn)(nil)).Elem()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := m.unmarshalArgs(bs, 1, ft)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// B) Context 创建基准测试
// ──────────────────────────────────────────────────────────────────────────────

// BenchmarkNewRpcContext 测量 rpcContext 创建的分配基线。
func BenchmarkNewRpcContext(b *testing.B) {
	b.ReportAllocs()

	parentCtx := context.Background()
	mRsp := &message{src: 2, dst: 1, sess: -1}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := newRpcContext(parentCtx, nil, mRsp, 1, 1, AddrLocal, nil, nil)
		// 必须调用 cancel 避免 goroutine 泄漏
		ctx.cancel()
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// C) Promise 创建基准测试
// ──────────────────────────────────────────────────────────────────────────────

// BenchmarkNewPromise 测量 promise 创建的分配基线。
func BenchmarkNewPromise(b *testing.B) {
	b.ReportAllocs()

	proxy := &serviceProxy{sAddr: 1}
	args := []any{"player_001", int64(12345)}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := newPromise(proxy, "RpcTestMethod", args)
		_ = p
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// D) Reflect 调度基准测试
// ──────────────────────────────────────────────────────────────────────────────

// BenchmarkReflectCall 测量 reflect.Value.Call 对典型 RPC handler 的调度开销。
// 这是展示反射开销的关键基准测试。
func BenchmarkReflectCall(b *testing.B) {
	b.ReportAllocs()

	svc := &benchSvc{}
	svc.methodMap = make(map[string]reflect.Value)
	svc.realSrv = svc

	// 构建 methodMap，与生产代码 registry.go 的逻辑一致
	svcType := reflect.TypeOf(svc)
	for i := 0; i < svcType.NumMethod(); i++ {
		m := svcType.Method(i)
		if strings.HasPrefix(m.Name, "Rpc") {
			svc.methodMap[m.Name] = m.Func
		}
	}

	f := svc.methodMap["RpcTest"]
	mRsp := &message{src: 2, dst: 1, sess: -1}
	ctx := newRpcContext(context.Background(), nil, mRsp, 1, 1, AddrLocal, nil, nil)
	ctx.flushed = true // 防止 flush 时查找 sender

	fArgs := []reflect.Value{
		reflect.ValueOf(svc),
		reflect.ValueOf(IRpcContext(ctx)),
		reflect.ValueOf("player_001"),
		reflect.ValueOf(int64(12345)),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Call(fArgs)
	}
}

// BenchmarkDirectCall 直接调用对比基线，用于量化 reflect.Call 的额外开销。
func BenchmarkDirectCall(b *testing.B) {
	b.ReportAllocs()

	svc := &benchSvc{}
	svc.realSrv = svc

	mRsp := &message{src: 2, dst: 1, sess: -1}
	ctx := newRpcContext(context.Background(), nil, mRsp, 1, 1, AddrLocal, nil, nil)
	ctx.flushed = true // 防止 flush 时查找 sender

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.RpcTest(ctx, "player_001", 12345)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// E) Codec 基准测试
// ──────────────────────────────────────────────────────────────────────────────

// BenchmarkJsonCodecMarshal 测量 JSON codec marshal 的分配基线。
func BenchmarkJsonCodecMarshal(b *testing.B) {
	b.ReportAllocs()

	codec := JsonCodec{}
	payload := []any{
		"player_001",
		int64(12345),
		benchPayload{UserID: 12345, Name: "player_001", Score: 9999, IsOnline: true},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := codec.Marshal(payload)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkJsonCodecUnmarshal 测量 JSON codec unmarshal 的分配基线。
func BenchmarkJsonCodecUnmarshal(b *testing.B) {
	b.ReportAllocs()

	codec := JsonCodec{}
	payload := []any{
		"player_001",
		int64(12345),
		benchPayload{UserID: 12345, Name: "player_001", Score: 9999, IsOnline: true},
	}
	bs, err := codec.Marshal(payload)
	if err != nil {
		b.Fatal(err)
	}

	// 准备反序列化目标容器（与 unmarshalArgs 模式一致）
	var target []any

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		target = nil
		if err := codec.Unmarshal(bs, &target); err != nil {
			b.Fatal(err)
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// F) 完整 Entry 路径基准测试
// ──────────────────────────────────────────────────────────────────────────────

// BenchmarkServiceEntry 测量完整的 Service.Entry() 路径：
// methodMap 查找 + argGetter（本地调用路径，无序列化）+ reflect.Call。
// 这是最重要的基准测试，展示每次 RPC 的总体开销。
func BenchmarkServiceEntry(b *testing.B) {
	b.ReportAllocs()

	// 构建最小化 Service，模拟注册后的状态
	svc := &benchSvc{}
	svc.methodMap = make(map[string]reflect.Value)
	svc.realSrv = svc

	svcType := reflect.TypeOf(svc)
	for i := 0; i < svcType.NumMethod(); i++ {
		m := svcType.Method(i)
		if strings.HasPrefix(m.Name, "Rpc") {
			svc.methodMap[m.Name] = m.Func
		}
	}

	// 预先准备本地调用的请求消息（args 不为 nil → 本地路径，无需反序列化）
	reqArgs := []reflect.Value{
		reflect.ValueOf("player_001"),
		reflect.ValueOf(int64(12345)),
	}

	mRsp := &message{src: 2, dst: 1, sess: -1}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 每次迭代模拟一次完整的本地 RPC Entry 调用

		// 1. 创建 rpcContext（生产代码在 doDispatch 中执行）
		ctx := newRpcContext(context.Background(), nil, mRsp, 1, 1, AddrLocal, nil, nil)
		ctx.flushed = true // 防止 flush 时尝试发送响应

		// 2. 准备本地请求消息的 argGetter（本地路径：直接返回 args，无序列化）
		mReq := &message{
			fName: "RpcTest",
			args:  reqArgs,
		}

		// 3. 调用 Entry：methodMap 查找 + argGetter + reflect.Call 的总和
		f := svc.Entry(ctx, "RpcTest", mReq.getRequestFuncArgs)
		if f != nil {
			f()
		}
	}
}

// BenchmarkServiceEntry_WithSerialization 测量带反序列化的完整 Entry 路径
// （模拟远程调用：data 非 nil，需要从 wire bytes 解析参数）。
func BenchmarkServiceEntry_WithSerialization(b *testing.B) {
	b.ReportAllocs()

	svc := &benchSvc{}
	svc.methodMap = make(map[string]reflect.Value)
	svc.realSrv = svc

	svcType := reflect.TypeOf(svc)
	for i := 0; i < svcType.NumMethod(); i++ {
		m := svcType.Method(i)
		if strings.HasPrefix(m.Name, "Rpc") {
			svc.methodMap[m.Name] = m.Func
		}
	}

	// 预先序列化请求（模拟 wire bytes 路径）
	setupMsg := &message{
		fName: "RpcTest",
		src:   1,
		dst:   2,
		sess:  42,
		args: []reflect.Value{
			reflect.ValueOf("player_001"),
			reflect.ValueOf(int64(12345)),
		},
	}
	wireBytes, err := setupMsg.marshal()
	if err != nil {
		b.Fatal(err)
	}
	wireBytesCopy := make([]byte, len(wireBytes))
	copy(wireBytesCopy, wireBytes)

	mRsp := &message{src: 2, dst: 1, sess: -42}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 模拟远程调用：从 wire bytes 构建请求消息
		mReq := &message{}
		if err := mReq.unmarshal(wireBytesCopy); err != nil {
			b.Fatal(err)
		}

		ctx := newRpcContext(context.Background(), nil, mRsp, 42, 1, AddrLocal, nil, nil)
		ctx.flushed = true

		f := svc.Entry(ctx, "RpcTest", mReq.getRequestFuncArgs)
		if f != nil {
			f()
		}
	}
}
