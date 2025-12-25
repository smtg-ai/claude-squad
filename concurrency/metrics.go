package concurrency

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// MetricType represents the type of metric
type MetricType int

const (
	MetricTypeCounter MetricType = iota
	MetricTypeGauge
	MetricTypeHistogram
	MetricTypeTimer
)

func (mt MetricType) String() string {
	switch mt {
	case MetricTypeCounter:
		return "counter"
	case MetricTypeGauge:
		return "gauge"
	case MetricTypeHistogram:
		return "histogram"
	case MetricTypeTimer:
		return "timer"
	default:
		return "unknown"
	}
}

// Counter is a monotonically increasing metric
type Counter struct {
	value uint64
	name  string
}

// NewCounter creates a new counter
func NewCounter(name string) *Counter {
	return &Counter{name: name}
}

// Inc increments the counter by 1
func (c *Counter) Inc() {
	atomic.AddUint64(&c.value, 1)
}

// Add adds the given value to the counter
func (c *Counter) Add(delta uint64) {
	atomic.AddUint64(&c.value, delta)
}

// Get returns the current value
func (c *Counter) Get() uint64 {
	return atomic.LoadUint64(&c.value)
}

// Reset resets the counter to 0
func (c *Counter) Reset() {
	atomic.StoreUint64(&c.value, 0)
}

// Name returns the counter name
func (c *Counter) Name() string {
	return c.name
}

// Gauge is a metric that can go up and down
type Gauge struct {
	value uint64 // stored as bits of float64
	name  string
}

// NewGauge creates a new gauge
func NewGauge(name string) *Gauge {
	return &Gauge{name: name}
}

// Set sets the gauge to the given value
func (g *Gauge) Set(value float64) {
	atomic.StoreUint64(&g.value, math.Float64bits(value))
}

// Inc increments the gauge by 1
func (g *Gauge) Inc() {
	g.Add(1.0)
}

// Dec decrements the gauge by 1
func (g *Gauge) Dec() {
	g.Add(-1.0)
}

// Add adds the given delta to the gauge
func (g *Gauge) Add(delta float64) {
	for {
		old := atomic.LoadUint64(&g.value)
		oldVal := math.Float64frombits(old)
		newVal := oldVal + delta
		if atomic.CompareAndSwapUint64(&g.value, old, math.Float64bits(newVal)) {
			return
		}
	}
}

// Get returns the current value
func (g *Gauge) Get() float64 {
	return math.Float64frombits(atomic.LoadUint64(&g.value))
}

// Name returns the gauge name
func (g *Gauge) Name() string {
	return g.name
}

// Histogram tracks the distribution of values
type Histogram struct {
	mu      sync.RWMutex
	name    string
	samples []float64
	count   uint64
	sum     uint64 // stored as bits of float64
	min     uint64 // stored as bits of float64
	max     uint64 // stored as bits of float64
}

// NewHistogram creates a new histogram
func NewHistogram(name string) *Histogram {
	return &Histogram{
		name:    name,
		samples: make([]float64, 0, 1000),
		min:     math.Float64bits(math.MaxFloat64),
		max:     math.Float64bits(0),
	}
}

// Observe records a new observation
func (h *Histogram) Observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.samples = append(h.samples, value)
	atomic.AddUint64(&h.count, 1)

	// Update sum
	for {
		old := atomic.LoadUint64(&h.sum)
		oldVal := math.Float64frombits(old)
		newVal := oldVal + value
		if atomic.CompareAndSwapUint64(&h.sum, old, math.Float64bits(newVal)) {
			break
		}
	}

	// Update min
	for {
		old := atomic.LoadUint64(&h.min)
		oldVal := math.Float64frombits(old)
		if value < oldVal {
			if atomic.CompareAndSwapUint64(&h.min, old, math.Float64bits(value)) {
				break
			}
		} else {
			break
		}
	}

	// Update max
	for {
		old := atomic.LoadUint64(&h.max)
		oldVal := math.Float64frombits(old)
		if value > oldVal {
			if atomic.CompareAndSwapUint64(&h.max, old, math.Float64bits(value)) {
				break
			}
		} else {
			break
		}
	}
}

// Count returns the number of observations
func (h *Histogram) Count() uint64 {
	return atomic.LoadUint64(&h.count)
}

