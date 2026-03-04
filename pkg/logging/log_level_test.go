package logging

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLevel_WhenConstants_ExpectCorrectValues(t *testing.T) {
	assert.Equal(t, Level(0), NONE, "NONE 应为 0")
	assert.Equal(t, Level(1), TRACE, "TRACE 应为 1")
	assert.Equal(t, Level(2), DEBUG, "DEBUG 应为 2")
	assert.Equal(t, Level(3), INFO, "INFO 应为 3")
	assert.Equal(t, Level(4), WARN, "WARN 应为 4")
	assert.Equal(t, Level(5), ERROR, "ERROR 应为 5")
	assert.Equal(t, Level(6), FATAL, "FATAL 应为 6")
}

func TestLevel_WhenComparing_ExpectCorrectOrder(t *testing.T) {
	assert.True(t, NONE < TRACE)
	assert.True(t, TRACE < DEBUG)
	assert.True(t, DEBUG < INFO)
	assert.True(t, INFO < WARN)
	assert.True(t, WARN < ERROR)
	assert.True(t, ERROR < FATAL)
}

func TestLevel_WhenUsedAsInt_ExpectCorrectIntValues(t *testing.T) {
	levels := []Level{NONE, TRACE, DEBUG, INFO, WARN, ERROR, FATAL}
	for i, level := range levels {
		assert.Equal(t, Level(i), level)
	}
}
