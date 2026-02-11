package xjson

import jsoniter "github.com/json-iterator/go"

// RawMessage 是对 json-iterator 的 RawMessage 的简单别名，
// 用于延迟解析或直接传递 JSON 字段。
type RawMessage = jsoniter.RawMessage

var defaultAPI = jsoniter.ConfigCompatibleWithStandardLibrary

// Marshal 使用默认配置将对象编码为 JSON 字节串。
func Marshal(v any) ([]byte, error) {
	return defaultAPI.Marshal(v)
}

// Unmarshal 使用默认配置将 JSON 字节串解码到对象。
func Unmarshal(data []byte, v any) error {
	return defaultAPI.Unmarshal(data, v)
}

// MarshalToString 使用默认配置将对象编码为 JSON 字符串。
func MarshalToString(v any) (string, error) {
	return defaultAPI.MarshalToString(v)
}

// UnmarshalFromString 使用默认配置将 JSON 字符串解码到对象。
func UnmarshalFromString(str string, v any) error {
	return defaultAPI.UnmarshalFromString(str, v)
}