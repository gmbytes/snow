package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

// PromCollector 是 Snow 内置的基于 Prometheus 的指标采集实现。
// 将 IMetricCollector 的 name 参数映射为 label，
// 统一落到少量核心指标上，便于 Prometheus 聚合与统计（QPS、P95/P99 等）。
type PromCollector struct {
	namespace string

	gauges     *prometheus.GaugeVec
	counters   *prometheus.CounterVec
	histograms *prometheus.HistogramVec
}

// NewPromCollector 创建一个带命名空间的默认 Prometheus 采集器。
func NewPromCollector(namespace string) *PromCollector {
	gauges := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "gauge",
		Help:      "Snow generic gauges, labeled by logical metric name.",
	}, []string{"name"})

	counters := promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "counter_total",
		Help:      "Snow generic counters, labeled by logical metric name.",
	}, []string{"name"})

	histograms := promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "duration_seconds",
		Help:      "Snow generic histograms for durations in seconds, labeled by logical metric name.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"name"})

	return &PromCollector{
		namespace:  namespace,
		gauges:     gauges,
		counters:   counters,
		histograms: histograms,
	}
}

// Gauge 实现 IMetricCollector.Gauge。
func (p *PromCollector) Gauge(name string, val int64) {
	if p == nil || p.gauges == nil {
		return
	}
	p.gauges.WithLabelValues(name).Set(float64(val))
}

// Counter 实现 IMetricCollector.Counter。
func (p *PromCollector) Counter(name string, val uint64) {
	if p == nil || p.counters == nil {
		return
	}
	if val == 0 {
		return
	}
	p.counters.WithLabelValues(name).Add(float64(val))
}

// Histogram 实现 IMetricCollector.Histogram。
// 上游传入纳秒级时长，此处自动转换为秒（Prometheus 推荐单位）。
func (p *PromCollector) Histogram(name string, val float64) {
	if p == nil || p.histograms == nil {
		return
	}
	const nsPerSecond = 1_000_000_000.0
	p.histograms.WithLabelValues(name).Observe(val / nsPerSecond)
}

// FastHTTPHandler 返回适配 fasthttp 的 /metrics 处理器。
func (p *PromCollector) FastHTTPHandler() fasthttp.RequestHandler {
	return fasthttpadaptor.NewFastHTTPHandler(promhttp.Handler())
}

// HTTPHandler 暴露底层 net/http Handler，便于上层自行挂载。
func (p *PromCollector) HTTPHandler() http.Handler {
	return promhttp.Handler()
}
