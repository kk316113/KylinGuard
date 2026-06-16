package tools

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// NetworkConnectionInspectorResult holds the output of network connection inspection.
type NetworkConnectionInspectorResult struct {
	Connections []NetworkConnectionInfo `json:"connections"`
	Count       int                     `json:"count"`
	Source      string                  `json:"source"`
	Message     string                  `json:"message"`
	Timestamp   time.Time               `json:"timestamp"`
}

// NetworkConnectionInfo describes a single network connection.
type NetworkConnectionInfo struct {
	Protocol     string `json:"protocol"`
	State        string `json:"state"`
	LocalAddress string `json:"local_address"`
	PeerAddress  string `json:"peer_address"`
	Process      string `json:"process,omitempty"`
}

// allowedNetworkStates is the whitelist of valid connection states.
var allowedNetworkStates = map[string]bool{
	"LISTEN":      true,
	"ESTABLISHED": true,
	"TIME-WAIT":   true,
	"TIME_WAIT":   true,
	"CLOSE-WAIT":  true,
	"CLOSE_WAIT":  true,
	"ALL":         true,
}

// NetworkConnectionInspector inspects network listening ports and connection states.
// It is a read-only diagnostic tool and does not modify network configuration.
func NetworkConnectionInspector(ctx context.Context, input map[string]any) (any, string, string, error) {
	state := strings.ToUpper(strings.TrimSpace(stringValue(input, "state", "ALL")))
	limit := intValue(input, "limit", 100)
	if limit < 1 {
		limit = 1
	}
	if limit > 500 {
		limit = 500
	}

	now := time.Now().UTC()

	// Validate state against whitelist.
	if !allowedNetworkStates[state] {
		result := NetworkConnectionInspectorResult{
			Connections: []NetworkConnectionInfo{},
			Count:       0,
			Source:      "",
			Message:     fmt.Sprintf("invalid state %q: allowed values are LISTEN, ESTABLISHED, TIME-WAIT, CLOSE-WAIT, ALL", state),
			Timestamp:   now,
		}
		return result, "network_connection_inspector: invalid state rejected", "low", fmt.Errorf("invalid network state %q", state)
	}

	// Normalize to canonical form.
	state = normalizeNetworkState(state)

	switch runtime.GOOS {
	case "linux":
		return networkConnectionInspectorLinux(ctx, state, limit, now)
	case "windows":
		return networkConnectionInspectorWindows(ctx, state, limit, now)
	default:
		result := NetworkConnectionInspectorResult{
			Connections: []NetworkConnectionInfo{},
			Count:       0,
			Source:      "",
			Message:     fmt.Sprintf("network_connection_inspector is not supported on %s", runtime.GOOS),
			Timestamp:   now,
		}
		return result, "network_connection_inspector unsupported on this OS", "low", nil
	}
}

func normalizeNetworkState(state string) string {
	switch state {
	case "TIME_WAIT":
		return "TIME-WAIT"
	case "CLOSE_WAIT":
		return "CLOSE-WAIT"
	default:
		return state
	}
}

func networkConnectionInspectorLinux(ctx context.Context, state string, limit int, now time.Time) (any, string, string, error) {
	_, ssErr := exec.LookPath("ss")
	if ssErr != nil {
		// Fallback to netstat if available.
		_, nsErr := exec.LookPath("netstat")
		if nsErr != nil {
			result := NetworkConnectionInspectorResult{
				Connections: []NetworkConnectionInfo{},
				Count:       0,
				Source:      "",
				Message:     "ss and netstat commands not found",
				Timestamp:   now,
			}
			return result, "network_connection_inspector: ss and netstat not found", "low", fmt.Errorf("ss lookup: %w; netstat lookup: %w", ssErr, nsErr)
		}
		return networkConnectionInspectorNetstat(ctx, state, limit, now)
	}

	args := []string{"-tunlp"}
	output, cmdErr := runCommand(ctx, defaultCommandTimeout, "ss", args...)
	var conns []NetworkConnectionInfo
	source := "ss"

	if cmdErr != nil {
		// Try netstat as fallback.
		_, nsErr := exec.LookPath("netstat")
		if nsErr == nil {
			return networkConnectionInspectorNetstat(ctx, state, limit, now)
		}
		result := NetworkConnectionInspectorResult{
			Connections: []NetworkConnectionInfo{},
			Count:       0,
			Source:      "ss",
			Message:     fmt.Sprintf("ss -tunlp failed: %v", cmdErr),
			Timestamp:   now,
		}
		return result, "network_connection_inspector: ss failed", "review", cmdErr
	}

	conns = parseSSOutput(output.Stdout, state, limit)
	summary := fmt.Sprintf("network_connection_inspector found %d connections (state=%s)", len(conns), state)

	result := NetworkConnectionInspectorResult{
		Connections: conns,
		Count:       len(conns),
		Source:      source,
		Message:     "network connection inspection completed",
		Timestamp:   now,
	}
	return result, summary, "low", nil
}

