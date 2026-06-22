package tools

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"kylin-guard-agent/agent-go/internal/execproxy"
	"kylin-guard-agent/agent-go/internal/logtrace"
)

const diskSectorBytes = 512

var wholeDiskNamePattern = regexp.MustCompile(`^(sd[a-z]+|vd[a-z]+|xvd[a-z]+|hd[a-z]+|nvme[0-9]+n[0-9]+|mmcblk[0-9]+)$`)

type DiskIOCheckerResult struct {
	Devices          []DiskIOMetric             `json:"devices"`
	SampleMS         int                        `json:"sample_ms"`
	RiskLevel        string                     `json:"risk_level"`
	Source           string                     `json:"source"`
	Message          string                     `json:"message"`
	Timestamp        time.Time                  `json:"timestamp"`
	ExecutionContext *logtrace.ExecutionContext `json:"-"`
}

type DiskIOMetric struct {
	Device           string  `json:"device"`
	ReadsPerSecond   float64 `json:"reads_per_second"`
	WritesPerSecond  float64 `json:"writes_per_second"`
	ReadBytesPerSec  float64 `json:"read_bytes_per_second"`
	WriteBytesPerSec float64 `json:"write_bytes_per_second"`
	UtilizationPct   float64 `json:"utilization_percent"`
	IOInProgress     uint64  `json:"io_in_progress"`
	WeightedIOMillis uint64  `json:"weighted_io_millis_delta"`
}

type diskStatSnapshot struct {
	Name             string
	ReadsCompleted   uint64
	SectorsRead      uint64
	WritesCompleted  uint64
	SectorsWritten   uint64
	IOInProgress     uint64
	IOMillis         uint64
	WeightedIOMillis uint64
}

func DiskIOChecker(ctx context.Context, input map[string]any) (any, string, string, error) {
	sampleMS := intValue(input, "sample_ms", 250)
	if sampleMS < 100 {
		sampleMS = 100
	}
	if sampleMS > 2000 {
		sampleMS = 2000
	}
	now := time.Now().UTC()
	ec := execproxy.NativeExecutionContext(execproxy.ProfileLowRead, "procfs_read", "native Go procfs sampler: /proc/diskstats")

	if runtime.GOOS != "linux" {
		result := DiskIOCheckerResult{
			Devices:          []DiskIOMetric{},
			SampleMS:         sampleMS,
			RiskLevel:        "unknown",
			Source:           "",
			Message:          "disk_io_checker is only supported on Linux",
			Timestamp:        now,
			ExecutionContext: ecPtr(ec),
		}
		return result, "disk_io_checker unsupported on non-Linux host", "low", nil
	}

	first, err := readDiskStatsFile()
	if err != nil {
		return diskIOErrorResult(sampleMS, now, ec, err)
	}
	timer := time.NewTimer(time.Duration(sampleMS) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return diskIOErrorResult(sampleMS, now, ec, ctx.Err())
	case <-timer.C:
	}
	second, err := readDiskStatsFile()
	if err != nil {
		return diskIOErrorResult(sampleMS, now, ec, err)
	}

	metrics := calculateDiskIOMetrics(first, second, time.Duration(sampleMS)*time.Millisecond)
	risk := diskIORisk(metrics)
	maxUtil := 0.0
	for _, metric := range metrics {
		if metric.UtilizationPct > maxUtil {
			maxUtil = metric.UtilizationPct
		}
	}
	result := DiskIOCheckerResult{
		Devices:          metrics,
		SampleMS:         sampleMS,
		RiskLevel:        risk,
		Source:           "procfs:/proc/diskstats",
		Message:          "disk I/O sampling completed",
		Timestamp:        now,
		ExecutionContext: ecPtr(ec),
	}
	summary := fmt.Sprintf("disk_io_checker sampled %d devices; max_utilization=%.1f%% risk=%s", len(metrics), maxUtil, risk)
	return result, summary, risk, nil
}

