package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type BenchmarkResult struct {
	System     string        `json:"system"`
	TestName   string        `json:"test_name"`
	Timestamp  time.Time     `json:"timestamp"`
	Throughput Throughput    `json:"throughput"`
	Latency    Latency       `json:"latency"`
	Resources  Resources     `json:"resources"`
	Errors     ErrorMetrics  `json:"errors"`
	Duration   time.Duration `json:"duration"`
	Config     TestConfig    `json:"config"`
}

type TestConfig struct {
	MessageSize   int64 `json:"message_size_bytes"`
	NumProducers  int   `json:"num_producers"`
	NumConsumers  int   `json:"num_consumers"`
	TotalMessages int64 `json:"total_messages"`
	TargetRPS     int   `json:"target_rps"`
}

type Throughput struct {
	TotalMessages int64   `json:"total_messages"`
	TotalBytes    int64   `json:"total_bytes"`
	MsgPerSec     float64 `json:"msg_per_sec"`
	MBPerSec      float64 `json:"mb_per_sec"`
}

type Latency struct {
	P50Ms time.Duration `json:"p50_ms"`
	P95Ms time.Duration `json:"p95_ms"`
	P99Ms time.Duration `json:"p99_ms"`
	MinMs time.Duration `json:"min_ms"`
	MaxMs time.Duration `json:"max_ms"`
	AvgMs time.Duration `json:"avg_ms"`
}

type Resources struct {
	CPUPercent  float64 `json:"cpu_percent"`
	MemoryMB    float64 `json:"memory_mb"`
	DiskWriteMB float64 `json:"disk_write_mb"`
	DiskReadMB  float64 `json:"disk_read_mb"`
	NetworkTxMB float64 `json:"network_tx_mb"`
	NetworkRxMB float64 `json:"network_rx_mb"`
}

type ErrorMetrics struct {
	ErrorCount int64   `json:"error_count"`
	ErrorRate  float64 `json:"error_rate"`
}

func main() {
	mtsDir := flag.String("mts", "../../results/minitoolstream", "MiniToolStream results directory")
	kafkaDir := flag.String("kafka", "../../results/kafka", "Kafka results directory")
	outputFile := flag.String("output", "../../results/comparison-report.md", "Output file")
	flag.Parse()

	fmt.Println("Comparative Analysis: MiniToolStream vs Kafka")
	fmt.Println("=" * 80)
	fmt.Println()

	// Load MiniToolStream results
	mtsResults, err := loadResults(*mtsDir)
	if err != nil {
		log.Fatalf("Failed to load MiniToolStream results: %v", err)
	}
	fmt.Printf("Loaded %d MiniToolStream results\n", len(mtsResults))

	// Load Kafka results
	kafkaResults, err := loadResults(*kafkaDir)
	if err != nil {
		log.Fatalf("Failed to load Kafka results: %v", err)
	}
	fmt.Printf("Loaded %d Kafka results\n", len(kafkaResults))

	if len(mtsResults) == 0 && len(kafkaResults) == 0 {
		log.Fatal("No results found to compare")
	}

	// Generate comparison report
	report := generateReport(mtsResults, kafkaResults)

	// Print to console
	fmt.Println("\n" + report)

	// Save to file
	if err := os.WriteFile(*outputFile, []byte(report), 0644); err != nil {
		log.Fatalf("Failed to write report: %v", err)
	}

	fmt.Printf("\nReport saved to: %s\n", *outputFile)
}

func loadResults(dir string) ([]BenchmarkResult, error) {
	var results []BenchmarkResult

	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			log.Printf("Warning: failed to read %s: %v", file, err)
			continue
		}

		var result BenchmarkResult
		if err := json.Unmarshal(data, &result); err != nil {
			log.Printf("Warning: failed to parse %s: %v", file, err)
			continue
		}

		results = append(results, result)
	}

	// Sort by timestamp
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.Before(results[j].Timestamp)
	})

	return results, nil
}

