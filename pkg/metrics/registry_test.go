package metrics

import (
	"math"
	"sync"
	"testing"
)

func TestCounter_WhenAdd_ExpectCollectReturnsSum(t *testing.T) {
	reg := NewMeterRegistry()
	c := reg.Meter("svc").Counter("req")
	c.Add(1)
	c.Add(2)
	c.Add(3)

	data := reg.Collect()
	found := findMetric(data, ".req")
	if found == nil {
		t.Fatal("counter metric not found")
	}
	if found.Type != MetricCounter {
		t.Errorf("expected MetricCounter, got %v", found.Type)
	}
	if found.Value != 6 {
		t.Errorf("expected value 6, got %v", found.Value)
	}
}

func TestGauge_WhenSet_ExpectCollectReturnsLastValue(t *testing.T) {
	reg := NewMeterRegistry()
	g := reg.Meter("svc").Gauge("connections")
	g.Set(10)
	g.Set(20)

	data := reg.Collect()
	found := findMetric(data, ".connections")
	if found == nil {
		t.Fatal("gauge metric not found")
	}
	if found.Type != MetricGauge {
		t.Errorf("expected MetricGauge, got %v", found.Type)
	}
	if found.Value != 20 {
		t.Errorf("expected value 20, got %v", found.Value)
	}
}

func TestGauge_WhenAdd_ExpectCollectReturnsAccumulatedValue(t *testing.T) {
	reg := NewMeterRegistry()
	g := reg.Meter("svc").Gauge("connections")
	g.Set(5)
	g.Add(3)
	g.Add(-2)

	data := reg.Collect()
	found := findMetric(data, ".connections")
	if found == nil {
		t.Fatal("gauge metric not found")
	}
	if found.Value != 6 {
		t.Errorf("expected value 6, got %v", found.Value)
	}
}

func TestHistogram_WhenRecord_ExpectCollectReturnsBuckets(t *testing.T) {
	reg := NewMeterRegistry()
	h := reg.Meter("svc").Histogram("latency", 10, 50, 100)
	h.Record(5)
	h.Record(30)
	h.Record(80)
	h.Record(200)

	data := reg.Collect()
	found := findMetric(data, ".latency")
	if found == nil {
		t.Fatal("histogram metric not found")
	}
	if found.Type != MetricHistogram {
		t.Errorf("expected MetricHistogram, got %v", found.Type)
	}
	if found.Count != 4 {
		t.Errorf("expected count 4, got %v", found.Count)
	}
	if found.Sum != 315 {
		t.Errorf("expected sum 315, got %v", found.Sum)
	}
	if found.Min != 5 {
		t.Errorf("expected min 5, got %v", found.Min)
	}
	if found.Max != 200 {
		t.Errorf("expected max 200, got %v", found.Max)
	}

	// bucket[10]: count of values <=10 => 5 falls here => 1
	if found.Buckets[10] != 1 {
		t.Errorf("expected bucket[10]=1, got %v", found.Buckets[10])
	}
	// bucket[50]: count of values <=50 => 5, 30 => 2
	if found.Buckets[50] != 2 {
		t.Errorf("expected bucket[50]=2, got %v", found.Buckets[50])
	}
	// bucket[100]: count of values <=100 => 5, 30, 80 => 3
	if found.Buckets[100] != 3 {
		t.Errorf("expected bucket[100]=3, got %v", found.Buckets[100])
	}
	// +Inf bucket: all 4
	if found.Buckets[math.Inf(1)] != 4 {
		t.Errorf("expected bucket[+Inf]=4, got %v", found.Buckets[math.Inf(1)])
	}
}

func TestForEach_WhenMultipleMeters_ExpectAllVisited(t *testing.T) {
	reg := NewMeterRegistry()
	reg.Meter("svc1").Counter("c1").Add(1)
	reg.Meter("svc2").Counter("c2").Add(2)
	reg.Meter("svc3").Gauge("g1").Set(3)

	visited := map[string]bool{}
	reg.ForEach(func(name string, meter IMeter) {
		visited[name] = true
	})

	for _, name := range []string{"svc1", "svc2", "svc3"} {
		if !visited[name] {
			t.Errorf("meter %q not visited", name)
		}
	}
}

