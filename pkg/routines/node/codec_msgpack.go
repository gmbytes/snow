package node

import "github.com/vmihailenco/msgpack/v5"

// MsgpackCodec 基于 vmihailenco/msgpack/v5 的二进制编解码器。
// 相比 JSON，序列化体积更小、编解码速度更快。
// 使用时通过 RegisterOption.Codec 注入。
type MsgpackCodec struct{}

var _ ICodec = MsgpackCodec{}

func (MsgpackCodec) Marshal(v any) ([]byte, error)      { return msgpack.Marshal(v) }
func (MsgpackCodec) Unmarshal(data []byte, v any) error { return msgpack.Unmarshal(data, v) }
func (MsgpackCodec) Name() string                       { return "msgpack" }
