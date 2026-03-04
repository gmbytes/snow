package logging

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// codeError 实现 ErrorCodeCarrier 的测试用错误类型。
type codeError struct {
	code string
	msg  string
}

func (e *codeError) Error() string     { return e.msg }
func (e *codeError) ErrorCode() string { return e.code }

// wrappedCodeError 包裹了 codeError，模拟 errors.As 链。
type wrappedCodeError struct {
	inner error
}

func (e *wrappedCodeError) Error() string { return "wrapped: " + e.inner.Error() }
func (e *wrappedCodeError) Unwrap() error { return e.inner }

// plainError 普通错误，不实现 ErrorCodeCarrier。
type plainError struct{}

func (e *plainError) Error() string { return "plain error" }

// TestExtractErrorCode_WhenNoArgs_ExpectEmpty
func TestExtractErrorCode_WhenNoArgs_ExpectEmpty(t *testing.T) {
	result := ExtractErrorCode([]any{})
	assert.Equal(t, "", result)
}

// TestExtractErrorCode_WhenNoErrors_ExpectEmpty
func TestExtractErrorCode_WhenNoErrors_ExpectEmpty(t *testing.T) {
	result := ExtractErrorCode([]any{"string", 42, true})
	assert.Equal(t, "", result)
}

// TestExtractErrorCode_WhenErrorCarrier_ExpectCode
func TestExtractErrorCode_WhenErrorCarrier_ExpectCode(t *testing.T) {
	err := &codeError{code: "ERR_500", msg: "gs error"}
	result := ExtractErrorCode([]any{"prefix", err, "suffix"})
	assert.Equal(t, "ERR_500", result)
}

// TestExtractErrorCode_WhenWrappedCarrier_ExpectCode
func TestExtractErrorCode_WhenWrappedCarrier_ExpectCode(t *testing.T) {
	inner := &codeError{code: "ERR_WRAP", msg: "inner"}
	wrapped := &wrappedCodeError{inner: inner}
	result := ExtractErrorCode([]any{wrapped})
	assert.Equal(t, "ERR_WRAP", result)
}

// TestExtractErrorCode_WhenNilError_ExpectEmpty
func TestExtractErrorCode_WhenNilError_ExpectEmpty(t *testing.T) {
	result := ExtractErrorCode([]any{nil})
	assert.Equal(t, "", result)
}

// TestExtractErrorCode_WhenNilErrorInterface_ExpectEmpty
func TestExtractErrorCode_WhenNilErrorInterface_ExpectEmpty(t *testing.T) {
	var err error = nil
	result := ExtractErrorCode([]any{err})
	assert.Equal(t, "", result)
}

// TestExtractErrorCode_WhenPlainError_ExpectEmpty
func TestExtractErrorCode_WhenPlainError_ExpectEmpty(t *testing.T) {
	err := &plainError{}
	result := ExtractErrorCode([]any{err})
	assert.Equal(t, "", result)
}

// TestExtractErrorCode_WhenStdError_ExpectEmpty
func TestExtractErrorCode_WhenStdError_ExpectEmpty(t *testing.T) {
	err := errors.New("standard error")
	result := ExtractErrorCode([]any{err})
	assert.Equal(t, "", result)
}

// TestExtractErrorCode_WhenMultipleErrors_ExpectFirstCode
func TestExtractErrorCode_WhenMultipleErrors_ExpectFirstCode(t *testing.T) {
	err1 := &codeError{code: "FIRST", msg: "first"}
	err2 := &codeError{code: "SECOND", msg: "second"}
	result := ExtractErrorCode([]any{err1, err2})
	assert.Equal(t, "FIRST", result, "应提取第一个错误码")
}

// TestErrorCodeFromError_WhenNil_ExpectEmpty
func TestErrorCodeFromError_WhenNil_ExpectEmpty(t *testing.T) {
	result := ErrorCodeFromError(nil)
	assert.Equal(t, "", result)
}

// TestErrorCodeFromError_WhenNonCarrier_ExpectEmpty
func TestErrorCodeFromError_WhenNonCarrier_ExpectEmpty(t *testing.T) {
	err := errors.New("ordinary error")
	result := ErrorCodeFromError(err)
	assert.Equal(t, "", result)
}

// TestErrorCodeFromError_WhenCarrier_ExpectCode
func TestErrorCodeFromError_WhenCarrier_ExpectCode(t *testing.T) {
	err := &codeError{code: "MY_CODE", msg: "my error"}
	result := ErrorCodeFromError(err)
	assert.Equal(t, "MY_CODE", result)
}

// TestErrorCodeFromError_WhenWrappedCarrier_ExpectCode
func TestErrorCodeFromError_WhenWrappedCarrier_ExpectCode(t *testing.T) {
	inner := &codeError{code: "DEEP_CODE", msg: "deep"}
	wrapped := fmt.Errorf("outer: %w", inner)
	result := ErrorCodeFromError(wrapped)
	assert.Equal(t, "DEEP_CODE", result)
}
