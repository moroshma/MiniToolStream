package metrics

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// BenchmarkResult represents complete benchmark results
type BenchmarkResult struct {
	System        string        `json:"system"`         // "minitoolstream" or "kafka"
	TestName      string        `json:"test_name"`      // "small-files-10kb" or "large-files-1gb"
	Timestamp     time.Time     `json:"timestamp"`      // When test was run
	Config        TestConfig    `json:"config"`         // Test configuration
	Throughput    Throughput    `json:"throughput"`     // Throughput metrics
	Latency       Latency       `json:"latency"`        // Latency metrics
	Resources     Resources     `json:"resources"`      // Resource usage
	Errors        ErrorMetrics  `json:"errors"`         // Error statistics
	Duration      time.Duration `json:"duration"`       // Total test duration
}

// TestConfig contains test parameters
type TestConfig struct {
	MessageSize   int64 `json:"message_size_bytes"`
	NumProducers  int   `json:"num_producers"`
	NumConsumers  int   `json:"num_consumers"`
	TotalMessages int64 `json:"total_messages"`
	TargetRPS     int   `json:"target_rps"`
}

// Throughput metrics
type Throughput struct {
	TotalMessages int64   `json:"total_messages"`
	TotalBytes    int64   `json:"total_bytes"`
	MsgPerSec     float64 `json:"msg_per_sec"`
	MBPerSec      float64 `json:"mb_per_sec"`
}

// Latency metrics
type Latency struct {
	P50Ms time.Duration `json:"p50_ms"`
	P95Ms time.Duration `json:"p95_ms"`
	P99Ms time.Duration `json:"p99_ms"`
	MinMs time.Duration `json:"min_ms"`
	MaxMs time.Duration `json:"max_ms"`
	AvgMs time.Duration `json:"avg_ms"`
}

// Resources metrics
type Resources struct {
	CPUPercent   float64 `json:"cpu_percent"`
	MemoryMB     float64 `json:"memory_mb"`
	DiskWriteMB  float64 `json:"disk_write_mb"`
	DiskReadMB   float64 `json:"disk_read_mb"`
	NetworkTxMB  float64 `json:"network_tx_mb"`
	NetworkRxMB  float64 `json:"network_rx_mb"`
}

// ErrorMetrics tracks errors
type ErrorMetrics struct {
	ErrorCount int64   `json:"error_count"`
	ErrorRate  float64 `json:"error_rate"`
}

// Collector collects metrics during benchmark
type Collector struct {
	mu              sync.RWMutex
	startTime       time.Time
	endTime         time.Time
	latencies       []time.Duration
	totalMessages   int64
	totalBytes      int64
	errorCount      int64
	config          TestConfig
	resourceSamples []Resources
}

// NewCollector creates a new metrics collector
func NewCollector(config TestConfig) *Collector {
	return &Collector{
		startTime:       time.Now(),
		latencies:       make([]time.Duration, 0, config.TotalMessages),
		config:          config,
		resourceSamples: make([]Resources, 0),
	}
}

// RecordLatency records a single message latency
func (c *Collector) RecordLatency(duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.latencies = append(c.latencies, duration)
}

// RecordMessage records a successful message
func (c *Collector) RecordMessage(size int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.totalMessages++
	c.totalBytes += size
}

// RecordError records an error
func (c *Collector) RecordError() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.errorCount++
}

// RecordResources records resource usage sample
func (c *Collector) RecordResources(res Resources) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resourceSamples = append(c.resourceSamples, res)
}

// Finalize marks the end of benchmark and calculates results
func (c *Collector) Finalize(system, testName string) *BenchmarkResult {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.endTime = time.Now()
	duration := c.endTime.Sub(c.startTime)

	// Calculate throughput
	durationSec := duration.Seconds()
	throughput := Throughput{
		TotalMessages: c.totalMessages,
		TotalBytes:    c.totalBytes,
		MsgPerSec:     float64(c.totalMessages) / durationSec,
		MBPerSec:      float64(c.totalBytes) / (1024 * 1024) / durationSec,
	}

	// Calculate latency percentiles
	latency := c.calculateLatency()

	// Calculate average resources
	resources := c.calculateAverageResources()

	// Calculate error rate
	errorRate := 0.0
	if c.totalMessages > 0 {
		errorRate = float64(c.errorCount) / float64(c.totalMessages+c.errorCount)
	}

	errors := ErrorMetrics{
		ErrorCount: c.errorCount,
		ErrorRate:  errorRate,
	}

	return &BenchmarkResult{
		System:     system,
		TestName:   testName,
		Timestamp:  c.startTime,
		Config:     c.config,
		Throughput: throughput,
		Latency:    latency,
		Resources:  resources,
		Errors:     errors,
		Duration:   duration,
	}
}