func TestCounter_WhenConcurrentAdd_ExpectCorrectTotal(t *testing.T) {
	reg := NewMeterRegistry()
	c := reg.Meter("svc").Counter("concurrent")

	const goroutines = 100
	const addsPerGoroutine = 1000
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < addsPerGoroutine; j++ {
				c.Add(1)
			}
		}()
	}
	wg.Wait()

	data := reg.Collect()
	found := findMetric(data, ".concurrent")
	if found == nil {
		t.Fatal("counter metric not found")
	}
	expected := float64(goroutines * addsPerGoroutine)
	if found.Value != expected {
		t.Errorf("expected value %v, got %v", expected, found.Value)
	}
}

func TestMeterRegistry_WhenSameMeterName_ExpectSameInstance(t *testing.T) {
	reg := NewMeterRegistry()
	m1 := reg.Meter("svc")
	m2 := reg.Meter("svc")
	m1.Counter("hits").Add(5)

	data := reg.Collect()
	found := findMetric(data, ".hits")
	if found == nil {
		t.Fatal("counter metric not found")
	}
	_ = m2
	if found.Value != 5 {
		t.Errorf("expected same meter instance, got value %v", found.Value)
	}
}

func TestMetricCollectorAdapter_WhenCounter_ExpectRegistryUpdated(t *testing.T) {
	reg := NewMeterRegistry()
	adapter := NewMetricCollectorAdapter(reg)
	adapter.Counter("requests", 10)
	adapter.Counter("requests", 5)

	data := reg.Collect()
	found := findMetric(data, ".requests")
	if found == nil {
		t.Fatal("counter metric not found")
	}
	if found.Value != 15 {
		t.Errorf("expected value 15, got %v", found.Value)
	}
}

func TestMetricCollectorAdapter_WhenGauge_ExpectRegistryUpdated(t *testing.T) {
	reg := NewMeterRegistry()
	adapter := NewMetricCollectorAdapter(reg)
	adapter.Gauge("players", 42)

	data := reg.Collect()
	found := findMetric(data, ".players")
	if found == nil {
		t.Fatal("gauge metric not found")
	}
	if found.Value != 42 {
		t.Errorf("expected value 42, got %v", found.Value)
	}
}

func TestMetricCollectorAdapter_WhenHistogram_ExpectRegistryUpdated(t *testing.T) {
	reg := NewMeterRegistry()
	adapter := NewMetricCollectorAdapter(reg)
	adapter.Histogram("rtt", 100.0)
	adapter.Histogram("rtt", 200.0)

	data := reg.Collect()
	found := findMetric(data, ".rtt")
	if found == nil {
		t.Fatal("histogram metric not found")
	}
	if found.Count != 2 {
		t.Errorf("expected count 2, got %v", found.Count)
	}
	if found.Sum != 300.0 {
		t.Errorf("expected sum 300.0, got %v", found.Sum)
	}
}

func TestMetricCollectorAdapter_WhenCounterZero_ExpectNoChange(t *testing.T) {
	reg := NewMeterRegistry()
	adapter := NewMetricCollectorAdapter(reg)
	adapter.Counter("noop", 0)
	adapter.Counter("real", 5)

	data := reg.Collect()
	found := findMetric(data, ".real")
	if found == nil {
		t.Fatal("counter metric not found")
	}
	if found.Value != 5 {
		t.Errorf("expected value 5, got %v", found.Value)
	}
}

func findMetric(data []MetricData, suffix string) *MetricData {
	for i := range data {
		name := data[i].Name
		if len(name) >= len(suffix) && name[len(name)-len(suffix):] == suffix {
			return &data[i]
		}
	}
	return nil
}