func diskIOErrorResult(sampleMS int, now time.Time, ec execproxy.ExecutionContext, err error) (any, string, string, error) {
	result := DiskIOCheckerResult{
		Devices:          []DiskIOMetric{},
		SampleMS:         sampleMS,
		RiskLevel:        "unknown",
		Source:           "procfs:/proc/diskstats",
		Message:          fmt.Sprintf("disk I/O sampling failed: %v", err),
		Timestamp:        now,
		ExecutionContext: ecPtr(ec),
	}
	return result, "disk_io_checker failed", "review", err
}

func readDiskStatsFile() (map[string]diskStatSnapshot, error) {
	data, err := os.ReadFile("/proc/diskstats")
	if err != nil {
		return nil, fmt.Errorf("cannot read /proc/diskstats: %w", err)
	}
	return parseDiskStats(string(data)), nil
}

func parseDiskStats(data string) map[string]diskStatSnapshot {
	result := make(map[string]diskStatSnapshot)
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 14 || !wholeDiskNamePattern.MatchString(fields[2]) {
			continue
		}
		values := make([]uint64, 11)
		valid := true
		for i := 0; i < 11; i++ {
			parsed, err := strconv.ParseUint(fields[i+3], 10, 64)
			if err != nil {
				valid = false
				break
			}
			values[i] = parsed
		}
		if !valid {
			continue
		}
		result[fields[2]] = diskStatSnapshot{
			Name:             fields[2],
			ReadsCompleted:   values[0],
			SectorsRead:      values[2],
			WritesCompleted:  values[4],
			SectorsWritten:   values[6],
			IOInProgress:     values[8],
			IOMillis:         values[9],
			WeightedIOMillis: values[10],
		}
	}
	return result
}

func calculateDiskIOMetrics(first, second map[string]diskStatSnapshot, interval time.Duration) []DiskIOMetric {
	seconds := interval.Seconds()
	if seconds <= 0 {
		return []DiskIOMetric{}
	}
	metrics := make([]DiskIOMetric, 0, len(second))
	for name, after := range second {
		before, ok := first[name]
		if !ok {
			continue
		}
		metric := DiskIOMetric{
			Device:           name,
			ReadsPerSecond:   float64(counterDelta(after.ReadsCompleted, before.ReadsCompleted)) / seconds,
			WritesPerSecond:  float64(counterDelta(after.WritesCompleted, before.WritesCompleted)) / seconds,
			ReadBytesPerSec:  float64(counterDelta(after.SectorsRead, before.SectorsRead)*diskSectorBytes) / seconds,
			WriteBytesPerSec: float64(counterDelta(after.SectorsWritten, before.SectorsWritten)*diskSectorBytes) / seconds,
			UtilizationPct:   float64(counterDelta(after.IOMillis, before.IOMillis)) / float64(interval.Milliseconds()) * 100,
			IOInProgress:     after.IOInProgress,
			WeightedIOMillis: counterDelta(after.WeightedIOMillis, before.WeightedIOMillis),
		}
		if metric.UtilizationPct > 100 {
			metric.UtilizationPct = 100
		}
		metrics = append(metrics, metric)
	}
	sortDiskIOMetrics(metrics)
	return metrics
}

func sortDiskIOMetrics(metrics []DiskIOMetric) {
	for i := 1; i < len(metrics); i++ {
		for j := i; j > 0 && metrics[j].Device < metrics[j-1].Device; j-- {
			metrics[j], metrics[j-1] = metrics[j-1], metrics[j]
		}
	}
}

func counterDelta(after, before uint64) uint64 {
	if after < before {
		return 0
	}
	return after - before
}

func diskIORisk(metrics []DiskIOMetric) string {
	risk := "low"
	for _, metric := range metrics {
		if metric.UtilizationPct >= 95 || metric.IOInProgress >= 64 {
			return "high"
		}
		if metric.UtilizationPct >= 80 || metric.IOInProgress >= 16 {
			risk = "medium"
		}
	}
	return risk
}