// Sum returns the sum of all observations
func (h *Histogram) Sum() float64 {
	return math.Float64frombits(atomic.LoadUint64(&h.sum))
}

// Mean returns the average of all observations
func (h *Histogram) Mean() float64 {
	count := h.Count()
	if count == 0 {
		return 0
	}
	return h.Sum() / float64(count)
}

// Min returns the minimum observed value
func (h *Histogram) Min() float64 {
	min := math.Float64frombits(atomic.LoadUint64(&h.min))
	if min == math.MaxFloat64 {
		return 0
	}
	return min
}

// Max returns the maximum observed value
func (h *Histogram) Max() float64 {
	return math.Float64frombits(atomic.LoadUint64(&h.max))
}

// Percentile returns the value at the given percentile (0-100)
func (h *Histogram) Percentile(p float64) float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.samples) == 0 {
		return 0
	}

	// Create a sorted copy
	sorted := make([]float64, len(h.samples))
	copy(sorted, h.samples)
	sort.Float64s(sorted)

	index := int(float64(len(sorted)) * p / 100.0)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

// Name returns the histogram name
func (h *Histogram) Name() string {
	return h.name
}

// Reset clears all observations
func (h *Histogram) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.samples = make([]float64, 0, 1000)
	atomic.StoreUint64(&h.count, 0)
	atomic.StoreUint64(&h.sum, 0)
	atomic.StoreUint64(&h.min, math.Float64bits(math.MaxFloat64))
	atomic.StoreUint64(&h.max, 0)
}

// Timer measures durations and provides timing statistics
type Timer struct {
	histogram *Histogram
	name      string
}

// NewTimer creates a new timer
func NewTimer(name string) *Timer {
	return &Timer{
		name:      name,
		histogram: NewHistogram(name),
	}
}

// Record records a duration
func (t *Timer) Record(duration time.Duration) {
	t.histogram.Observe(duration.Seconds())
}

// Time returns a function that when called, records the elapsed time
func (t *Timer) Time() func() {
	start := time.Now()
	return func() {
		t.Record(time.Since(start))
	}
}

// Count returns the number of recorded durations
func (t *Timer) Count() uint64 {
	return t.histogram.Count()
}

// Mean returns the average duration
func (t *Timer) Mean() time.Duration {
	return time.Duration(t.histogram.Mean() * float64(time.Second))
}

// Min returns the minimum duration
func (t *Timer) Min() time.Duration {
	return time.Duration(t.histogram.Min() * float64(time.Second))
}

// Max returns the maximum duration
func (t *Timer) Max() time.Duration {
	return time.Duration(t.histogram.Max() * float64(time.Second))
}

// Percentile returns the duration at the given percentile
func (t *Timer) Percentile(p float64) time.Duration {
	return time.Duration(t.histogram.Percentile(p) * float64(time.Second))
}

// Name returns the timer name
func (t *Timer) Name() string {
	return t.name
}

// Reset clears all recorded durations
func (t *Timer) Reset() {
	t.histogram.Reset()
}

// RollingWindow maintains statistics over a sliding time window
type RollingWindow struct {
	mu       sync.RWMutex
	name     string
	window   time.Duration
	buckets  []bucketData
	current  int
	lastTick time.Time
}

type bucketData struct {
	timestamp time.Time
	count     uint64
	sum       float64
}

// NewRollingWindow creates a new rolling window with the given duration and bucket count
func NewRollingWindow(name string, window time.Duration, buckets int) *RollingWindow {
	if buckets <= 0 {
		buckets = 60
	}
	rw := &RollingWindow{
		name:     name,
		window:   window,
		buckets:  make([]bucketData, buckets),
		lastTick: time.Now(),
	}
	return rw
}

// Add records a value in the current bucket
func (rw *RollingWindow) Add(value float64) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	now := time.Now()
	rw.rotate(now)

	rw.buckets[rw.current].count++
	rw.buckets[rw.current].sum += value
}

// Inc increments the count in the current bucket
func (rw *RollingWindow) Inc() {
	rw.Add(1.0)
}

