package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/moroshma/MiniToolStreamConnector/minitoolstream_connector"
	"benchmarks/minitoolstream/pkg/metrics"
)

type Config struct {
	Server struct {
		Address string        `yaml:"address"`
		Timeout time.Duration `yaml:"timeout"`
	} `yaml:"server"`
	Test struct {
		Name          string        `yaml:"name"`
		MessageSize   int64         `yaml:"message_size"`
		TotalMessages int64         `yaml:"total_messages"`
		NumProducers  int           `yaml:"num_producers"`
		NumConsumers  int           `yaml:"num_consumers"`
		TargetRPS     int           `yaml:"target_rps"`
		Duration      time.Duration `yaml:"duration"`
		Warmup        time.Duration `yaml:"warmup"`
	} `yaml:"test"`
	Subject    string `yaml:"subject"`
	Monitoring struct {
		Enabled    bool          `yaml:"enabled"`
		Interval   time.Duration `yaml:"interval"`
		Containers []string      `yaml:"containers"`
	} `yaml:"monitoring"`
	Prometheus struct {
		Enabled        bool          `yaml:"enabled"`
		PushgatewayURL string        `yaml:"pushgateway_url"`
		PushInterval   time.Duration `yaml:"push_interval"`
		Instance       string        `yaml:"instance"`
	} `yaml:"prometheus"`
	Output struct {
		ResultsDir     string `yaml:"results_dir"`
		FilenamePrefix string `yaml:"filename_prefix"`
		PrintSummary   bool   `yaml:"print_summary"`
		SaveJSON       bool   `yaml:"save_json"`
		SaveCSV        bool   `yaml:"save_csv"`
	} `yaml:"output"`
}

