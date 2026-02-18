package node

import "github.com/gmbytes/snow/core/xjson"

// JsonCodec 基于 xjson（json-iterator）的默认编解码器。
type JsonCodec struct{}

func (JsonCodec) Marshal(v any) ([]byte, error)        { return xjson.Marshal(v) }
func (JsonCodec) Unmarshal(data []byte, v any) error    { return xjson.Unmarshal(data, v) }
func (JsonCodec) Name() string                          { return "json" }

// nodeCodec 返回当前全局 Node 的 Codec 实例，供 message 层调用。
// 若 gNode 尚未初始化则回退到 JSON。
func nodeCodec() ICodec {
	if gNode != nil && gNode.regOpt != nil && gNode.regOpt.Codec != nil {
		return gNode.regOpt.Codec
	}
	return JsonCodec{}
}