// rotate advances the window if necessary
func (rw *RollingWindow) rotate(now time.Time) {
	bucketDuration := rw.window / time.Duration(len(rw.buckets))
	elapsed := now.Sub(rw.lastTick)

	if elapsed >= bucketDuration {
		bucketsToRotate := int(elapsed / bucketDuration)
		if bucketsToRotate >= len(rw.buckets) {
			// Clear all buckets
			for i := range rw.buckets {
				rw.buckets[i] = bucketData{timestamp: now}
			}
		} else {
			// Rotate buckets
			for i := 0; i < bucketsToRotate; i++ {
				rw.current = (rw.current + 1) % len(rw.buckets)
				rw.buckets[rw.current] = bucketData{timestamp: now}
			}
		}
		rw.lastTick = now
	}
}

// Sum returns the sum of all values in the window
func (rw *RollingWindow) Sum() float64 {
	rw.mu.RLock()
	defer rw.mu.RUnlock()

	rw.rotate(time.Now())

	var sum float64
	cutoff := time.Now().Add(-rw.window)
	for _, bucket := range rw.buckets {
		if bucket.timestamp.After(cutoff) {
			sum += bucket.sum
		}
	}
	return sum
}

// Count returns the count of all values in the window
func (rw *RollingWindow) Count() uint64 {
	rw.mu.RLock()
	defer rw.mu.RUnlock()

	rw.rotate(time.Now())

	var count uint64
	cutoff := time.Now().Add(-rw.window)
	for _, bucket := range rw.buckets {
		if bucket.timestamp.After(cutoff) {
			count += bucket.count
		}
	}
	return count
}

// Rate returns the average rate per second over the window
func (rw *RollingWindow) Rate() float64 {
	count := rw.Count()
	return float64(count) / rw.window.Seconds()
}

// Name returns the rolling window name
func (rw *RollingWindow) Name() string {
	return rw.name
}

// AgentMetrics tracks metrics for a specific agent
type AgentMetrics struct {
	mu             sync.RWMutex
	agentID        string
	tasksCompleted *Counter
	tasksActive    *Gauge
	taskDuration   *Timer
	errors         *Counter
	operations     map[string]*Timer
}

// NewAgentMetrics creates metrics for an agent
func NewAgentMetrics(agentID string) *AgentMetrics {
	return &AgentMetrics{
		agentID:        agentID,
		tasksCompleted: NewCounter(fmt.Sprintf("agent_%s_tasks_completed", agentID)),
		tasksActive:    NewGauge(fmt.Sprintf("agent_%s_tasks_active", agentID)),
		taskDuration:   NewTimer(fmt.Sprintf("agent_%s_task_duration", agentID)),
		errors:         NewCounter(fmt.Sprintf("agent_%s_errors", agentID)),
		operations:     make(map[string]*Timer),
	}
}

// TaskStart marks the start of a task
func (am *AgentMetrics) TaskStart() {
	am.tasksActive.Inc()
}

// TaskComplete marks the completion of a task
func (am *AgentMetrics) TaskComplete(duration time.Duration) {
	am.tasksActive.Dec()
	am.tasksCompleted.Inc()
	am.taskDuration.Record(duration)
}

// RecordError records an error
func (am *AgentMetrics) RecordError() {
	am.errors.Inc()
}

// RecordOperation records the duration of a specific operation
func (am *AgentMetrics) RecordOperation(operation string, duration time.Duration) {
	am.mu.Lock()
	timer, exists := am.operations[operation]
	if !exists {
		timer = NewTimer(fmt.Sprintf("agent_%s_operation_%s", am.agentID, operation))
		am.operations[operation] = timer
	}
	am.mu.Unlock()

	timer.Record(duration)
}

// TimeOperation returns a function that records the operation duration when called
func (am *AgentMetrics) TimeOperation(operation string) func() {
	start := time.Now()
	return func() {
		am.RecordOperation(operation, time.Since(start))
	}
}

// GetAgentID returns the agent ID
func (am *AgentMetrics) GetAgentID() string {
	return am.agentID
}

// GetTasksCompleted returns total completed tasks
func (am *AgentMetrics) GetTasksCompleted() uint64 {
	return am.tasksCompleted.Get()
}

// GetTasksActive returns current active tasks
func (am *AgentMetrics) GetTasksActive() float64 {
	return am.tasksActive.Get()
}

// GetErrorCount returns total errors
func (am *AgentMetrics) GetErrorCount() uint64 {
	return am.errors.Get()
}

