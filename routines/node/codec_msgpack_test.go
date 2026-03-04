package node

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// msgpackTestPayload 模拟 RPC 请求中典型的结构体参数（与 benchPayload 字段对应）。
type msgpackTestPayload struct {
	UserID   int64  `msgpack:"user_id"`
	Name     string `msgpack:"name"`
	Score    int32  `msgpack:"score"`
	IsOnline bool   `msgpack:"is_online"`
}

// ──────────────────────────────────────────────────────────────────────────────
// 基础功能测试
// ──────────────────────────────────────────────────────────────────────────────

// TestMsgpackCodec_Name_返回msgpack字符串
func TestMsgpackCodec_Name_返回msgpack字符串(t *testing.T) {
	codec := MsgpackCodec{}
	assert.Equal(t, "msgpack", codec.Name())
}

// TestMsgpackCodec_实现ICodec接口
func TestMsgpackCodec_实现ICodec接口(t *testing.T) {
	var _ ICodec = MsgpackCodec{}
}

// ──────────────────────────────────────────────────────────────────────────────
// 基础类型往返
// ──────────────────────────────────────────────────────────────────────────────

// TestMsgpackCodec_MarshalUnmarshal_Primitive 验证基本类型往返正确性。
func TestMsgpackCodec_MarshalUnmarshal_Primitive(t *testing.T) {
	codec := MsgpackCodec{}

	t.Run("string", func(t *testing.T) {
		data, err := codec.Marshal("hello msgpack")
		require.NoError(t, err)
		var got string
		require.NoError(t, codec.Unmarshal(data, &got))
		assert.Equal(t, "hello msgpack", got)
	})

	t.Run("int64", func(t *testing.T) {
		data, err := codec.Marshal(int64(9999999999))
		require.NoError(t, err)
		var got int64
		require.NoError(t, codec.Unmarshal(data, &got))
		assert.Equal(t, int64(9999999999), got)
	})

	t.Run("float64", func(t *testing.T) {
		data, err := codec.Marshal(3.14159)
		require.NoError(t, err)
		var got float64
		require.NoError(t, codec.Unmarshal(data, &got))
		assert.InDelta(t, 3.14159, got, 1e-9)
	})

	t.Run("bool_true", func(t *testing.T) {
		data, err := codec.Marshal(true)
		require.NoError(t, err)
		var got bool
		require.NoError(t, codec.Unmarshal(data, &got))
		assert.True(t, got)
	})

	t.Run("bool_false", func(t *testing.T) {
		data, err := codec.Marshal(false)
		require.NoError(t, err)
		var got bool
		require.NoError(t, codec.Unmarshal(data, &got))
		assert.False(t, got)
	})
}

// TestMsgpackCodec_MarshalUnmarshal_Struct 验证结构体往返正确性。
func TestMsgpackCodec_MarshalUnmarshal_Struct(t *testing.T) {
	codec := MsgpackCodec{}

	original := msgpackTestPayload{
		UserID:   12345,
		Name:     "player_001",
		Score:    9999,
		IsOnline: true,
	}

	data, err := codec.Marshal(original)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	var got msgpackTestPayload
	require.NoError(t, codec.Unmarshal(data, &got))
	assert.Equal(t, original, got)
}

// TestMsgpackCodec_MarshalUnmarshal_SliceAny 验证 []any 混合类型往返——
// 这是 RPC args 的核心序列化模式。
func TestMsgpackCodec_MarshalUnmarshal_SliceAny(t *testing.T) {
	codec := MsgpackCodec{}

	payload := msgpackTestPayload{UserID: 99, Name: "bob", Score: 10, IsOnline: false}
	args := []any{"hello", int64(42), payload}

	data, err := codec.Marshal(args)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	// 按照 unmarshalArgs 的预分配指针模式解码
	targets := []any{new(string), new(int64), new(msgpackTestPayload)}
	require.NoError(t, codec.Unmarshal(data, &targets))

	assert.Equal(t, "hello", *targets[0].(*string))
	assert.Equal(t, int64(42), *targets[1].(*int64))
	assert.Equal(t, payload, *targets[2].(*msgpackTestPayload))
}

// TestMsgpackCodec_MarshalUnmarshal_WireError 验证 wireError 结构体往返——
// 用于 RPC 错误编码。
func TestMsgpackCodec_MarshalUnmarshal_WireError(t *testing.T) {
	codec := MsgpackCodec{}

	we := wireError{
		Code: ErrCodec,
		Msg:  "codec test error",
	}

	data, err := codec.Marshal(we)
	require.NoError(t, err)

	var got wireError
	require.NoError(t, codec.Unmarshal(data, &got))
	assert.Equal(t, we.Code, got.Code)
	assert.Equal(t, we.Msg, got.Msg)
}

