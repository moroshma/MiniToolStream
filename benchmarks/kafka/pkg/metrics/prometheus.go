package metrics

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

// PrometheusExporter exports metrics to Prometheus Pushgateway
type PrometheusExporter struct {
	pushgatewayURL string
	jobName        string
	instance       string

	// Metrics
	messagesTotal   prometheus.Counter
	bytesTotal      prometheus.Counter
	latencyHistogram prometheus.Histogram
	errorsTotal     prometheus.Counter

	registry *prometheus.Registry
	pusher   *push.Pusher
	mu       sync.Mutex
}

// NewPrometheusExporter creates a new Prometheus exporter
func NewPrometheusExporter(pushgatewayURL, system, testName, instance string) *PrometheusExporter {
	registry := prometheus.NewRegistry()

	labels := prometheus.Labels{
		"system":    system,
		"test_name": testName,
		"instance":  instance,
	}

	// Messages total counter
	messagesTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:        "benchmark_messages_total",
		Help:        "Total number of messages processed",
		ConstLabels: labels,
	})

	// Bytes total counter
	bytesTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:        "benchmark_bytes_total",
		Help:        "Total bytes processed",
		ConstLabels: labels,
	})

	// Latency histogram
	latencyHistogram := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:        "benchmark_latency_seconds",
		Help:        "Message latency in seconds",
		ConstLabels: labels,
		Buckets:     prometheus.ExponentialBuckets(0.001, 2, 15), // 1ms to ~16s
	})

	// Errors total counter
	errorsTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:        "benchmark_errors_total",
		Help:        "Total number of errors",
		ConstLabels: labels,
	})

	// Register metrics
	registry.MustRegister(messagesTotal)
	registry.MustRegister(bytesTotal)
	registry.MustRegister(latencyHistogram)
	registry.MustRegister(errorsTotal)

	// Create pusher
	// Note: "instance" is already in ConstLabels, so we don't use Grouping for it
	pusher := push.New(pushgatewayURL, fmt.Sprintf("%s-%s", system, testName)).
		Gatherer(registry)

	return &PrometheusExporter{
		pushgatewayURL:   pushgatewayURL,
		jobName:          fmt.Sprintf("%s-%s", system, testName),
		instance:         instance,
		messagesTotal:    messagesTotal,
		bytesTotal:       bytesTotal,
		latencyHistogram: latencyHistogram,
		errorsTotal:      errorsTotal,
		registry:         registry,
		pusher:           pusher,
	}
}

// RecordMessage records a successful message
func (e *PrometheusExporter) RecordMessage(size int64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.messagesTotal.Inc()
	e.bytesTotal.Add(float64(size))
}

// RecordLatency records message latency
func (e *PrometheusExporter) RecordLatency(duration time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.latencyHistogram.Observe(duration.Seconds())
}

// RecordError records an error
func (e *PrometheusExporter) RecordError() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.errorsTotal.Inc()
}

// Push pushes metrics to Pushgateway
func (e *PrometheusExporter) Push() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.pusher.Push(); err != nil {
		return fmt.Errorf("failed to push metrics: %w", err)
	}

	return nil
}

// StartPeriodicPush starts periodic push to Pushgateway
func (e *PrometheusExporter) StartPeriodicPush(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Final push before exit
			if err := e.Push(); err != nil {
				log.Printf("Failed to push final metrics: %v", err)
			}
			return
		case <-ticker.C:
			if err := e.Push(); err != nil {
				log.Printf("Failed to push metrics: %v", err)
			}
		}
	}
}

// Delete deletes metrics from Pushgateway
func (e *PrometheusExporter) Delete() error {
	return e.pusher.Delete()
}

// PrometheusCollectorWrapper wraps standard Collector with Prometheus export
type PrometheusCollectorWrapper struct {
	*Collector
	prometheusExporter *PrometheusExporter
}

// NewPrometheusCollectorWrapper creates a wrapped collector with Prometheus export
func NewPrometheusCollectorWrapper(config TestConfig, pushgatewayURL, system, testName, instance string) *PrometheusCollectorWrapper {
	collector := NewCollector(config)
	prometheusExporter := NewPrometheusExporter(pushgatewayURL, system, testName, instance)

	return &PrometheusCollectorWrapper{
		Collector:          collector,
		prometheusExporter: prometheusExporter,
	}
}

// RecordMessage records to both collectors
func (w *PrometheusCollectorWrapper) RecordMessage(size int64) {
	w.Collector.RecordMessage(size)
	w.prometheusExporter.RecordMessage(size)
}

// RecordLatency records to both collectors
func (w *PrometheusCollectorWrapper) RecordLatency(duration time.Duration) {
	w.Collector.RecordLatency(duration)
	w.prometheusExporter.RecordLatency(duration)
}

// RecordError records to both collectors
func (w *PrometheusCollectorWrapper) RecordError() {
	w.Collector.RecordError()
	w.prometheusExporter.RecordError()
}

// StartPeriodicPush starts periodic Prometheus push
func (w *PrometheusCollectorWrapper) StartPeriodicPush(ctx context.Context, interval time.Duration) {
	go w.prometheusExporter.StartPeriodicPush(ctx, interval)
}

// Push pushes current metrics to Prometheus
func (w *PrometheusCollectorWrapper) Push() error {
	return w.prometheusExporter.Push()
}