// MetricRegistry manages all metrics in the system
type MetricRegistry struct {
	mu          sync.RWMutex
	counters    map[string]*Counter
	gauges      map[string]*Gauge
	histograms  map[string]*Histogram
	timers      map[string]*Timer
	rollingWins map[string]*RollingWindow
	agents      map[string]*AgentMetrics
}

// NewMetricRegistry creates a new metric registry
func NewMetricRegistry() *MetricRegistry {
	return &MetricRegistry{
		counters:    make(map[string]*Counter),
		gauges:      make(map[string]*Gauge),
		histograms:  make(map[string]*Histogram),
		timers:      make(map[string]*Timer),
		rollingWins: make(map[string]*RollingWindow),
		agents:      make(map[string]*AgentMetrics),
	}
}

// RegisterCounter registers a new counter
func (mr *MetricRegistry) RegisterCounter(name string) *Counter {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	if c, exists := mr.counters[name]; exists {
		return c
	}

	c := NewCounter(name)
	mr.counters[name] = c
	return c
}

// RegisterGauge registers a new gauge
func (mr *MetricRegistry) RegisterGauge(name string) *Gauge {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	if g, exists := mr.gauges[name]; exists {
		return g
	}

	g := NewGauge(name)
	mr.gauges[name] = g
	return g
}

// RegisterHistogram registers a new histogram
func (mr *MetricRegistry) RegisterHistogram(name string) *Histogram {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	if h, exists := mr.histograms[name]; exists {
		return h
	}

	h := NewHistogram(name)
	mr.histograms[name] = h
	return h
}

// RegisterTimer registers a new timer
func (mr *MetricRegistry) RegisterTimer(name string) *Timer {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	if t, exists := mr.timers[name]; exists {
		return t
	}

	t := NewTimer(name)
	mr.timers[name] = t
	return t
}

// RegisterRollingWindow registers a new rolling window
func (mr *MetricRegistry) RegisterRollingWindow(name string, window time.Duration, buckets int) *RollingWindow {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	if rw, exists := mr.rollingWins[name]; exists {
		return rw
	}

	rw := NewRollingWindow(name, window, buckets)
	mr.rollingWins[name] = rw
	return rw
}

// RegisterAgent registers a new agent metrics tracker
func (mr *MetricRegistry) RegisterAgent(agentID string) *AgentMetrics {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	if am, exists := mr.agents[agentID]; exists {
		return am
	}

	am := NewAgentMetrics(agentID)
	mr.agents[agentID] = am
	return am
}

// GetCounter retrieves a counter by name
func (mr *MetricRegistry) GetCounter(name string) (*Counter, bool) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	c, exists := mr.counters[name]
	return c, exists
}

// GetGauge retrieves a gauge by name
func (mr *MetricRegistry) GetGauge(name string) (*Gauge, bool) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	g, exists := mr.gauges[name]
	return g, exists
}

// GetHistogram retrieves a histogram by name
func (mr *MetricRegistry) GetHistogram(name string) (*Histogram, bool) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	h, exists := mr.histograms[name]
	return h, exists
}

// GetTimer retrieves a timer by name
func (mr *MetricRegistry) GetTimer(name string) (*Timer, bool) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	t, exists := mr.timers[name]
	return t, exists
}

// GetRollingWindow retrieves a rolling window by name
func (mr *MetricRegistry) GetRollingWindow(name string) (*RollingWindow, bool) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	rw, exists := mr.rollingWins[name]
	return rw, exists
}

// GetAgent retrieves agent metrics by ID
func (mr *MetricRegistry) GetAgent(agentID string) (*AgentMetrics, bool) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	am, exists := mr.agents[agentID]
	return am, exists
}

// MetricSnapshot represents a point-in-time snapshot of all metrics
type MetricSnapshot struct {
	Timestamp   time.Time                    `json:"timestamp"`
	Counters    map[string]uint64            `json:"counters"`
	Gauges      map[string]float64           `json:"gauges"`
	Histograms  map[string]HistogramSnapshot `json:"histograms"`
	Timers      map[string]TimerSnapshot     `json:"timers"`
	RollingWins map[string]RollingSnapshot   `json:"rolling_windows"`
	Agents      map[string]AgentSnapshot     `json:"agents"`
}

