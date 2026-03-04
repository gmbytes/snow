package logging

import (
	"bytes"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewSimpleLogHandler_WhenCreated_ExpectNonNil
func TestNewSimpleLogHandler_WhenCreated_ExpectNonNil(t *testing.T) {
	h := NewSimpleLogHandler()
	assert.NotNil(t, h)
}

// TestSimpleLogHandler_WhenLog_ExpectOutputToLogger
func TestSimpleLogHandler_WhenLog_ExpectOutputToLogger(t *testing.T) {
	var buf bytes.Buffer
	// 替换 log 标准输出到缓冲区，测试后恢复
	original := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() {
		log.SetOutput(original)
	})
	// 关闭前缀和时间戳以便断言
	origFlags := log.Flags()
	log.SetFlags(0)
	origPrefix := log.Prefix()
	log.SetPrefix("")
	t.Cleanup(func() {
		log.SetFlags(origFlags)
		log.SetPrefix(origPrefix)
	})

	h := NewSimpleLogHandler()
	data := &LogData{
		Time:    time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC),
		Level:   INFO,
		Message: func() string { return "hello-from-simple-handler" },
	}
	h.Log(data)

	output := buf.String()
	require.NotEmpty(t, output, "simpleLogHandler 应有输出")
	assert.True(t, strings.Contains(output, "hello-from-simple-handler"), "应包含消息内容，实际: %s", output)
}

// TestSimpleLogHandler_WhenLogMultipleTimes_ExpectAllOutput
func TestSimpleLogHandler_WhenLogMultipleTimes_ExpectAllOutput(t *testing.T) {
	var buf bytes.Buffer
	original := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(original) })

	h := NewSimpleLogHandler()
	for i := 0; i < 3; i++ {
		data := &LogData{
			Time:    time.Now(),
			Level:   DEBUG,
			Message: func() string { return "repeated" },
		}
		h.Log(data)
	}
	output := buf.String()
	count := strings.Count(output, "repeated")
	assert.Equal(t, 3, count, "应有 3 条日志")
}