func networkConnectionInspectorNetstat(ctx context.Context, state string, limit int, now time.Time) (any, string, string, error) {
	args := []string{"-tunlp"}
	output, cmdErr := runCommand(ctx, defaultCommandTimeout, "netstat", args...)
	if cmdErr != nil {
		result := NetworkConnectionInspectorResult{
			Connections: []NetworkConnectionInfo{},
			Count:       0,
			Source:      "netstat",
			Message:     fmt.Sprintf("netstat -tunlp failed: %v", cmdErr),
			Timestamp:   now,
		}
		return result, "network_connection_inspector: netstat failed", "review", cmdErr
	}

	conns := parseNetstatOutput(output.Stdout, state, limit)
	summary := fmt.Sprintf("network_connection_inspector found %d connections (state=%s)", len(conns), state)

	result := NetworkConnectionInspectorResult{
		Connections: conns,
		Count:       len(conns),
		Source:      "netstat",
		Message:     "network connection inspection completed",
		Timestamp:   now,
	}
	return result, summary, "low", nil
}

func networkConnectionInspectorWindows(ctx context.Context, state string, limit int, now time.Time) (any, string, string, error) {
	_, err := exec.LookPath("netstat")
	if err != nil {
		result := NetworkConnectionInspectorResult{
			Connections: []NetworkConnectionInfo{},
			Count:       0,
			Source:      "",
			Message:     "netstat command not found",
			Timestamp:   now,
		}
		return result, "network_connection_inspector: netstat not found", "low", err
	}

	args := []string{"-ano"}
	output, cmdErr := runCommand(ctx, defaultCommandTimeout, "netstat", args...)
	if cmdErr != nil {
		result := NetworkConnectionInspectorResult{
			Connections: []NetworkConnectionInfo{},
			Count:       0,
			Source:      "netstat",
			Message:     fmt.Sprintf("netstat -ano failed: %v", cmdErr),
			Timestamp:   now,
		}
		return result, "network_connection_inspector: netstat failed", "review", cmdErr
	}

	conns := parseNetstatOutput(output.Stdout, state, limit)
	summary := fmt.Sprintf("network_connection_inspector found %d connections (state=%s)", len(conns), state)

	result := NetworkConnectionInspectorResult{
		Connections: conns,
		Count:       len(conns),
		Source:      "netstat",
		Message:     "network connection inspection completed",
		Timestamp:   now,
	}
	return result, summary, "low", nil
}

// parseSSOutput parses `ss -tunlp` output.
func parseSSOutput(output string, filterState string, limit int) []NetworkConnectionInfo {
	conns := make([]NetworkConnectionInfo, 0, limit)
	lines := strings.Split(output, "\n")
	// Skip header line.
	startIdx := 0
	if len(lines) > 0 && strings.HasPrefix(strings.ToLower(strings.TrimSpace(lines[0])), "netid") {
		startIdx = 1
	}

	for _, line := range lines[startIdx:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		// ss -tunlp format: Netid State Recv-Q Send-Q Local Address:Port Peer Address:Port Process
		proto := fields[0]
		connState := fields[1]
		localAddr := fields[4]
		peerAddr := ""
		if len(fields) > 5 {
			peerAddr = fields[5]
		}
		proc := ""
		if len(fields) > 6 {
			proc = strings.Join(fields[6:], " ")
		}

		if filterState != "" && filterState != "ALL" && !strings.EqualFold(connState, filterState) {
			continue
		}

		conns = append(conns, NetworkConnectionInfo{
			Protocol:     proto,
			State:        connState,
			LocalAddress: localAddr,
			PeerAddress:  peerAddr,
			Process:      proc,
		})

		if len(conns) >= limit {
			break
		}
	}
	return conns
}

// parseNetstatOutput parses `netstat -tunlp` or `netstat -ano` output.
func parseNetstatOutput(output string, filterState string, limit int) []NetworkConnectionInfo {
	conns := make([]NetworkConnectionInfo, 0, limit)
	lines := strings.Split(output, "\n")
	// Skip header lines.
	startIdx := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(strings.ToLower(trimmed), "proto") || strings.HasPrefix(strings.ToLower(trimmed), "active") {
			startIdx = i + 1
			continue
		}
		break
	}

	for _, line := range lines[startIdx:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		proto := strings.ToLower(fields[0])
		localAddr := fields[3]
		peerAddr := ""
		if len(fields) > 4 {
			peerAddr = fields[4]
		}

		connState := ""
		proc := ""
		if len(fields) == 5 {
			// Windows netstat -ano: Proto Local Address Foreign Address State PID
			connState = fields[4]
			if filterState != "" && filterState != "ALL" && !strings.EqualFold(connState, filterState) {
				continue
			}
		} else if len(fields) >= 6 {
			// Linux netstat -tunlp: Proto Recv-Q Send-Q Local Address Foreign Address State PID/Program name
			connState = fields[5]
			if filterState != "" && filterState != "ALL" && !strings.EqualFold(connState, filterState) {
				continue
			}
			if len(fields) > 6 {
				proc = strings.Join(fields[6:], " ")
			}
		}

		conns = append(conns, NetworkConnectionInfo{
			Protocol:     proto,
			State:        connState,
			LocalAddress: localAddr,
			PeerAddress:  peerAddr,
			Process:      proc,
		})

		if len(conns) >= limit {
			break
		}
	}
	return conns
}
