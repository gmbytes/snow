package logging

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeLogData 创建带固定时间戳的测试 LogData，方便断言格式化结果。
func makeLogData(level Level, nodeName string, nodeID int, id, name, file string, line int, errorCode, message string) *LogData {
	return &LogData{
		Time:      time.Date(2026, 3, 4, 12, 30, 45, 999_000_000, time.UTC),
		NodeID:    nodeID,
		NodeName:  nodeName,
		Level:     level,
		ID:        id,
		Name:      name,
		File:      file,
		Line:      line,
		ErrorCode: errorCode,
		Message:   func() string { return message },
	}
}

// TestDefaultLogFormatter_WhenBasicData_ExpectDateTimePrefix 验证时间前缀格式
func TestDefaultLogFormatter_WhenBasicData_ExpectDateTimePrefix(t *testing.T) {
	data := makeLogData(INFO, "node1", 1, "id1", "svc", "", 0, "", "hello world")
	result := DefaultLogFormatter(data)
	assert.True(t, strings.HasPrefix(result, "2026/03/04 12:30:45.99"), "应包含正确时间戳，实际: %s", result)
}

// TestDefaultLogFormatter_WhenEmptyNodeName_ExpectPadded
func TestDefaultLogFormatter_WhenEmptyNodeName_ExpectPadded(t *testing.T) {
	data := makeLogData(INFO, "", 0, "", "", "", 0, "", "msg")
	result := DefaultLogFormatter(data)
	// 节点名为空时应该有格式填充
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "msg")
}

// TestDefaultLogFormatter_WhenLongNodeName_ExpectTruncated
func TestDefaultLogFormatter_WhenLongNodeName_ExpectTruncated(t *testing.T) {
	longName := "this-is-a-very-long-node-name-exceeds-26chars" // >26 chars
	data := makeLogData(INFO, longName, 42, "", "", "", 0, "", "msg")
	result := DefaultLogFormatter(data)
	// 超过 26 字符的节点名应该截断并加 ".."
	assert.Contains(t, result, "..", "长节点名应有 '..' 截断标记，实际: %s", result)
	// 不应包含完整长名
	assert.NotContains(t, result, longName)
}

// TestDefaultLogFormatter_WhenShortNodeName_ExpectNotTruncated
func TestDefaultLogFormatter_WhenShortNodeName_ExpectNotTruncated(t *testing.T) {
	shortName := "shortname"
	data := makeLogData(INFO, shortName, 1, "", "", "", 0, "", "msg")
	result := DefaultLogFormatter(data)
	assert.Contains(t, result, shortName)
}

// TestDefaultLogFormatter_WhenEmptyID_ExpectDashPlaceholder
func TestDefaultLogFormatter_WhenEmptyID_ExpectDashPlaceholder(t *testing.T) {
	data := makeLogData(INFO, "n", 1, "", "svc", "", 0, "", "msg")
	result := DefaultLogFormatter(data)
	assert.Contains(t, result, "-", "空 ID 应显示 '-'，实际: %s", result)
}

// TestDefaultLogFormatter_WhenLongID_ExpectTruncated
func TestDefaultLogFormatter_WhenLongID_ExpectTruncated(t *testing.T) {
	longID := "this-id-exceeds-12-chars"
	data := makeLogData(INFO, "n", 1, longID, "svc", "", 0, "", "msg")
	result := DefaultLogFormatter(data)
	assert.Contains(t, result, "..", "长 ID 应有 '..' 截断，实际: %s", result)
	assert.NotContains(t, result, longID)
}

// TestDefaultLogFormatter_WhenShortID_ExpectIncluded
func TestDefaultLogFormatter_WhenShortID_ExpectIncluded(t *testing.T) {
	shortID := "abc123"
	data := makeLogData(INFO, "n", 1, shortID, "svc", "", 0, "", "msg")
	result := DefaultLogFormatter(data)
	assert.Contains(t, result, shortID)
}

// TestDefaultLogFormatter_WhenEmptyName_ExpectSystemPlaceholder
func TestDefaultLogFormatter_WhenEmptyName_ExpectSystemPlaceholder(t *testing.T) {
	data := makeLogData(INFO, "n", 1, "id", "", "", 0, "", "msg")
	result := DefaultLogFormatter(data)
	assert.Contains(t, result, "System", "空 Name 应显示 'System'，实际: %s", result)
}

// TestDefaultLogFormatter_WhenLongName_ExpectTruncated
func TestDefaultLogFormatter_WhenLongName_ExpectTruncated(t *testing.T) {
	longName := "this-service-name-exceeds-16-chars"
	data := makeLogData(INFO, "n", 1, "id", longName, "", 0, "", "msg")
	result := DefaultLogFormatter(data)
	assert.Contains(t, result, "..", "长 Name 应有 '..' 截断，实际: %s", result)
	assert.NotContains(t, result, longName)
}

// TestDefaultLogFormatter_WhenFileAndLine_ExpectIncluded
func TestDefaultLogFormatter_WhenFileAndLine_ExpectIncluded(t *testing.T) {
	data := makeLogData(INFO, "n", 1, "id", "svc", "main.go", 99, "", "msg")
	result := DefaultLogFormatter(data)
	assert.Contains(t, result, "main.go(99)", "应包含文件和行号，实际: %s", result)
}

