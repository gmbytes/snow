package metrics

import (
	"math"
	"sync"
)

var infPos = math.Inf(1)

var DefaultHistogramBoundaries = []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000}

var _ IMeterRegistry = (*basicMeterRegistry)(nil)

type basicMeterRegistry struct {
	lock   sync.RWMutex
	meters map[string]*basicMeter
}

func NewMeterRegistry() IMeterRegistry {
	return &basicMeterRegistry{
		meters: make(map[string]*basicMeter),
	}
}

func (r *basicMeterRegistry) Meter(name string) IMeter {
	r.lock.RLock()
	if m, ok := r.meters[name]; ok {
		r.lock.RUnlock()
		return m
	}
	r.lock.RUnlock()

	r.lock.Lock()
	defer r.lock.Unlock()
	if m, ok := r.meters[name]; ok {
		return m
	}
	m := newBasicMeter(name)
	r.meters[name] = m
	return m
}

func (r *basicMeterRegistry) ForEach(fn func(name string, meter IMeter)) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	for name, meter := range r.meters {
		fn(name, meter)
	}
}

func (r *basicMeterRegistry) Collect() []MetricData {
	r.lock.RLock()
	meters := make([]*basicMeter, 0, len(r.meters))
	for _, m := range r.meters {
		meters = append(meters, m)
	}
	r.lock.RUnlock()

	result := make([]MetricData, 0, len(meters)*3)
	for _, m := range meters {
		result = append(result, m.collectMetrics()...)
	}
	return result
}

var _ IMeter = (*basicMeter)(nil)

type basicMeter struct {
	name string
	lock sync.RWMutex

	counters   map[string]float64
	gauges     map[string]float64
	histograms map[string]*histogramData
}

func newBasicMeter(name string) *basicMeter {
	return &basicMeter{
		name:       name,
		counters:   make(map[string]float64),
		gauges:     make(map[string]float64),
		histograms: make(map[string]*histogramData),
	}
}

func (m *basicMeter) Counter(name string) ICounter {
	m.lock.Lock()
	defer m.lock.Unlock()

	fullName := m.name + "." + name
	if _, ok := m.counters[fullName]; !ok {
		m.counters[fullName] = 0
	}
	return &basicCounter{meter: m, name: fullName}
}

func (m *basicMeter) Gauge(name string) IGauge {
	m.lock.Lock()
	defer m.lock.Unlock()

	fullName := m.name + "." + name
	if _, ok := m.gauges[fullName]; !ok {
		m.gauges[fullName] = 0
	}
	return &basicGauge{meter: m, name: fullName}
}

func (m *basicMeter) Histogram(name string, boundaries ...float64) IHistogram {
	m.lock.Lock()
	defer m.lock.Unlock()

	bnds := boundaries
	if len(bnds) == 0 {
		bnds = DefaultHistogramBoundaries
	}

	fullName := m.name + "." + name
	if _, ok := m.histograms[fullName]; !ok {
		m.histograms[fullName] = newHistogramData(bnds)
	}
	return &basicHistogram{meter: m, name: fullName}
}

func (m *basicMeter) collectMetrics() []MetricData {
	m.lock.RLock()
	defer m.lock.RUnlock()

	capacity := len(m.counters) + len(m.gauges) + len(m.histograms)
	result := make([]MetricData, 0, capacity)

	for name, value := range m.counters {
		result = append(result, MetricData{Name: name, Type: MetricCounter, Value: value})
	}
	for name, value := range m.gauges {
		result = append(result, MetricData{Name: name, Type: MetricGauge, Value: value})
	}
	for name, h := range m.histograms {
		bucketsMap := make(map[float64]uint64, len(h.buckets))
		for i, bound := range h.boundaries {
			bucketsMap[bound] = h.buckets[i]
		}
		if len(h.buckets) > 0 {
			bucketsMap[infPos] = h.buckets[len(h.buckets)-1]
		}
		result = append(result, MetricData{
			Name:    name,
			Type:    MetricHistogram,
			Count:   h.count,
			Sum:     h.sum,
			Min:     h.min,
			Max:     h.max,
			Buckets: bucketsMap,
		})
	}
	return result
}

type histogramData struct {
	count      uint64
	sum        float64
	min        float64
	max        float64
	boundaries []float64
	buckets    []uint64
}

func newHistogramData(boundaries []float64) *histogramData {
	bnds := make([]float64, len(boundaries))
	copy(bnds, boundaries)
	return &histogramData{
		min:        math.MaxFloat64,
		boundaries: bnds,
		buckets:    make([]uint64, len(bnds)+1),
	}
}

var _ ICounter = (*basicCounter)(nil)

type basicCounter struct {
	meter *basicMeter
	name  string
}

func (c *basicCounter) Add(value float64) {
	c.meter.lock.Lock()
	defer c.meter.lock.Unlock()
	c.meter.counters[c.name] += value
}

var _ IGauge = (*basicGauge)(nil)

type basicGauge struct {
	meter *basicMeter
	name  string
}

func (g *basicGauge) Set(value float64) {
	g.meter.lock.Lock()
	defer g.meter.lock.Unlock()
	g.meter.gauges[g.name] = value
}

func (g *basicGauge) Add(value float64) {
	g.meter.lock.Lock()
	defer g.meter.lock.Unlock()
	g.meter.gauges[g.name] += value
}

var _ IHistogram = (*basicHistogram)(nil)

type basicHistogram struct {
	meter *basicMeter
	name  string
}

func (h *basicHistogram) Record(value float64) {
	h.meter.lock.Lock()
	defer h.meter.lock.Unlock()

	data := h.meter.histograms[h.name]
	data.count++
	data.sum += value
	if value < data.min {
		data.min = value
	}
	if value > data.max {
		data.max = value
	}

	lo, hi := 0, len(data.boundaries)
	for lo < hi {
		mid := (lo + hi) / 2
		if data.boundaries[mid] < value {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	for i := lo; i < len(data.buckets); i++ {
		data.buckets[i]++
	}
}