// TestMsgpackCodec_RoundTrip_WithPointers 验证 unmarshalArgs 实际使用的 reflect.New 指针模式——
// 框架在 unmarshalArgs 中通过 reflect.New(ft.In(i)).Interface() 创建目标指针，
// 解码后再通过 reflect.ValueOf(arg).Elem() 取出值。
func TestMsgpackCodec_RoundTrip_WithPointers(t *testing.T) {
	codec := MsgpackCodec{}

	// 1. 序列化
	args := []any{"hello", int64(42), int64(9999999)}
	data, err := codec.Marshal(args)
	require.NoError(t, err)

	// 2. 按 RPC unmarshalArgs 模式：reflect.New 创建指针目标
	type rpcFn func(string, int64, int64)
	ft := reflect.TypeOf((*rpcFn)(nil)).Elem()

	tArgs := make([]any, 0, ft.NumIn())
	for i := 0; i < ft.NumIn(); i++ {
		tArgs = append(tArgs, reflect.New(ft.In(i)).Interface())
	}

	require.NoError(t, codec.Unmarshal(data, &tArgs))

	// 3. reflect.ValueOf(arg).Elem() 取值（与 unmarshalArgs 代码一致）
	vals := make([]reflect.Value, 0, len(tArgs))
	for i, arg := range tArgs {
		require.NotNil(t, arg)
		vals = append(vals, reflect.ValueOf(arg).Elem())
		_ = i
	}

	assert.Equal(t, "hello", vals[0].Interface())
	assert.Equal(t, int64(42), vals[1].Interface())
	assert.Equal(t, int64(9999999), vals[2].Interface())
}

// ──────────────────────────────────────────────────────────────────────────────
// 边界情况
// ──────────────────────────────────────────────────────────────────────────────

// TestMsgpackCodec_EmptySlice 验证空切片往返。
func TestMsgpackCodec_EmptySlice(t *testing.T) {
	codec := MsgpackCodec{}

	args := []any{}
	data, err := codec.Marshal(args)
	require.NoError(t, err)

	var got []any
	require.NoError(t, codec.Unmarshal(data, &got))
	assert.Empty(t, got)
}

// TestMsgpackCodec_Nil 验证 nil 值序列化。
func TestMsgpackCodec_Nil(t *testing.T) {
	codec := MsgpackCodec{}

	data, err := codec.Marshal(nil)
	require.NoError(t, err)

	var got any
	require.NoError(t, codec.Unmarshal(data, &got))
	assert.Nil(t, got)
}

// ──────────────────────────────────────────────────────────────────────────────
// 与 JsonCodec 对比：msgpack 体积应更小
// ──────────────────────────────────────────────────────────────────────────────

// TestMsgpackCodec_编码体积小于JSON 验证 msgpack 序列化体积优势。
func TestMsgpackCodec_编码体积小于JSON(t *testing.T) {
	jsonCodec := JsonCodec{}
	msgpCodec := MsgpackCodec{}

	payload := []any{
		"player_001",
		int64(12345),
		msgpackTestPayload{UserID: 12345, Name: "player_001", Score: 9999, IsOnline: true},
	}

	jsonBytes, err := jsonCodec.Marshal(payload)
	require.NoError(t, err)

	msgpBytes, err := msgpCodec.Marshal(payload)
	require.NoError(t, err)

	t.Logf("JSON size: %d bytes, Msgpack size: %d bytes", len(jsonBytes), len(msgpBytes))
	assert.Less(t, len(msgpBytes), len(jsonBytes), "msgpack 编码体积应小于 JSON")
}

// ──────────────────────────────────────────────────────────────────────────────
// 基准测试（与 JsonCodec 基准对比）
// ──────────────────────────────────────────────────────────────────────────────

// BenchmarkMsgpackCodecMarshal 测量 msgpack codec marshal 的分配基线。
// 对比基线：BenchmarkJsonCodecMarshal ~482ns/2allocs。
func BenchmarkMsgpackCodecMarshal(b *testing.B) {
	b.ReportAllocs()

	codec := MsgpackCodec{}
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

// BenchmarkMsgpackCodecUnmarshal 测量 msgpack codec unmarshal 的分配基线。
// 对比基线：BenchmarkJsonCodecUnmarshal ~1556ns/20allocs。
func BenchmarkMsgpackCodecUnmarshal(b *testing.B) {
	b.ReportAllocs()

	codec := MsgpackCodec{}
	payload := []any{
		"player_001",
		int64(12345),
		benchPayload{UserID: 12345, Name: "player_001", Score: 9999, IsOnline: true},
	}
	bs, err := codec.Marshal(payload)
	if err != nil {
		b.Fatal(err)
	}

	var target []any

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		target = nil
		if err := codec.Unmarshal(bs, &target); err != nil {
			b.Fatal(err)
		}
	}
}
