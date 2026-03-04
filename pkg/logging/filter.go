package logging

type ILogFilter interface {
	ShouldLog(level Level, name string, path string) bool
}

type LevelFilter struct {
	Min Level
}

func (f *LevelFilter) ShouldLog(level Level, _ string, _ string) bool {
	return level >= f.Min
}

type combineFilter struct {
	filters []ILogFilter
}

func CombineFilter(filters ...ILogFilter) ILogFilter {
	return &combineFilter{filters: filters}
}

func (c *combineFilter) ShouldLog(level Level, name string, path string) bool {
	for _, f := range c.filters {
		if f != nil && !f.ShouldLog(level, name, path) {
			return false
		}
	}
	return true
}