func main() {
	configPath := flag.String("config", "../../configs/small-files.yaml", "Path to config file")
	flag.Parse()

	// Load config
	config, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Starting MiniToolStream benchmark: %s\n", config.Test.Name)
	fmt.Printf("Server: %s\n", config.Server.Address)
	fmt.Printf("Message size: %d bytes (%.2f KB)\n", config.Test.MessageSize, float64(config.Test.MessageSize)/1024)
	fmt.Printf("Total messages: %d\n", config.Test.TotalMessages)
	fmt.Printf("Producers: %d\n", config.Test.NumProducers)
	fmt.Printf("Target RPS: %d\n", config.Test.TargetRPS)
	fmt.Printf("Prometheus export: %v\n", config.Prometheus.Enabled)
	fmt.Println()

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create metrics collector
	metricsConfig := metrics.TestConfig{
		MessageSize:   config.Test.MessageSize,
		NumProducers:  config.Test.NumProducers,
		NumConsumers:  config.Test.NumConsumers,
		TotalMessages: config.Test.TotalMessages,
		TargetRPS:     config.Test.TargetRPS,
	}

	// Interface for collecting metrics (extended with Finalize)
	type FullMetricsCollector interface {
		MetricsCollector
		Finalize(system string, testName string) *metrics.BenchmarkResult
	}

	var collector FullMetricsCollector
	var baseCollector *metrics.Collector

	// Use Prometheus wrapper if enabled
	if config.Prometheus.Enabled {
		promWrapper := metrics.NewPrometheusCollectorWrapper(
			metricsConfig,
			config.Prometheus.PushgatewayURL,
			"minitoolstream",
			config.Test.Name,
			config.Prometheus.Instance,
		)
		promWrapper.StartPeriodicPush(ctx, config.Prometheus.PushInterval)
		collector = promWrapper  // Use wrapper directly, not inner Collector!
		baseCollector = promWrapper.Collector  // For DockerMonitor
		fmt.Printf("Prometheus push enabled: %s (interval: %s)\n", config.Prometheus.PushgatewayURL, config.Prometheus.PushInterval)
	} else {
		c := metrics.NewCollector(metricsConfig)
		collector = c
		baseCollector = c
	}

	// Handle interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt, stopping benchmark...")
		cancel()
	}()

	// Start resource monitoring
	if config.Monitoring.Enabled {
		monitor := metrics.NewDockerMonitor(config.Monitoring.Containers, config.Monitoring.Interval, baseCollector)
		go monitor.Start(ctx)
		defer monitor.Stop()
		fmt.Println("Resource monitoring started")
	}

	// Warmup period
	if config.Test.Warmup > 0 {
		fmt.Printf("Warmup for %s...\n", config.Test.Warmup)
		time.Sleep(config.Test.Warmup)
	}

	// Run benchmark
	fmt.Println("Starting benchmark...")
	startTime := time.Now()

	err = runBenchmark(ctx, config, collector)
	if err != nil {
		log.Fatalf("Benchmark failed: %v", err)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("\nBenchmark completed in %s\n", elapsed)

	// Finalize and save results
	result := collector.Finalize("minitoolstream", config.Test.Name)

	if config.Output.PrintSummary {
		result.PrintSummary()
	}

	if config.Output.SaveJSON {
		timestamp := time.Now().Format("20060102-150405")
		filename := fmt.Sprintf("%s-%s.json", config.Output.FilenamePrefix, timestamp)
		outputPath := filepath.Join(config.Output.ResultsDir, filename)

		// Create directory if not exists
		if err := os.MkdirAll(config.Output.ResultsDir, 0755); err != nil {
			log.Printf("Failed to create results directory: %v", err)
		} else {
			if err := result.SaveToFile(outputPath); err != nil {
				log.Printf("Failed to save results: %v", err)
			} else {
				fmt.Printf("Results saved to: %s\n", outputPath)
			}
		}
	}
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// MetricsCollector interface for recording benchmark metrics
type MetricsCollector interface {
	RecordMessage(size int64)
	RecordLatency(duration time.Duration)
	RecordError()
}

func runBenchmark(ctx context.Context, config *Config, collector MetricsCollector) error {
	// Calculate messages per producer
	messagesPerProducer := config.Test.TotalMessages / int64(config.Test.NumProducers)
	remainder := config.Test.TotalMessages % int64(config.Test.NumProducers)

	// Create wait group for producers
	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64

	// Rate limiter
	var rateLimiter <-chan time.Time
	if config.Test.TargetRPS > 0 {
		rpsPerProducer := config.Test.TargetRPS / config.Test.NumProducers
		if rpsPerProducer > 0 {
			interval := time.Second / time.Duration(rpsPerProducer)
			rateLimiter = time.Tick(interval)
		}
	}

	// Generate test data
	testData := make([]byte, config.Test.MessageSize)
	if _, err := rand.Read(testData); err != nil {
		return fmt.Errorf("failed to generate test data: %w", err)
	}

	// Start producers
	for i := 0; i < config.Test.NumProducers; i++ {
		wg.Add(1)

		messages := messagesPerProducer
		if i == 0 {
			messages += remainder
		}

		go func(producerID int, numMessages int64) {
			defer wg.Done()

			if err := runProducer(ctx, producerID, numMessages, testData, config, collector, rateLimiter, &successCount, &errorCount); err != nil {
				log.Printf("Producer %d error: %v", producerID, err)
			}
		}(i, messages)
	}

	// Wait for all producers to finish
	wg.Wait()

	fmt.Printf("\nTotal successful: %d, errors: %d\n", atomic.LoadInt64(&successCount), atomic.LoadInt64(&errorCount))

	return nil
}

func runProducer(
	ctx context.Context,
	producerID int,
	numMessages int64,
	testData []byte,
	config *Config,
	collector MetricsCollector,
	rateLimiter <-chan time.Time,
	successCount *int64,
	errorCount *int64,
) error {
	// Create publisher
	publisher, err := minitoolstream_connector.NewPublisherBuilder(config.Server.Address).Build()
	if err != nil {
		return fmt.Errorf("failed to create publisher: %w", err)
	}
	defer publisher.Close()

	fmt.Printf("Producer %d: starting (%d messages)\n", producerID, numMessages)

	for i := int64(0); i < numMessages; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Rate limiting
		if rateLimiter != nil {
			<-rateLimiter
		}

		// Measure latency
		start := time.Now()

		// Publish message using MessagePreparerFunc
		preparer := minitoolstream_connector.MessagePreparerFunc(func(ctx context.Context) (*minitoolstream_connector.PublishMessage, error) {
			return &minitoolstream_connector.PublishMessage{
				Subject: config.Subject,
				Data:    testData,
				Headers: map[string]string{
					"producer_id": fmt.Sprintf("%d", producerID),
					"message_id":  fmt.Sprintf("%d", i),
				},
			}, nil
		})

		err := publisher.Publish(ctx, preparer)
		latency := time.Since(start)

		if err != nil {
			collector.RecordError()
			atomic.AddInt64(errorCount, 1)
			log.Printf("Producer %d: publish error: %v", producerID, err)
			continue
		}

		// Record metrics
		collector.RecordLatency(latency)
		collector.RecordMessage(config.Test.MessageSize)
		atomic.AddInt64(successCount, 1)

		// Progress report
		if (i+1)%100 == 0 {
			fmt.Printf("Producer %d: %d/%d messages sent\n", producerID, i+1, numMessages)
		}
	}

	fmt.Printf("Producer %d: completed\n", producerID)
	return nil
}
