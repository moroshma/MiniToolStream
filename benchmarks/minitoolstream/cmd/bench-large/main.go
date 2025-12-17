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

	"github.com/moroshma/minitoolstream_connector"
	"../../pkg/metrics"
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
	Output struct {
		ResultsDir     string `yaml:"results_dir"`
		FilenamePrefix string `yaml:"filename_prefix"`
		PrintSummary   bool   `yaml:"print_summary"`
		SaveJSON       bool   `yaml:"save_json"`
		SaveCSV        bool   `yaml:"save_csv"`
	} `yaml:"output"`
}

func main() {
	configPath := flag.String("config", "../../configs/large-files.yaml", "Path to config file")
	flag.Parse()

	// Load config
	config, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Starting MiniToolStream benchmark: %s\n", config.Test.Name)
	fmt.Printf("Server: %s\n", config.Server.Address)
	fmt.Printf("Message size: %d bytes (%.2f GB)\n", config.Test.MessageSize, float64(config.Test.MessageSize)/(1024*1024*1024))
	fmt.Printf("Total messages: %d\n", config.Test.TotalMessages)
	fmt.Printf("Producers: %d\n", config.Test.NumProducers)
	fmt.Println()

	// Create metrics collector
	metricsConfig := metrics.TestConfig{
		MessageSize:   config.Test.MessageSize,
		NumProducers:  config.Test.NumProducers,
		NumConsumers:  config.Test.NumConsumers,
		TotalMessages: config.Test.TotalMessages,
		TargetRPS:     config.Test.TargetRPS,
	}
	collector := metrics.NewCollector(metricsConfig)

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
		monitor := metrics.NewDockerMonitor(config.Monitoring.Containers, config.Monitoring.Interval, collector)
		go monitor.Start(ctx)
		defer monitor.Stop()
		fmt.Println("Resource monitoring started")
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

func runBenchmark(ctx context.Context, config *Config, collector *metrics.Collector) error {
	// Calculate messages per producer
	messagesPerProducer := config.Test.TotalMessages / int64(config.Test.NumProducers)
	remainder := config.Test.TotalMessages % int64(config.Test.NumProducers)

	// Create wait group for producers
	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64

	// For large files, we'll generate data on-the-fly to save memory
	// Instead of pre-allocating 1GB

	// Start producers
	for i := 0; i < config.Test.NumProducers; i++ {
		wg.Add(1)

		messages := messagesPerProducer
		if i == 0 {
			messages += remainder
		}

		go func(producerID int, numMessages int64) {
			defer wg.Done()

			if err := runProducer(ctx, producerID, numMessages, config, collector, &successCount, &errorCount); err != nil {
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
	config *Config,
	collector *metrics.Collector,
	successCount *int64,
	errorCount *int64,
) error {
	// Create publisher
	publisher, err := minitoolstream_connector.NewPublisherBuilder(config.Server.Address).Build()
	if err != nil {
		return fmt.Errorf("failed to create publisher: %w", err)
	}
	defer publisher.Close()

	fmt.Printf("Producer %d: starting (%d messages of %.2f GB each)\n",
		producerID, numMessages, float64(config.Test.MessageSize)/(1024*1024*1024))

	for i := int64(0); i < numMessages; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		fmt.Printf("Producer %d: generating %.2f GB of random data for message %d/%d...\n",
			producerID, float64(config.Test.MessageSize)/(1024*1024*1024), i+1, numMessages)

		// Generate large random data
		// For 1GB, we generate in chunks to avoid memory issues
		chunkSize := int64(100 * 1024 * 1024) // 100MB chunks
		numChunks := config.Test.MessageSize / chunkSize
		lastChunkSize := config.Test.MessageSize % chunkSize

		testData := make([]byte, 0, config.Test.MessageSize)

		for c := int64(0); c < numChunks; c++ {
			chunk := make([]byte, chunkSize)
			if _, err := rand.Read(chunk); err != nil {
				return fmt.Errorf("failed to generate chunk: %w", err)
			}
			testData = append(testData, chunk...)

			if (c+1)%5 == 0 {
				fmt.Printf("  Generated %d/%d chunks (%.1f%%)\n", c+1, numChunks, float64(c+1)/float64(numChunks)*100)
			}
		}

		if lastChunkSize > 0 {
			chunk := make([]byte, lastChunkSize)
			if _, err := rand.Read(chunk); err != nil {
				return fmt.Errorf("failed to generate last chunk: %w", err)
			}
			testData = append(testData, chunk...)
		}

		fmt.Printf("Producer %d: data generated, publishing message %d/%d...\n", producerID, i+1, numMessages)

		// Measure latency
		start := time.Now()

		// Publish message
		msg := &minitoolstream_connector.Message{
			Subject: config.Subject,
			Data:    testData,
			Headers: map[string]string{
				"producer_id": fmt.Sprintf("%d", producerID),
				"message_id":  fmt.Sprintf("%d", i),
				"size_gb":     fmt.Sprintf("%.2f", float64(len(testData))/(1024*1024*1024)),
			},
		}

		_, err := publisher.Publish(ctx, msg)
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

		throughputMBps := float64(config.Test.MessageSize) / (1024 * 1024) / latency.Seconds()
		fmt.Printf("Producer %d: message %d/%d published in %s (%.2f MB/s)\n",
			producerID, i+1, numMessages, latency, throughputMBps)
	}

	fmt.Printf("Producer %d: completed\n", producerID)
	return nil
}
