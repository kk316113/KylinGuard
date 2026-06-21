package tools

import (
	"context"
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"time"

	"kylin-guard-agent/agent-go/internal/execproxy"
	"kylin-guard-agent/agent-go/internal/logtrace"
)

const maxDriftPackages = 5

var rpmPackageNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+:-]{0,127}$`)

type ConfigurationDriftFinding struct {
	Package       string   `json:"package"`
	Path          string   `json:"path"`
	ChangedFields []string `json:"changed_fields"`
	Configuration bool     `json:"configuration_file"`
	Missing       bool     `json:"missing"`
	Unverifiable  bool     `json:"unverifiable"`
}

type ConfigurationDriftResult struct {
	BaselineSource    string                      `json:"baseline_source"`
	PackagesChecked   []string                    `json:"packages_checked"`
	PackagesMissing   []string                    `json:"packages_missing"`
	Findings          []ConfigurationDriftFinding `json:"findings"`
	DriftCount        int                         `json:"drift_count"`
	UnverifiableCount int                         `json:"unverifiable_count"`
	RiskLevel         string                      `json:"risk_level"`
	Message           string                      `json:"message"`
	Timestamp         time.Time                   `json:"timestamp"`
	ExecutionContext  *logtrace.ExecutionContext  `json:"-"`
}

func IsSafeRPMPackageName(name string) bool {
	return rpmPackageNamePattern.MatchString(strings.TrimSpace(name))
}

func ConfigurationDriftDetector(ctx context.Context, input map[string]any) (any, string, string, error) {
	if !validPackageInput(input["packages"]) {
		return nil, "configuration_drift_detector rejected malformed packages", "review", fmt.Errorf("packages must be a string or an array of strings")
	}
	packages := packageNamesFromInput(input)
	if len(packages) == 0 || len(packages) > maxDriftPackages {
		return nil, "configuration_drift_detector requires 1-5 packages", "review", fmt.Errorf("packages must contain 1-%d RPM package names", maxDriftPackages)
	}
	for _, name := range packages {
		if !IsSafeRPMPackageName(name) {
			return nil, "configuration_drift_detector rejected an unsafe package name", "review", fmt.Errorf("invalid RPM package name %q", name)
		}
	}

	result := ConfigurationDriftResult{
		BaselineSource:  "rpm_package_database",
		PackagesChecked: []string{}, PackagesMissing: []string{}, Findings: []ConfigurationDriftFinding{},
		RiskLevel: "low", Timestamp: time.Now().UTC(),
	}
	if runtime.GOOS != "linux" {
		ec := execproxy.NativeExecutionContext(execproxy.ProfileLowRead, "unsupported_platform", "RPM verification is only supported on Linux")
		result.Message = "configuration drift detection is only supported on RPM-based Linux"
		result.ExecutionContext = ecPtr(ec)
		return result, result.Message, "low", nil
	}

	executor := execproxy.NewExecutor()
	for _, name := range packages {
		execResult, execErr := executor.Execute(ctx, execproxy.CommandSpec{
			ToolName: "configuration_drift_detector", Profile: execproxy.ProfileSensitiveRead,
			Command: "rpm", Args: []string{"--verify", name}, Timeout: 10 * time.Second,
			MaxOutputBytes: 128 * 1024, AllowNonZeroExit: true, SensitiveOutput: true,
			Reason: "compare installed package files with the trusted RPM database baseline",
		})
		if result.ExecutionContext == nil {
			result.ExecutionContext = ecPtr(execResult.Context)
		}
		stderr := strings.ToLower(execResult.Stderr)
		if strings.Contains(stderr, "is not installed") || strings.Contains(stderr, "未安装") {
			result.PackagesMissing = append(result.PackagesMissing, name)
			continue
		}
		if execErr != nil && execResult.ExitCode != 1 {
			return result, "configuration drift verification failed", "review", fmt.Errorf("rpm verification failed for %s: %w", name, execErr)
		}
		result.PackagesChecked = append(result.PackagesChecked, name)
		result.Findings = append(result.Findings, parseRPMVerifyOutput(name, execResult.Stdout)...)
	}

	result.DriftCount = len(result.Findings)
	for _, finding := range result.Findings {
		if finding.Unverifiable {
			result.UnverifiableCount++
		}
	}
	if result.DriftCount > 0 {
		result.RiskLevel = "medium"
	}
	if result.DriftCount >= 10 {
		result.RiskLevel = "high"
	}
	result.Message = fmt.Sprintf("RPM baseline verification completed: %d drift findings across %d packages", result.DriftCount, len(result.PackagesChecked))
	return result, result.Message, result.RiskLevel, nil
}

func validPackageInput(value any) bool {
	switch typed := value.(type) {
	case []string:
		return true
	case []any:
		for _, item := range typed {
			if _, ok := item.(string); !ok {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func packageNamesFromInput(input map[string]any) []string {
	value := input["packages"]
	items := []string{}
	switch typed := value.(type) {
	case []string:
		items = typed
	case []any:
		for _, item := range typed {
			if name, ok := item.(string); ok {
				items = append(items, name)
			}
		}
	}
	seen := map[string]bool{}
	result := make([]string, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item)
		if name != "" && !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}
	return result
}

func parseRPMVerifyOutput(packageName, output string) []ConfigurationDriftFinding {
	findings := []ConfigurationDriftFinding{}
	fieldsByPosition := []string{"size", "mode", "digest", "device", "symlink", "owner", "group", "mtime", "capabilities"}
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		finding := ConfigurationDriftFinding{Package: packageName, ChangedFields: []string{}}
		pathIndex := 1
		if parts[0] == "missing" {
			finding.Missing = true
			finding.ChangedFields = append(finding.ChangedFields, "missing")
		} else {
			flags := parts[0]
			for i, field := range fieldsByPosition {
				if i < len(flags) && flags[i] != '.' {
					finding.ChangedFields = append(finding.ChangedFields, field)
					if flags[i] == '?' {
						finding.Unverifiable = true
					}
				}
			}
		}
		if pathIndex < len(parts) && len(parts[pathIndex]) == 1 && strings.Contains("cdglr", parts[pathIndex]) {
			finding.Configuration = parts[pathIndex] == "c"
			pathIndex++
		}
		if pathIndex >= len(parts) {
			continue
		}
		finding.Path = strings.Join(parts[pathIndex:], " ")
		findings = append(findings, finding)
	}
	return findings
}
