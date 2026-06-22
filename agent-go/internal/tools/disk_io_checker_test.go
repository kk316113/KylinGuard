package tools

import (
	"math"
	"testing"
	"time"
)

func TestParseDiskStatsFiltersPartitionsAndVirtualDevices(t *testing.T) {
	data := "   8       0 sda 100 0 200 10 300 0 400 20 2 30 40\n" +
		"   8       1 sda1 50 0 100 5 100 0 200 10 0 15 20\n" +
		"   7       0 loop0 10 0 20 1 0 0 0 0 0 1 1\n" +
		" 259       0 nvme0n1 200 0 500 20 400 0 600 30 1 50 70\n"
	stats := parseDiskStats(data)
	if len(stats) != 2 {
		t.Fatalf("expected 2 whole disks, got %#v", stats)
	}
	if stats["sda"].SectorsWritten != 400 || stats["nvme0n1"].ReadsCompleted != 200 {
		t.Fatalf("unexpected parsed stats: %#v", stats)
	}
}

func TestCalculateDiskIOMetrics(t *testing.T) {
	first := map[string]diskStatSnapshot{
		"sda": {Name: "sda", ReadsCompleted: 100, SectorsRead: 200, WritesCompleted: 300, SectorsWritten: 400, IOMillis: 1000, WeightedIOMillis: 1200},
	}
	second := map[string]diskStatSnapshot{
		"sda": {Name: "sda", ReadsCompleted: 110, SectorsRead: 240, WritesCompleted: 320, SectorsWritten: 480, IOInProgress: 3, IOMillis: 1500, WeightedIOMillis: 1800},
	}
	metrics := calculateDiskIOMetrics(first, second, time.Second)
	if len(metrics) != 1 {
		t.Fatalf("expected one metric, got %#v", metrics)
	}
	metric := metrics[0]
	if metric.ReadsPerSecond != 10 || metric.WritesPerSecond != 20 {
		t.Fatalf("unexpected IOPS: %#v", metric)
	}
	if math.Abs(metric.ReadBytesPerSec-20480) > 0.1 || math.Abs(metric.WriteBytesPerSec-40960) > 0.1 {
		t.Fatalf("unexpected throughput: %#v", metric)
	}
	if metric.UtilizationPct != 50 || metric.IOInProgress != 3 || metric.WeightedIOMillis != 600 {
		t.Fatalf("unexpected pressure metrics: %#v", metric)
	}
}

func TestDiskIORiskThresholds(t *testing.T) {
	if got := diskIORisk([]DiskIOMetric{{UtilizationPct: 79, IOInProgress: 2}}); got != "low" {
		t.Fatalf("expected low, got %s", got)
	}
	if got := diskIORisk([]DiskIOMetric{{UtilizationPct: 80}}); got != "medium" {
		t.Fatalf("expected medium, got %s", got)
	}
	if got := diskIORisk([]DiskIOMetric{{IOInProgress: 64}}); got != "high" {
		t.Fatalf("expected high, got %s", got)
	}
}