func generateReport(mtsResults, kafkaResults []BenchmarkResult) string {
	var sb strings.Builder

	sb.WriteString("# Comparative Benchmark Report: MiniToolStream vs Kafka\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	sb.WriteString("## Executive Summary\n\n")

	// Small files comparison
	sb.WriteString("### Small Files (10KB)\n\n")
	mtsSmall := findTestResults(mtsResults, "small")
	kafkaSmall := findTestResults(kafkaResults, "small")

	if len(mtsSmall) > 0 && len(kafkaSmall) > 0 {
		sb.WriteString(compareResults(mtsSmall[0], kafkaSmall[0]))
	} else if len(mtsSmall) > 0 {
		sb.WriteString("**MiniToolStream results available, Kafka results missing**\n\n")
		sb.WriteString(formatSingleResult(mtsSmall[0]))
	} else if len(kafkaSmall) > 0 {
		sb.WriteString("**Kafka results available, MiniToolStream results missing**\n\n")
		sb.WriteString(formatSingleResult(kafkaSmall[0]))
	} else {
		sb.WriteString("**No results available for small files**\n\n")
	}

	// Large files comparison
	sb.WriteString("### Large Files (1GB)\n\n")
	mtsLarge := findTestResults(mtsResults, "large")
	kafkaLarge := findTestResults(kafkaResults, "large")

	if len(mtsLarge) > 0 {
		sb.WriteString("**MiniToolStream Results:**\n\n")
		sb.WriteString(formatSingleResult(mtsLarge[0]))
	} else {
		sb.WriteString("**No MiniToolStream large file results available**\n\n")
	}

	if len(kafkaLarge) > 0 {
		sb.WriteString("**Kafka Results (chunked):**\n\n")
		sb.WriteString(formatSingleResult(kafkaLarge[0]))
	} else {
		sb.WriteString("**Note:** Kafka does not support 1GB messages natively. Requires chunking.\n\n")
	}

	// Detailed results
	sb.WriteString("\n## Detailed Results\n\n")

	sb.WriteString("### MiniToolStream\n\n")
	for _, result := range mtsResults {
		sb.WriteString(formatDetailedResult(result))
		sb.WriteString("\n---\n\n")
	}

	sb.WriteString("### Kafka\n\n")
	for _, result := range kafkaResults {
		sb.WriteString(formatDetailedResult(result))
		sb.WriteString("\n---\n\n")
	}

	return sb.String()
}

func findTestResults(results []BenchmarkResult, testType string) []BenchmarkResult {
	var filtered []BenchmarkResult
	for _, r := range results {
		if strings.Contains(strings.ToLower(r.TestName), testType) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func compareResults(mts, kafka BenchmarkResult) string {
	var sb strings.Builder

	sb.WriteString("| Metric | MiniToolStream | Kafka | Winner |\n")
	sb.WriteString("|--------|---------------|-------|--------|\n")

	// Throughput
	winner := "MTS"
	if kafka.Throughput.MsgPerSec > mts.Throughput.MsgPerSec {
		winner = "Kafka"
	}
	sb.WriteString(fmt.Sprintf("| **Throughput (msg/s)** | %.2f | %.2f | %s |\n",
		mts.Throughput.MsgPerSec, kafka.Throughput.MsgPerSec, winner))

	winner = "MTS"
	if kafka.Throughput.MBPerSec > mts.Throughput.MBPerSec {
		winner = "Kafka"
	}
	sb.WriteString(fmt.Sprintf("| **Throughput (MB/s)** | %.2f | %.2f | %s |\n",
		mts.Throughput.MBPerSec, kafka.Throughput.MBPerSec, winner))

	// Latency P95
	winner = "MTS"
	if kafka.Latency.P95Ms < mts.Latency.P95Ms {
		winner = "Kafka"
	}
	sb.WriteString(fmt.Sprintf("| **Latency P95** | %s | %s | %s |\n",
		mts.Latency.P95Ms, kafka.Latency.P95Ms, winner))

	// Latency P99
	winner = "MTS"
	if kafka.Latency.P99Ms < mts.Latency.P99Ms {
		winner = "Kafka"
	}
	sb.WriteString(fmt.Sprintf("| **Latency P99** | %s | %s | %s |\n",
		mts.Latency.P99Ms, kafka.Latency.P99Ms, winner))

	// CPU
	winner = "MTS"
	if kafka.Resources.CPUPercent < mts.Resources.CPUPercent {
		winner = "Kafka"
	}
	sb.WriteString(fmt.Sprintf("| **CPU Usage** | %.2f%% | %.2f%% | %s |\n",
		mts.Resources.CPUPercent, kafka.Resources.CPUPercent, winner))

	// Memory
	winner = "MTS"
	if kafka.Resources.MemoryMB < mts.Resources.MemoryMB {
		winner = "Kafka"
	}
	sb.WriteString(fmt.Sprintf("| **Memory Usage** | %.2f MB | %.2f MB | %s |\n",
		mts.Resources.MemoryMB, kafka.Resources.MemoryMB, winner))

	// Error Rate
	winner = "MTS"
	if kafka.Errors.ErrorRate < mts.Errors.ErrorRate {
		winner = "Kafka"
	}
	sb.WriteString(fmt.Sprintf("| **Error Rate** | %.4f%% | %.4f%% | %s |\n",
		mts.Errors.ErrorRate*100, kafka.Errors.ErrorRate*100, winner))

	sb.WriteString("\n")
	return sb.String()
}

func formatSingleResult(r BenchmarkResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("- **Throughput:** %.2f msg/s, %.2f MB/s\n", r.Throughput.MsgPerSec, r.Throughput.MBPerSec))
	sb.WriteString(fmt.Sprintf("- **Latency P95:** %s\n", r.Latency.P95Ms))
	sb.WriteString(fmt.Sprintf("- **Latency P99:** %s\n", r.Latency.P99Ms))
	sb.WriteString(fmt.Sprintf("- **CPU:** %.2f%%\n", r.Resources.CPUPercent))
	sb.WriteString(fmt.Sprintf("- **Memory:** %.2f MB\n", r.Resources.MemoryMB))
	sb.WriteString(fmt.Sprintf("- **Error Rate:** %.4f%%\n", r.Errors.ErrorRate*100))
	sb.WriteString("\n")

	return sb.String()
}

func formatDetailedResult(r BenchmarkResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("#### %s - %s\n\n", r.System, r.TestName))
	sb.WriteString(fmt.Sprintf("**Timestamp:** %s\n\n", r.Timestamp.Format("2006-01-02 15:04:05")))

	sb.WriteString("**Configuration:**\n")
	sb.WriteString(fmt.Sprintf("- Message Size: %d bytes (%.2f KB)\n", r.Config.MessageSize, float64(r.Config.MessageSize)/1024))
	sb.WriteString(fmt.Sprintf("- Total Messages: %d\n", r.Config.TotalMessages))
	sb.WriteString(fmt.Sprintf("- Producers: %d\n", r.Config.NumProducers))
	sb.WriteString(fmt.Sprintf("- Consumers: %d\n", r.Config.NumConsumers))
	sb.WriteString(fmt.Sprintf("- Duration: %s\n\n", r.Duration))

	sb.WriteString("**Throughput:**\n")
	sb.WriteString(fmt.Sprintf("- Messages/sec: %.2f\n", r.Throughput.MsgPerSec))
	sb.WriteString(fmt.Sprintf("- MB/sec: %.2f\n", r.Throughput.MBPerSec))
	sb.WriteString(fmt.Sprintf("- Total: %d messages, %.2f MB\n\n", r.Throughput.TotalMessages, float64(r.Throughput.TotalBytes)/(1024*1024)))

	sb.WriteString("**Latency:**\n")
	sb.WriteString(fmt.Sprintf("- Min: %s\n", r.Latency.MinMs))
	sb.WriteString(fmt.Sprintf("- Avg: %s\n", r.Latency.AvgMs))
	sb.WriteString(fmt.Sprintf("- P50: %s\n", r.Latency.P50Ms))
	sb.WriteString(fmt.Sprintf("- P95: %s\n", r.Latency.P95Ms))
	sb.WriteString(fmt.Sprintf("- P99: %s\n", r.Latency.P99Ms))
	sb.WriteString(fmt.Sprintf("- Max: %s\n\n", r.Latency.MaxMs))

	sb.WriteString("**Resources:**\n")
	sb.WriteString(fmt.Sprintf("- CPU: %.2f%%\n", r.Resources.CPUPercent))
	sb.WriteString(fmt.Sprintf("- Memory: %.2f MB\n", r.Resources.MemoryMB))
	sb.WriteString(fmt.Sprintf("- Disk Write: %.2f MB\n", r.Resources.DiskWriteMB))
	sb.WriteString(fmt.Sprintf("- Network TX: %.2f MB\n\n", r.Resources.NetworkTxMB))

	sb.WriteString("**Errors:**\n")
	sb.WriteString(fmt.Sprintf("- Count: %d\n", r.Errors.ErrorCount))
	sb.WriteString(fmt.Sprintf("- Rate: %.4f%%\n", r.Errors.ErrorRate*100))

	return sb.String()
}
