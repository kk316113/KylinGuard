package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"

	"kylin-guard-agent/agent-go/internal/execproxy"
	"kylin-guard-agent/agent-go/internal/logtrace"
)

const (
	defaultInventoryLimit = 100
	maxInventoryLimit     = 500
	rpmQueryFormat        = "%{NAME}\\t%{VERSION}\\t%{RELEASE}\\t%{ARCH}\\n"
)

type SystemdUnitInventoryResult struct {
	Units            []SystemdUnitInfo          `json:"units"`
	Count            int                        `json:"count"`
	Source           string                     `json:"source"`
	Message          string                     `json:"message"`
	Timestamp        time.Time                  `json:"timestamp"`
	ExecutionContext *logtrace.ExecutionContext `json:"-"`
}

type SystemdUnitInfo struct {
	Unit        string `json:"unit"`
	Load        string `json:"load"`
	Active      string `json:"active"`
	Sub         string `json:"sub"`
	Description string `json:"description"`
}

type BlockDeviceInventoryResult struct {
	Devices          []BlockDeviceInfo          `json:"devices"`
	Count            int                        `json:"count"`
	Source           string                     `json:"source"`
	Message          string                     `json:"message"`
	Timestamp        time.Time                  `json:"timestamp"`
	ExecutionContext *logtrace.ExecutionContext `json:"-"`
}

type BlockDeviceInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Size       string `json:"size,omitempty"`
	FSType     string `json:"fstype,omitempty"`
	Mountpoint string `json:"mountpoint,omitempty"`
	Rotational any    `json:"rota,omitempty"`
	Model      string `json:"model,omitempty"`
}

type MountInventoryResult struct {
	Mounts           []MountInfo                `json:"mounts"`
	Count            int                        `json:"count"`
	Source           string                     `json:"source"`
	Message          string                     `json:"message"`
	Timestamp        time.Time                  `json:"timestamp"`
	ExecutionContext *logtrace.ExecutionContext `json:"-"`
}

type MountInfo struct {
	Target  string `json:"target"`
	Source  string `json:"source"`
	FSType  string `json:"fstype"`
	Options string `json:"options,omitempty"`
}

type RPMPackageInventoryResult struct {
	Packages         []RPMPackageInfo           `json:"packages"`
	Count            int                        `json:"count"`
	Query            string                     `json:"query,omitempty"`
	Source           string                     `json:"source"`
	Message          string                     `json:"message"`
	Timestamp        time.Time                  `json:"timestamp"`
	ExecutionContext *logtrace.ExecutionContext `json:"-"`
}

type RPMPackageInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Release string `json:"release"`
	Arch    string `json:"arch"`
}

func SystemdUnitInventory(ctx context.Context, input map[string]any) (any, string, string, error) {
	limit := boundedInventoryLimit(input)
	now := time.Now().UTC()
	if runtime.GOOS != "linux" {
		ec := execproxy.NativeExecutionContext(execproxy.ProfileLowRead, "unsupported_platform", "systemd unit inventory is only supported on Linux")
		result := SystemdUnitInventoryResult{
			Units:            []SystemdUnitInfo{},
			Source:           "",
			Message:          "systemd unit inventory is only supported on Linux targets",
			Timestamp:        now,
			ExecutionContext: ecPtr(ec),
		}
		return result, "systemd_unit_inventory unsupported on non-Linux host", "low", nil
	}

	exec := execproxy.NewExecutor()
	execResult, execErr := exec.Execute(ctx, execproxy.CommandSpec{
		ToolName: "systemd_unit_inventory",
		Profile:  execproxy.ProfileLowRead,
		Command:  "systemctl",
		Args:     []string{"list-units", "--type=service", "--all", "--no-pager", "--plain", "--legend=false"},
		Reason:   "inventory systemd service units without modifying unit state",
	})
	if execErr != nil || execResult.Status == "error" {
		result := SystemdUnitInventoryResult{
			Units:            []SystemdUnitInfo{},
			Source:           "systemctl",
			Message:          fmt.Sprintf("systemctl list-units failed: %v", execErr),
			Timestamp:        now,
			ExecutionContext: ecPtr(execResult.Context),
		}
		return result, "systemd_unit_inventory failed", "review", execErr
	}
	units := parseSystemctlListUnits(execResult.Stdout, limit)
	result := SystemdUnitInventoryResult{
		Units:            units,
		Count:            len(units),
		Source:           "systemctl list-units",
		Message:          "systemd service unit inventory completed",
		Timestamp:        now,
		ExecutionContext: ecPtr(execResult.Context),
	}
	return result, fmt.Sprintf("systemd_unit_inventory found %d service units", len(units)), "low", nil
}