// TestDefaultLogFormatter_WhenNoFile_ExpectNoFileLine
func TestDefaultLogFormatter_WhenNoFile_ExpectNoFileLine(t *testing.T) {
	data := makeLogData(INFO, "n", 1, "id", "svc", "", 0, "", "msg")
	result := DefaultLogFormatter(data)
	assert.NotContains(t, result, "(0)", "无文件时不应出现行号，实际: %s", result)
}

// TestDefaultLogFormatter_WhenErrorCode_ExpectIncluded
func TestDefaultLogFormatter_WhenErrorCode_ExpectIncluded(t *testing.T) {
	data := makeLogData(ERROR, "n", 1, "id", "svc", "", 0, "ERR_404", "msg")
	result := DefaultLogFormatter(data)
	assert.Contains(t, result, "error_code=ERR_404", "应包含错误码，实际: %s", result)
}

// TestDefaultLogFormatter_WhenNoErrorCode_ExpectNoErrorCodeField
func TestDefaultLogFormatter_WhenNoErrorCode_ExpectNoErrorCodeField(t *testing.T) {
	data := makeLogData(INFO, "n", 1, "id", "svc", "", 0, "", "msg")
	result := DefaultLogFormatter(data)
	assert.NotContains(t, result, "error_code=", "无错误码时不应出现 error_code= 字段，实际: %s", result)
}

// TestDefaultLogFormatter_WhenMessage_ExpectAtEnd
func TestDefaultLogFormatter_WhenMessage_ExpectAtEnd(t *testing.T) {
	data := makeLogData(INFO, "n", 1, "id", "svc", "", 0, "", "hello-unique-message")
	result := DefaultLogFormatter(data)
	assert.True(t, strings.HasSuffix(result, "hello-unique-message"), "消息应在末尾，实际: %s", result)
}

// TestColorLogFormatter_WhenCalled_ExpectAnsiCodes
func TestColorLogFormatter_WhenCalled_ExpectAnsiCodes(t *testing.T) {
	data := makeLogData(INFO, "n", 1, "id", "svc", "", 0, "", "msg")
	result := ColorLogFormatter(data)
	// ANSI escape 以 \x1b[ 开头，以 \x1b[0m 结尾
	assert.Contains(t, result, "\x1b[", "ColorLogFormatter 应包含 ANSI 前缀，实际: %s", result)
	assert.True(t, strings.HasSuffix(result, "\x1b[0m"), "ColorLogFormatter 应以 \\x1b[0m 结尾，实际: %s", result)
}

// TestColorLogFormatter_WhenDifferentLevels_ExpectDifferentColors
func TestColorLogFormatter_WhenDifferentLevels_ExpectDifferentColors(t *testing.T) {
	levels := []Level{TRACE, DEBUG, INFO, WARN, ERROR, FATAL}
	colors := make(map[string]struct{})
	for _, level := range levels {
		data := makeLogData(level, "n", 1, "id", "svc", "", 0, "", "msg")
		result := ColorLogFormatter(data)
		// 提取 ANSI 颜色码
		start := strings.Index(result, "\x1b[1;")
		require.True(t, start >= 0, "level=%d 应有颜色码", level)
		end := strings.Index(result[start:], "m")
		require.True(t, end >= 0)
		colorCode := result[start : start+end+1]
		colors[colorCode] = struct{}{}
	}
	// 各 level 应至少有多种颜色（不完全相同）
	assert.Greater(t, len(colors), 1, "不同级别应有不同颜色")
}

// TestColorLogFormatter_WhenLongNodeName_ExpectTruncated
func TestColorLogFormatter_WhenLongNodeName_ExpectTruncated(t *testing.T) {
	longName := "abcdefghijklmnopqrstuvwxyz-overflow" // >26
	data := makeLogData(INFO, longName, 1, "", "", "", 0, "", "msg")
	result := ColorLogFormatter(data)
	assert.Contains(t, result, "..")
}

// TestLogFormatterContainer_WhenAddAndGet_ExpectSameFormatter
func TestLogFormatterContainer_WhenAddAndGet_ExpectSameFormatter(t *testing.T) {
	repo := NewLogFormatterRepository()
	called := false
	myFormatter := func(logData *LogData) string {
		called = true
		return "custom"
	}
	repo.AddFormatter("myFmt", myFormatter)
	got := repo.GetFormatter("myFmt")
	require.NotNil(t, got)
	result := got(nil)
	assert.True(t, called)
	assert.Equal(t, "custom", result)
}

// TestLogFormatterContainer_WhenGetMissing_ExpectNil
func TestLogFormatterContainer_WhenGetMissing_ExpectNil(t *testing.T) {
	repo := NewLogFormatterRepository()
	got := repo.GetFormatter("nonexistent")
	assert.Nil(t, got)
}

// TestLogFormatterContainer_WhenOverwrite_ExpectNewFormatter
func TestLogFormatterContainer_WhenOverwrite_ExpectNewFormatter(t *testing.T) {
	repo := NewLogFormatterRepository()
	repo.AddFormatter("f", func(*LogData) string { return "v1" })
	repo.AddFormatter("f", func(*LogData) string { return "v2" })
	got := repo.GetFormatter("f")
	require.NotNil(t, got)
	assert.Equal(t, "v2", got(nil))
}
