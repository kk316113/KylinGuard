package tools

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// ResourceUsageCheckerResult holds the output of resource usage inspection.
type ResourceUsageCheckerResult struct {
	LoadAvg   *LoadAvgInfo `json:"loadavg"`
	Memory    *MemInfo     `json:"memory"`
	RiskLevel string       `json:"risk_level"`
	Source    string       `json:"source"`
	Message   string       `json:"message"`
	Timestamp time.Time    `json:"timestamp"`
}

// LoadAvgInfo holds system load averages.
type LoadAvgInfo struct {
	OneMin     float64 `json:"one_min"`
	FiveMin    float64 `json:"five_min"`
	FifteenMin float64 `json:"fifteen_min"`
}

// MemInfo holds memory usage information.
type MemInfo struct {
	MemTotalKB        int64   `json:"mem_total_kb"`
	MemAvailableKB    int64   `json:"mem_available_kb"`
	MemAvailableRatio float64 `json:"mem_available_ratio"`
}

// ResourceUsageChecker reads system load and memory usage from procfs.
// It is a read-only diagnostic tool.
func ResourceUsageChecker(ctx context.Context, input map[string]any) (any, string, string, error) {
	_ = input // No input parameters needed.
	now := time.Now().UTC()

	if runtime.GOOS != "linux" {
		result := ResourceUsageCheckerResult{
			RiskLevel: "unknown",
			Source:    "",
			Message:   "resource_usage_checker is only supported on Linux",
			Timestamp: now,
		}
		return result, "resource_usage_checker unsupported on non-Linux host", "low", nil
	}

	var warnings []string

	loadAvg, loadWarn := readLoadAvg()
	if loadWarn != "" {
		warnings = append(warnings, loadWarn)
	}

	mem, memWarn := readMemInfo()
	if memWarn != "" {
		warnings = append(warnings, memWarn)
	}

	riskLevel := "low"
	if mem != nil {
		if mem.MemAvailableRatio < 0.1 {
			riskLevel = "high"
		} else if mem.MemAvailableRatio < 0.2 {
			riskLevel = "medium"
		}
	}

	message := "resource usage check completed"
	if len(warnings) > 0 {
		message = fmt.Sprintf("resource usage check completed with warnings: %s", strings.Join(warnings, "; "))
	}

	summary := fmt.Sprintf("resource_usage_checker: loadavg_1m=%.2f, mem_available_ratio=%.2f, risk=%s",
		loadAvgField(loadAvg, func(l *LoadAvgInfo) float64 { return l.OneMin }),
		memField(mem, func(m *MemInfo) float64 { return m.MemAvailableRatio }),
		riskLevel,
	)

	result := ResourceUsageCheckerResult{
		LoadAvg:   loadAvg,
		Memory:    mem,
		RiskLevel: riskLevel,
		Source:    "procfs",
		Message:   message,
		Timestamp: now,
	}
	return result, summary, riskLevel, nil
}

func readLoadAvg() (*LoadAvgInfo, string) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return nil, fmt.Sprintf("cannot read /proc/loadavg: %v", err)
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return nil, "unexpected /proc/loadavg format"
	}

	one, err1 := strconv.ParseFloat(fields[0], 64)
	five, err2 := strconv.ParseFloat(fields[1], 64)
	fifteen, err3 := strconv.ParseFloat(fields[2], 64)

	if err1 != nil || err2 != nil || err3 != nil {
		return nil, "cannot parse /proc/loadavg values"
	}

	return &LoadAvgInfo{
		OneMin:     one,
		FiveMin:    five,
		FifteenMin: fifteen,
	}, ""
}

func readMemInfo() (*MemInfo, string) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return nil, fmt.Sprintf("cannot read /proc/meminfo: %v", err)
	}

	var memTotalKB, memAvailableKB int64
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		val, parseErr := strconv.ParseInt(fields[1], 10, 64)
		if parseErr != nil {
			continue
		}
		switch key {
		case "MemTotal":
			memTotalKB = val
		case "MemAvailable":
			memAvailableKB = val
		}
	}

	if memTotalKB == 0 {
		return nil, "cannot parse MemTotal from /proc/meminfo"
	}

	ratio := float64(memAvailableKB) / float64(memTotalKB)
	return &MemInfo{
		MemTotalKB:        memTotalKB,
		MemAvailableKB:    memAvailableKB,
		MemAvailableRatio: ratio,
	}, ""
}

func loadAvgField(load *LoadAvgInfo, fn func(*LoadAvgInfo) float64) float64 {
	if load == nil {
		return 0
	}
	return fn(load)
}

func memField(mem *MemInfo, fn func(*MemInfo) float64) float64 {
	if mem == nil {
		return 0
	}
	return fn(mem)
}