func BlockDeviceInventory(ctx context.Context, input map[string]any) (any, string, string, error) {
	limit := boundedInventoryLimit(input)
	now := time.Now().UTC()
	if runtime.GOOS != "linux" {
		ec := execproxy.NativeExecutionContext(execproxy.ProfileLowRead, "unsupported_platform", "block device inventory is only supported on Linux")
		result := BlockDeviceInventoryResult{
			Devices:          []BlockDeviceInfo{},
			Message:          "block device inventory is only supported on Linux targets",
			Timestamp:        now,
			ExecutionContext: ecPtr(ec),
		}
		return result, "block_device_inventory unsupported on non-Linux host", "low", nil
	}

	exec := execproxy.NewExecutor()
	execResult, execErr := exec.Execute(ctx, execproxy.CommandSpec{
		ToolName: "block_device_inventory",
		Profile:  execproxy.ProfileLowRead,
		Command:  "lsblk",
		Args:     []string{"--json", "--output", "NAME,TYPE,SIZE,FSTYPE,MOUNTPOINT,ROTA,MODEL", "--nodeps"},
		Reason:   "inventory block devices without mounting or modifying storage",
	})
	if execErr != nil || execResult.Status == "error" {
		result := BlockDeviceInventoryResult{
			Devices:          []BlockDeviceInfo{},
			Source:           "lsblk",
			Message:          fmt.Sprintf("lsblk inventory failed: %v", execErr),
			Timestamp:        now,
			ExecutionContext: ecPtr(execResult.Context),
		}
		return result, "block_device_inventory failed", "review", execErr
	}
	devices, parseErr := parseLSBLKJSON(execResult.Stdout, limit)
	if parseErr != nil {
		return BlockDeviceInventoryResult{Devices: []BlockDeviceInfo{}, Source: "lsblk", Message: parseErr.Error(), Timestamp: now, ExecutionContext: ecPtr(execResult.Context)}, "block_device_inventory parse failed", "review", parseErr
	}
	result := BlockDeviceInventoryResult{
		Devices:          devices,
		Count:            len(devices),
		Source:           "lsblk --json",
		Message:          "block device inventory completed",
		Timestamp:        now,
		ExecutionContext: ecPtr(execResult.Context),
	}
	return result, fmt.Sprintf("block_device_inventory found %d devices", len(devices)), "low", nil
}

func MountInventory(ctx context.Context, input map[string]any) (any, string, string, error) {
	limit := boundedInventoryLimit(input)
	now := time.Now().UTC()
	if runtime.GOOS != "linux" {
		ec := execproxy.NativeExecutionContext(execproxy.ProfileLowRead, "unsupported_platform", "mount inventory is only supported on Linux")
		result := MountInventoryResult{
			Mounts:           []MountInfo{},
			Message:          "mount inventory is only supported on Linux targets",
			Timestamp:        now,
			ExecutionContext: ecPtr(ec),
		}
		return result, "mount_inventory unsupported on non-Linux host", "low", nil
	}

	exec := execproxy.NewExecutor()
	execResult, execErr := exec.Execute(ctx, execproxy.CommandSpec{
		ToolName: "mount_inventory",
		Profile:  execproxy.ProfileLowRead,
		Command:  "findmnt",
		Args:     []string{"--json", "--output", "TARGET,SOURCE,FSTYPE,OPTIONS"},
		Reason:   "inventory mount topology without changing mounts",
	})
	if execErr != nil || execResult.Status == "error" {
		result := MountInventoryResult{
			Mounts:           []MountInfo{},
			Source:           "findmnt",
			Message:          fmt.Sprintf("findmnt inventory failed: %v", execErr),
			Timestamp:        now,
			ExecutionContext: ecPtr(execResult.Context),
		}
		return result, "mount_inventory failed", "review", execErr
	}
	mounts, parseErr := parseFindmntJSON(execResult.Stdout, limit)
	if parseErr != nil {
		return MountInventoryResult{Mounts: []MountInfo{}, Source: "findmnt", Message: parseErr.Error(), Timestamp: now, ExecutionContext: ecPtr(execResult.Context)}, "mount_inventory parse failed", "review", parseErr
	}
	result := MountInventoryResult{
		Mounts:           mounts,
		Count:            len(mounts),
		Source:           "findmnt --json",
		Message:          "mount inventory completed",
		Timestamp:        now,
		ExecutionContext: ecPtr(execResult.Context),
	}
	return result, fmt.Sprintf("mount_inventory found %d mount entries", len(mounts)), "low", nil
}

