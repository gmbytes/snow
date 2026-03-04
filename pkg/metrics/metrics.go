package metrics

type IMeterProvider interface {
	Meter(name string) IMeter
}

type IMeter interface {
	Counter(name string) ICounter
	Gauge(name string) IGauge
	Histogram(name string, boundaries ...float64) IHistogram
}

type ICounter interface {
	Add(value float64)
}

type IGauge interface {
	Set(value float64)
	Add(value float64)
}

type IHistogram interface {
	Record(value float64)
}

type MetricType int

const (
	MetricUnknown MetricType = iota
	MetricCounter
	MetricGauge
	MetricHistogram
)

type MetricData struct {
	Name    string
	Type    MetricType
	Value   float64
	Count   uint64
	Sum     float64
	Min     float64
	Max     float64
	Buckets map[float64]uint64
}

type IMeterRegistry interface {
	IMeterProvider
	ForEach(fn func(name string, meter IMeter))
	Collect() []MetricData
}
