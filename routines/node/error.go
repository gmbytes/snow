package node

import (
	"errors"
	"fmt"
)

// ErrorCode 用于稳定的错误聚合与告警统计。
type ErrorCode string

const (
	ErrUnknown         ErrorCode = "UNKNOWN"
	ErrTimeout         ErrorCode = "TIMEOUT"
	ErrServiceNotFound ErrorCode = "SERVICE_NOT_FOUND"
	ErrCodec           ErrorCode = "CODEC"
	ErrTransport       ErrorCode = "TRANSPORT"
	ErrCancelled       ErrorCode = "CANCELLED"
	ErrInvalidArgument ErrorCode = "INVALID_ARGUMENT"
	ErrInternal        ErrorCode = "INTERNAL"
)

// Error 是统一错误包装，支持 errors.Is/errors.As。
type Error struct {
	Code  ErrorCode
	Op    string
	Msg   string
	Cause error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}

	base := fmt.Sprintf("[%s]", e.Code)
	if e.Op != "" {
		base += " " + e.Op
	}
	if e.Msg != "" {
		base += ": " + e.Msg
	}
	if e.Cause != nil {
		base += ": " + e.Cause.Error()
	}
	return base
}

// ErrorCode 返回稳定错误码，供日志与告警聚合使用。
func (e *Error) ErrorCode() string {
	if e == nil || e.Code == "" {
		return string(ErrUnknown)
	}
	return string(e.Code)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok || t == nil {
		return false
	}
	if t.Code == "" {
		return false
	}
	return e.Code == t.Code
}

func NewError(code ErrorCode, msg string) error {
	return &Error{
		Code: code,
		Msg:  msg,
	}
}

func WrapError(code ErrorCode, op string, err error) error {
	if err == nil {
		return nil
	}

	return &Error{
		Code:  code,
		Op:    op,
		Msg:   err.Error(),
		Cause: err,
	}
}

func CodeOf(err error) ErrorCode {
	if err == nil {
		return ErrUnknown
	}

	var e *Error
	if errors.As(err, &e) && e != nil && e.Code != "" {
		return e.Code
	}

	return ErrUnknown
}

func IsCode(err error, code ErrorCode) bool {
	return errors.Is(err, &Error{Code: code})
}
