package metrics

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// DockerMonitor monitors Docker container resources
type DockerMonitor struct {
	containerNames []string
	interval       time.Duration
	stopChan       chan struct{}
	collector      *Collector
}

// NewDockerMonitor creates a new Docker resource monitor
func NewDockerMonitor(containerNames []string, interval time.Duration, collector *Collector) *DockerMonitor {
	return &DockerMonitor{
		containerNames: containerNames,
		interval:       interval,
		stopChan:       make(chan struct{}),
		collector:      collector,
	}
}

// Start begins monitoring Docker containers
func (d *DockerMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopChan:
			return
		case <-ticker.C:
			resources, err := d.collectDockerStats()
			if err != nil {
				fmt.Printf("Warning: failed to collect docker stats: %v\n", err)
				continue
			}
			d.collector.RecordResources(resources)
		}
	}
}

// Stop stops the monitor
func (d *DockerMonitor) Stop() {
	close(d.stopChan)
}

// collectDockerStats collects stats from Docker containers
func (d *DockerMonitor) collectDockerStats() (Resources, error) {
	var resources Resources

	for _, containerName := range d.containerNames {
		// Get container stats using docker stats --no-stream
		cmd := exec.Command("docker", "stats", containerName, "--no-stream", "--format", "{{.CPUPerc}},{{.MemUsage}},{{.NetIO}},{{.BlockIO}}")
		output, err := cmd.Output()
		if err != nil {
			return resources, fmt.Errorf("failed to get stats for %s: %w", containerName, err)
		}

		// Parse output
		stats := strings.TrimSpace(string(output))
		parts := strings.Split(stats, ",")
		if len(parts) < 4 {
			continue
		}

		// Parse CPU
		cpuStr := strings.TrimSuffix(parts[0], "%")
		if cpu, err := strconv.ParseFloat(cpuStr, 64); err == nil {
			resources.CPUPercent += cpu
		}

		// Parse Memory (format: "123.4MiB / 1.5GiB")
		memParts := strings.Split(parts[1], " / ")
		if len(memParts) > 0 {
			memUsage := parseSizeToMB(strings.TrimSpace(memParts[0]))
			resources.MemoryMB += memUsage
		}

		// Parse Network I/O (format: "1.23MB / 4.56MB")
		netParts := strings.Split(parts[2], " / ")
		if len(netParts) == 2 {
			resources.NetworkRxMB += parseSizeToMB(strings.TrimSpace(netParts[0]))
			resources.NetworkTxMB += parseSizeToMB(strings.TrimSpace(netParts[1]))
		}

		// Parse Block I/O (format: "1.23MB / 4.56MB")
		blockParts := strings.Split(parts[3], " / ")
		if len(blockParts) == 2 {
			resources.DiskReadMB += parseSizeToMB(strings.TrimSpace(blockParts[0]))
			resources.DiskWriteMB += parseSizeToMB(strings.TrimSpace(blockParts[1]))
		}
	}

	return resources, nil
}

// parseSizeToMB converts size string (e.g., "123.4MiB", "1.5GiB", "500kB") to MB
func parseSizeToMB(sizeStr string) float64 {
	sizeStr = strings.TrimSpace(sizeStr)
	if sizeStr == "" || sizeStr == "0B" {
		return 0
	}

	// Extract number and unit
	var value float64
	var unit string

	// Try different formats
	if _, err := fmt.Sscanf(sizeStr, "%f%s", &value, &unit); err != nil {
		return 0
	}

	unit = strings.ToUpper(unit)

	// Convert to MB
	switch unit {
	case "B":
		return value / (1024 * 1024)
	case "KB", "KIB":
		return value / 1024
	case "MB", "MIB":
		return value
	case "GB", "GIB":
		return value * 1024
	case "TB", "TIB":
		return value * 1024 * 1024
	default:
		return 0
	}
}

// GetContainerStats gets one-time stats for containers (useful for final snapshot)
func GetContainerStats(containerNames []string) (Resources, error) {
	monitor := &DockerMonitor{containerNames: containerNames}
	return monitor.collectDockerStats()
}

// MonitorStats continuously monitors Docker stats and calls callback
func MonitorStats(ctx context.Context, containerNames []string, interval time.Duration, callback func(Resources)) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			monitor := &DockerMonitor{containerNames: containerNames}
			stats, err := monitor.collectDockerStats()
			if err != nil {
				fmt.Printf("Warning: failed to collect stats: %v\n", err)
				continue
			}
			callback(stats)
		}
	}
}

// ParseDockerStatsStream parses docker stats streaming output
func ParseDockerStatsStream(ctx context.Context, containerName string, callback func(Resources)) error {
	cmd := exec.CommandContext(ctx, "docker", "stats", containerName, "--format", "{{.CPUPerc}},{{.MemUsage}},{{.NetIO}},{{.BlockIO}}")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start docker stats: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ",")
		if len(parts) < 4 {
			continue
		}

		var resources Resources

		// Parse CPU
		cpuStr := strings.TrimSuffix(parts[0], "%")
		if cpu, err := strconv.ParseFloat(cpuStr, 64); err == nil {
			resources.CPUPercent = cpu
		}

		// Parse Memory
		memParts := strings.Split(parts[1], " / ")
		if len(memParts) > 0 {
			resources.MemoryMB = parseSizeToMB(strings.TrimSpace(memParts[0]))
		}

		// Parse Network
		netParts := strings.Split(parts[2], " / ")
		if len(netParts) == 2 {
			resources.NetworkRxMB = parseSizeToMB(strings.TrimSpace(netParts[0]))
			resources.NetworkTxMB = parseSizeToMB(strings.TrimSpace(netParts[1]))
		}

		// Parse Disk
		blockParts := strings.Split(parts[3], " / ")
		if len(blockParts) == 2 {
			resources.DiskReadMB = parseSizeToMB(strings.TrimSpace(blockParts[0]))
			resources.DiskWriteMB = parseSizeToMB(strings.TrimSpace(blockParts[1]))
		}

		callback(resources)
	}

	return cmd.Wait()
}