func RPMPackageInventory(ctx context.Context, input map[string]any) (any, string, string, error) {
	query := strings.ToLower(strings.TrimSpace(stringValue(input, "query", "")))
	if len(query) > 80 {
		return nil, "rpm_package_inventory rejected overlong query", "review", fmt.Errorf("query must be at most 80 characters")
	}
	if query != "" && !IsSafeRPMPackageName(query) {
		return nil, "rpm_package_inventory rejected unsafe query", "review", fmt.Errorf("query contains unsafe characters")
	}
	limit := boundedInventoryLimit(input)
	now := time.Now().UTC()
	if runtime.GOOS != "linux" {
		ec := execproxy.NativeExecutionContext(execproxy.ProfileLowRead, "unsupported_platform", "RPM inventory is only supported on Linux")
		result := RPMPackageInventoryResult{
			Packages:         []RPMPackageInfo{},
			Query:            query,
			Message:          "RPM package inventory is only supported on RPM-based Linux targets",
			Timestamp:        now,
			ExecutionContext: ecPtr(ec),
		}
		return result, "rpm_package_inventory unsupported on non-Linux host", "low", nil
	}

	exec := execproxy.NewExecutor()
	execResult, execErr := exec.Execute(ctx, execproxy.CommandSpec{
		ToolName:       "rpm_package_inventory",
		Profile:        execproxy.ProfileLowRead,
		Command:        "rpm",
		Args:           []string{"-qa", "--qf", rpmQueryFormat},
		Timeout:        10 * time.Second,
		MaxOutputBytes: 512 * 1024,
		Reason:         "inventory installed RPM packages without reading package file contents",
	})
	if execErr != nil || execResult.Status == "error" {
		result := RPMPackageInventoryResult{
			Packages:         []RPMPackageInfo{},
			Query:            query,
			Source:           "rpm",
			Message:          fmt.Sprintf("rpm package inventory failed: %v", execErr),
			Timestamp:        now,
			ExecutionContext: ecPtr(execResult.Context),
		}
		return result, "rpm_package_inventory failed", "review", execErr
	}
	packages := parseRPMPackageQuery(execResult.Stdout, query, limit)
	result := RPMPackageInventoryResult{
		Packages:         packages,
		Count:            len(packages),
		Query:            query,
		Source:           "rpm -qa",
		Message:          "RPM package inventory completed",
		Timestamp:        now,
		ExecutionContext: ecPtr(execResult.Context),
	}
	return result, fmt.Sprintf("rpm_package_inventory found %d packages", len(packages)), "low", nil
}

func boundedInventoryLimit(input map[string]any) int {
	limit := intValue(input, "limit", defaultInventoryLimit)
	if limit < 1 {
		return 1
	}
	if limit > maxInventoryLimit {
		return maxInventoryLimit
	}
	return limit
}

func parseSystemctlListUnits(output string, limit int) []SystemdUnitInfo {
	units := make([]SystemdUnitInfo, 0, limit)
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 || !strings.HasSuffix(fields[0], ".service") {
			continue
		}
		description := ""
		if len(fields) > 4 {
			description = strings.Join(fields[4:], " ")
		}
		units = append(units, SystemdUnitInfo{
			Unit:        fields[0],
			Load:        fields[1],
			Active:      fields[2],
			Sub:         fields[3],
			Description: description,
		})
		if len(units) >= limit {
			break
		}
	}
	return units
}

func parseLSBLKJSON(output string, limit int) ([]BlockDeviceInfo, error) {
	var parsed struct {
		BlockDevices []BlockDeviceInfo `json:"blockdevices"`
	}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		return nil, fmt.Errorf("cannot parse lsblk JSON: %w", err)
	}
	if len(parsed.BlockDevices) > limit {
		parsed.BlockDevices = parsed.BlockDevices[:limit]
	}
	return parsed.BlockDevices, nil
}

func parseFindmntJSON(output string, limit int) ([]MountInfo, error) {
	var parsed struct {
		Filesystems []MountInfo `json:"filesystems"`
	}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		return nil, fmt.Errorf("cannot parse findmnt JSON: %w", err)
	}
	if len(parsed.Filesystems) > limit {
		parsed.Filesystems = parsed.Filesystems[:limit]
	}
	return parsed.Filesystems, nil
}

func parseRPMPackageQuery(output string, query string, limit int) []RPMPackageInfo {
	packages := make([]RPMPackageInfo, 0, limit)
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 4 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		if query != "" && !strings.Contains(strings.ToLower(name), query) {
			continue
		}
		packages = append(packages, RPMPackageInfo{
			Name:    name,
			Version: strings.TrimSpace(parts[1]),
			Release: strings.TrimSpace(parts[2]),
			Arch:    strings.TrimSpace(parts[3]),
		})
		if len(packages) >= limit {
			break
		}
	}
	return packages
}
