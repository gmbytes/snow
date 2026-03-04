package metrics

// MetricCollectorAdapter 将 IMeterRegistry 适配为 node.IMetricCollector 接口。
// 所有指标记录到 registry 中名为 "" 的默认 Meter。
type MetricCollectorAdapter struct {
	meter IMeter
}

func NewMetricCollectorAdapter(registry IMeterRegistry) *MetricCollectorAdapter {
	return &MetricCollectorAdapter{meter: registry.Meter("")}
}

func (a *MetricCollectorAdapter) Counter(name string, val uint64) {
	if val == 0 {
		return
	}
	a.meter.Counter(name).Add(float64(val))
}

func (a *MetricCollectorAdapter) Gauge(name string, val int64) {
	a.meter.Gauge(name).Set(float64(val))
}

func (a *MetricCollectorAdapter) Histogram(name string, val float64) {
	a.meter.Histogram(name).Record(val)
}