// calculateLatency calculates latency percentiles
func (c *Collector) calculateLatency() Latency {
	if len(c.latencies) == 0 {
		return Latency{}
	}

	// Sort latencies
	sorted := make([]time.Duration, len(c.latencies))
	copy(sorted, c.latencies)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	// Calculate percentiles
	p50 := sorted[len(sorted)*50/100]
	p95 := sorted[len(sorted)*95/100]
	p99 := sorted[len(sorted)*99/100]
	min := sorted[0]
	max := sorted[len(sorted)-1]

	// Calculate average
	var sum time.Duration
	for _, lat := range sorted {
		sum += lat
	}
	avg := sum / time.Duration(len(sorted))

	return Latency{
		P50Ms: p50,
		P95Ms: p95,
		P99Ms: p99,
		MinMs: min,
		MaxMs: max,
		AvgMs: avg,
	}
}

// calculateAverageResources calculates average resource usage
func (c *Collector) calculateAverageResources() Resources {
	if len(c.resourceSamples) == 0 {
		return Resources{}
	}

	var avgRes Resources
	for _, res := range c.resourceSamples {
		avgRes.CPUPercent += res.CPUPercent
		avgRes.MemoryMB += res.MemoryMB
		avgRes.DiskWriteMB += res.DiskWriteMB
		avgRes.DiskReadMB += res.DiskReadMB
		avgRes.NetworkTxMB += res.NetworkTxMB
		avgRes.NetworkRxMB += res.NetworkRxMB
	}

	count := float64(len(c.resourceSamples))
	avgRes.CPUPercent /= count
	avgRes.MemoryMB /= count
	avgRes.DiskWriteMB /= count
	avgRes.DiskReadMB /= count
	avgRes.NetworkTxMB /= count
	avgRes.NetworkRxMB /= count

	return avgRes
}

// SaveToFile saves results to JSON file
func (r *BenchmarkResult) SaveToFile(filename string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// PrintSummary prints a summary of results to stdout
func (r *BenchmarkResult) PrintSummary() {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Printf("Benchmark Results: %s - %s\n", r.System, r.TestName)
	fmt.Println(strings.Repeat("=", 80))

	fmt.Println("\n[Configuration]")
	fmt.Printf("  Message Size:     %d bytes\n", r.Config.MessageSize)
	fmt.Printf("  Producers:        %d\n", r.Config.NumProducers)
	fmt.Printf("  Consumers:        %d\n", r.Config.NumConsumers)
	fmt.Printf("  Total Messages:   %d\n", r.Config.TotalMessages)
	fmt.Printf("  Duration:         %s\n", r.Duration)

	fmt.Println("\n[Throughput]")
	fmt.Printf("  Messages/sec:     %.2f\n", r.Throughput.MsgPerSec)
	fmt.Printf("  MB/sec:           %.2f\n", r.Throughput.MBPerSec)
	fmt.Printf("  Total Messages:   %d\n", r.Throughput.TotalMessages)
	fmt.Printf("  Total Data:       %.2f MB\n", float64(r.Throughput.TotalBytes)/(1024*1024))

	fmt.Println("\n[Latency]")
	fmt.Printf("  Min:              %s\n", r.Latency.MinMs)
	fmt.Printf("  Avg:              %s\n", r.Latency.AvgMs)
	fmt.Printf("  P50:              %s\n", r.Latency.P50Ms)
	fmt.Printf("  P95:              %s\n", r.Latency.P95Ms)
	fmt.Printf("  P99:              %s\n", r.Latency.P99Ms)
	fmt.Printf("  Max:              %s\n", r.Latency.MaxMs)

	fmt.Println("\n[Resources]")
	fmt.Printf("  CPU:              %.2f%%\n", r.Resources.CPUPercent)
	fmt.Printf("  Memory:           %.2f MB\n", r.Resources.MemoryMB)
	fmt.Printf("  Disk Write:       %.2f MB\n", r.Resources.DiskWriteMB)
	fmt.Printf("  Disk Read:        %.2f MB\n", r.Resources.DiskReadMB)
	fmt.Printf("  Network TX:       %.2f MB\n", r.Resources.NetworkTxMB)
	fmt.Printf("  Network RX:       %.2f MB\n", r.Resources.NetworkRxMB)

	fmt.Println("\n[Errors]")
	fmt.Printf("  Error Count:      %d\n", r.Errors.ErrorCount)
	fmt.Printf("  Error Rate:       %.4f%%\n", r.Errors.ErrorRate*100)

	fmt.Println("\n" + strings.Repeat("=", 80) + "\n")
}
