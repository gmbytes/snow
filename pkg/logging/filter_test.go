package logging

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLevelFilter_WhenLevelAboveMin_ExpectShouldLogTrue
func TestLevelFilter_WhenLevelAboveMin_ExpectShouldLogTrue(t *testing.T) {
	f := &LevelFilter{Min: WARN}
	assert.True(t, f.ShouldLog(WARN, "", ""))
	assert.True(t, f.ShouldLog(ERROR, "", ""))
	assert.True(t, f.ShouldLog(FATAL, "", ""))
}

// TestLevelFilter_WhenLevelBelowMin_ExpectShouldLogFalse
func TestLevelFilter_WhenLevelBelowMin_ExpectShouldLogFalse(t *testing.T) {
	f := &LevelFilter{Min: WARN}
	assert.False(t, f.ShouldLog(TRACE, "", ""))
	assert.False(t, f.ShouldLog(DEBUG, "", ""))
	assert.False(t, f.ShouldLog(INFO, "", ""))
}

// TestCombineFilter_WhenAllFiltersPass_ExpectShouldLogTrue
func TestCombineFilter_WhenAllFiltersPass_ExpectShouldLogTrue(t *testing.T) {
	f1 := &LevelFilter{Min: DEBUG}
	f2 := &LevelFilter{Min: INFO}
	combined := CombineFilter(f1, f2)
	assert.True(t, combined.ShouldLog(INFO, "", ""))
	assert.True(t, combined.ShouldLog(WARN, "", ""))
}

// TestCombineFilter_WhenOneFilterFails_ExpectShouldLogFalse
func TestCombineFilter_WhenOneFilterFails_ExpectShouldLogFalse(t *testing.T) {
	f1 := &LevelFilter{Min: TRACE}
	f2 := &LevelFilter{Min: ERROR}
	combined := CombineFilter(f1, f2)
	assert.False(t, combined.ShouldLog(INFO, "", ""))
}

// TestCombineFilter_WhenNoFilters_ExpectShouldLogTrue
func TestCombineFilter_WhenNoFilters_ExpectShouldLogTrue(t *testing.T) {
	combined := CombineFilter()
	assert.True(t, combined.ShouldLog(TRACE, "", ""))
	assert.True(t, combined.ShouldLog(FATAL, "", ""))
}

// TestCombineFilter_WhenNilFilterInSlice_ExpectShouldLogTrue
func TestCombineFilter_WhenNilFilterInSlice_ExpectShouldLogTrue(t *testing.T) {
	combined := CombineFilter(nil, nil)
	assert.True(t, combined.ShouldLog(TRACE, "", ""))
}

// TestDefaultLogger_WhenFilterRejectLevel_ExpectHandlerNotCalled
func TestDefaultLogger_WhenFilterRejectLevel_ExpectHandlerNotCalled(t *testing.T) {
	h := &mockHandler{}
	logger := NewDefaultLogger("p", h, nil)
	logger.SetFilter(&LevelFilter{Min: ERROR})
	logger.Infof("should be filtered")
	assert.Equal(t, 0, h.count())
}

// TestDefaultLogger_WhenFilterAllowLevel_ExpectHandlerCalled
func TestDefaultLogger_WhenFilterAllowLevel_ExpectHandlerCalled(t *testing.T) {
	h := &mockHandler{}
	logger := NewDefaultLogger("p", h, nil)
	logger.SetFilter(&LevelFilter{Min: INFO})
	logger.Errorf("should pass")
	require.Equal(t, 1, h.count())
	assert.Equal(t, ERROR, h.last().Level)
}

// TestDefaultLogger_WhenNilFilter_ExpectPassthrough
func TestDefaultLogger_WhenNilFilter_ExpectPassthrough(t *testing.T) {
	h := &mockHandler{}
	logger := NewDefaultLogger("p", h, nil)
	logger.SetFilter(nil)
	logger.Tracef("no filter")
	assert.Equal(t, 1, h.count())
}

// TestDefaultLogger_WhenFilterCombined_ExpectBothApplied
func TestDefaultLogger_WhenFilterCombined_ExpectBothApplied(t *testing.T) {
	h := &mockHandler{}
	logger := NewDefaultLogger("p", h, nil)
	logger.SetFilter(CombineFilter(&LevelFilter{Min: WARN}, &LevelFilter{Min: INFO}))
	logger.Infof("blocked by first filter")
	assert.Equal(t, 0, h.count())
	logger.Warnf("passes both")
	assert.Equal(t, 1, h.count())
}

// TestLevelFilter_WhenCheckPath_ExpectPathIgnored
func TestLevelFilter_WhenCheckPath_ExpectPathIgnored(t *testing.T) {
	f := &LevelFilter{Min: INFO}
	assert.True(t, f.ShouldLog(INFO, "any/name", "any/path"))
}
