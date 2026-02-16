package logging

import "errors"

// ErrorCodeCarrier 可选接口：错误可暴露稳定错误码用于聚合。
type ErrorCodeCarrier interface {
	ErrorCode() string
}

// ExtractErrorCode 从日志参数中提取第一个可识别错误码。
func ExtractErrorCode(args []any) string {
	for _, arg := range args {
		err, ok := arg.(error)
		if !ok || err == nil {
			continue
		}
		if code := ErrorCodeFromError(err); code != "" {
			return code
		}
	}
	return ""
}

// ErrorCodeFromError 从 error 链中提取错误码。
func ErrorCodeFromError(err error) string {
	if err == nil {
		return ""
	}

	var carrier ErrorCodeCarrier
	if errors.As(err, &carrier) && carrier != nil {
		return carrier.ErrorCode()
	}
	return ""
}