// HistogramSnapshot contains histogram statistics
type HistogramSnapshot struct {
	Count uint64  `json:"count"`
	Sum   float64 `json:"sum"`
	Mean  float64 `json:"mean"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	P50   float64 `json:"p50"`
	P95   float64 `json:"p95"`
	P99   float64 `json:"p99"`
}

// TimerSnapshot contains timer statistics
type TimerSnapshot struct {
	Count uint64  `json:"count"`
	Mean  float64 `json:"mean_seconds"`
	Min   float64 `json:"min_seconds"`
	Max   float64 `json:"max_seconds"`
	P50   float64 `json:"p50_seconds"`
	P95   float64 `json:"p95_seconds"`
	P99   float64 `json:"p99_seconds"`
}

// RollingSnapshot contains rolling window statistics
type RollingSnapshot struct {
	Count uint64  `json:"count"`
	Sum   float64 `json:"sum"`
	Rate  float64 `json:"rate_per_second"`
}

// AgentSnapshot contains agent metrics snapshot
type AgentSnapshot struct {
	TasksCompleted uint64                   `json:"tasks_completed"`
	TasksActive    float64                  `json:"tasks_active"`
	Errors         uint64                   `json:"errors"`
	TaskDuration   TimerSnapshot            `json:"task_duration"`
	Operations     map[string]TimerSnapshot `json:"operations"`
}

// Snapshot captures a point-in-time snapshot of all metrics
func (mr *MetricRegistry) Snapshot() *MetricSnapshot {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	snapshot := &MetricSnapshot{
		Timestamp:   time.Now(),
		Counters:    make(map[string]uint64),
		Gauges:      make(map[string]float64),
		Histograms:  make(map[string]HistogramSnapshot),
		Timers:      make(map[string]TimerSnapshot),
		RollingWins: make(map[string]RollingSnapshot),
		Agents:      make(map[string]AgentSnapshot),
	}

	// Capture counters
	for name, counter := range mr.counters {
		snapshot.Counters[name] = counter.Get()
	}

	// Capture gauges
	for name, gauge := range mr.gauges {
		snapshot.Gauges[name] = gauge.Get()
	}

	// Capture histograms
	for name, hist := range mr.histograms {
		snapshot.Histograms[name] = HistogramSnapshot{
			Count: hist.Count(),
			Sum:   hist.Sum(),
			Mean:  hist.Mean(),
			Min:   hist.Min(),
			Max:   hist.Max(),
			P50:   hist.Percentile(50),
			P95:   hist.Percentile(95),
			P99:   hist.Percentile(99),
		}
	}

	// Capture timers
	for name, timer := range mr.timers {
		snapshot.Timers[name] = TimerSnapshot{
			Count: timer.Count(),
			Mean:  timer.Mean().Seconds(),
			Min:   timer.Min().Seconds(),
			Max:   timer.Max().Seconds(),
			P50:   timer.Percentile(50).Seconds(),
			P95:   timer.Percentile(95).Seconds(),
			P99:   timer.Percentile(99).Seconds(),
		}
	}

	// Capture rolling windows
	for name, rw := range mr.rollingWins {
		snapshot.RollingWins[name] = RollingSnapshot{
			Count: rw.Count(),
			Sum:   rw.Sum(),
			Rate:  rw.Rate(),
		}
	}

	// Capture agent metrics
	for agentID, agent := range mr.agents {
		agent.mu.RLock()
		agentSnap := AgentSnapshot{
			TasksCompleted: agent.tasksCompleted.Get(),
			TasksActive:    agent.tasksActive.Get(),
			Errors:         agent.errors.Get(),
			TaskDuration: TimerSnapshot{
				Count: agent.taskDuration.Count(),
				Mean:  agent.taskDuration.Mean().Seconds(),
				Min:   agent.taskDuration.Min().Seconds(),
				Max:   agent.taskDuration.Max().Seconds(),
				P50:   agent.taskDuration.Percentile(50).Seconds(),
				P95:   agent.taskDuration.Percentile(95).Seconds(),
				P99:   agent.taskDuration.Percentile(99).Seconds(),
			},
			Operations: make(map[string]TimerSnapshot),
		}

		for opName, opTimer := range agent.operations {
			agentSnap.Operations[opName] = TimerSnapshot{
				Count: opTimer.Count(),
				Mean:  opTimer.Mean().Seconds(),
				Min:   opTimer.Min().Seconds(),
				Max:   opTimer.Max().Seconds(),
				P50:   opTimer.Percentile(50).Seconds(),
				P95:   opTimer.Percentile(95).Seconds(),
				P99:   opTimer.Percentile(99).Seconds(),
			}
		}
		agent.mu.RUnlock()

		snapshot.Agents[agentID] = agentSnap
	}

	return snapshot
}

// ExportJSON exports the current metrics as JSON
func (mr *MetricRegistry) ExportJSON() ([]byte, error) {
	snapshot := mr.Snapshot()
	return json.MarshalIndent(snapshot, "", "  ")
}

// ExportPrometheus exports metrics in Prometheus text format
func (mr *MetricRegistry) ExportPrometheus() string {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	var builder strings.Builder

	// Export counters
	for name, counter := range mr.counters {
		fmt.Fprintf(&builder, "# TYPE %s counter\n", name)
		fmt.Fprintf(&builder, "%s %d\n\n", name, counter.Get())
	}

	// Export gauges
	for name, gauge := range mr.gauges {
		fmt.Fprintf(&builder, "# TYPE %s gauge\n", name)
		fmt.Fprintf(&builder, "%s %.6f\n\n", name, gauge.Get())
	}

	// Export histograms
	for name, hist := range mr.histograms {
		count := hist.Count()
		if count == 0 {
			continue
		}
		fmt.Fprintf(&builder, "# TYPE %s histogram\n", name)
		fmt.Fprintf(&builder, "%s_count %d\n", name, count)
		fmt.Fprintf(&builder, "%s_sum %.6f\n", name, hist.Sum())
		fmt.Fprintf(&builder, "%s_min %.6f\n", name, hist.Min())
		fmt.Fprintf(&builder, "%s_max %.6f\n", name, hist.Max())
		fmt.Fprintf(&builder, "%s{quantile=\"0.5\"} %.6f\n", name, hist.Percentile(50))
		fmt.Fprintf(&builder, "%s{quantile=\"0.95\"} %.6f\n", name, hist.Percentile(95))
		fmt.Fprintf(&builder, "%s{quantile=\"0.99\"} %.6f\n\n", name, hist.Percentile(99))
	}

	// Export timers
	for name, timer := range mr.timers {
		count := timer.Count()
		if count == 0 {
			continue
		}
		fmt.Fprintf(&builder, "# TYPE %s histogram\n", name)
		fmt.Fprintf(&builder, "%s_count %d\n", name, count)
		fmt.Fprintf(&builder, "%s_sum %.6f\n", name, timer.Mean().Seconds()*float64(count))
		fmt.Fprintf(&builder, "%s_min %.6f\n", name, timer.Min().Seconds())
		fmt.Fprintf(&builder, "%s_max %.6f\n", name, timer.Max().Seconds())
		fmt.Fprintf(&builder, "%s{quantile=\"0.5\"} %.6f\n", name, timer.Percentile(50).Seconds())
		fmt.Fprintf(&builder, "%s{quantile=\"0.95\"} %.6f\n", name, timer.Percentile(95).Seconds())
		fmt.Fprintf(&builder, "%s{quantile=\"0.99\"} %.6f\n\n", name, timer.Percentile(99).Seconds())
	}

	// Export rolling windows
	for name, rw := range mr.rollingWins {
		fmt.Fprintf(&builder, "# TYPE %s_count counter\n", name)
		fmt.Fprintf(&builder, "%s_count %d\n", name, rw.Count())
		fmt.Fprintf(&builder, "# TYPE %s_rate gauge\n", name)
		fmt.Fprintf(&builder, "%s_rate %.6f\n\n", name, rw.Rate())
	}

	// Export agent metrics
	for agentID, agent := range mr.agents {
		agent.mu.RLock()
		fmt.Fprintf(&builder, "# Agent: %s\n", agentID)
		fmt.Fprintf(&builder, "# TYPE %s counter\n", agent.tasksCompleted.Name())
		fmt.Fprintf(&builder, "%s %d\n", agent.tasksCompleted.Name(), agent.tasksCompleted.Get())
		fmt.Fprintf(&builder, "# TYPE %s gauge\n", agent.tasksActive.Name())
		fmt.Fprintf(&builder, "%s %.6f\n", agent.tasksActive.Name(), agent.tasksActive.Get())
		fmt.Fprintf(&builder, "# TYPE %s counter\n", agent.errors.Name())
		fmt.Fprintf(&builder, "%s %d\n\n", agent.errors.Name(), agent.errors.Get())
		agent.mu.RUnlock()
	}

	return builder.String()
}

// MetricsCollector is the main collector that orchestrates all metrics
type MetricsCollector struct {
	registry       *MetricRegistry
	globalCounters struct {
		totalTasks   *Counter
		totalErrors  *Counter
		activeAgents *Gauge
	}
	globalTimers struct {
		taskExecution *Timer
		systemLatency *Timer
	}
	globalWindows struct {
		requestRate *RollingWindow
		errorRate   *RollingWindow
	}
}

// NewMetricsCollector creates a new metrics collector with pre-configured global metrics
func NewMetricsCollector() *MetricsCollector {
	mc := &MetricsCollector{
		registry: NewMetricRegistry(),
	}

	// Initialize global counters
	mc.globalCounters.totalTasks = mc.registry.RegisterCounter("total_tasks")
	mc.globalCounters.totalErrors = mc.registry.RegisterCounter("total_errors")
	mc.globalCounters.activeAgents = mc.registry.RegisterGauge("active_agents")

	// Initialize global timers
	mc.globalTimers.taskExecution = mc.registry.RegisterTimer("task_execution_time")
	mc.globalTimers.systemLatency = mc.registry.RegisterTimer("system_latency")

	// Initialize rolling windows (1 minute window, 60 buckets = 1 second per bucket)
	mc.globalWindows.requestRate = mc.registry.RegisterRollingWindow("request_rate", time.Minute, 60)
	mc.globalWindows.errorRate = mc.registry.RegisterRollingWindow("error_rate", time.Minute, 60)

	return mc
}

// Registry returns the underlying metric registry
func (mc *MetricsCollector) Registry() *MetricRegistry {
	return mc.registry
}

// RecordTask records a task execution
func (mc *MetricsCollector) RecordTask(duration time.Duration, err error) {
	mc.globalCounters.totalTasks.Inc()
	mc.globalTimers.taskExecution.Record(duration)
	mc.globalWindows.requestRate.Inc()

	if err != nil {
		mc.globalCounters.totalErrors.Inc()
		mc.globalWindows.errorRate.Inc()
	}
}

// SetActiveAgents sets the number of active agents
func (mc *MetricsCollector) SetActiveAgents(count float64) {
	mc.globalCounters.activeAgents.Set(count)
}

// RecordLatency records system latency
func (mc *MetricsCollector) RecordLatency(duration time.Duration) {
	mc.globalTimers.systemLatency.Record(duration)
}

// GetAgentMetrics retrieves or creates agent metrics
func (mc *MetricsCollector) GetAgentMetrics(agentID string) *AgentMetrics {
	return mc.registry.RegisterAgent(agentID)
}

// Snapshot returns a point-in-time snapshot of all metrics
func (mc *MetricsCollector) Snapshot() *MetricSnapshot {
	return mc.registry.Snapshot()
}

// ExportJSON exports metrics as JSON
func (mc *MetricsCollector) ExportJSON() ([]byte, error) {
	return mc.registry.ExportJSON()
}

// ExportPrometheus exports metrics in Prometheus format
func (mc *MetricsCollector) ExportPrometheus() string {
	return mc.registry.ExportPrometheus()
}

// GetTotalTasks returns total tasks counter
func (mc *MetricsCollector) GetTotalTasks() uint64 {
	return mc.globalCounters.totalTasks.Get()
}

// GetTotalErrors returns total errors counter
func (mc *MetricsCollector) GetTotalErrors() uint64 {
	return mc.globalCounters.totalErrors.Get()
}

// GetRequestRate returns the current request rate per second
func (mc *MetricsCollector) GetRequestRate() float64 {
	return mc.globalWindows.requestRate.Rate()
}

// GetErrorRate returns the current error rate per second
func (mc *MetricsCollector) GetErrorRate() float64 {
	return mc.globalWindows.errorRate.Rate()
}
