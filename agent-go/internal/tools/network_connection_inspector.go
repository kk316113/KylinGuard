package tools

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"kylin-guard-agent/agent-go/internal/execproxy"
	"kylin-guard-agent/agent-go/internal/logtrace"
)

type NetworkConnectionInspectorResult struct {
	Connections      []NetworkConnectionInfo    `json:"connections"`
	Count            int                        `json:"count"`
	Source           string                     `json:"source"`
	Message          string                     `json:"message"`
	Timestamp        time.Time                  `json:"timestamp"`
	ExecutionContext *logtrace.ExecutionContext `json:"-"`
}

type NetworkConnectionInfo struct {
	Protocol     string `json:"protocol"`
	State        string `json:"state"`
	LocalAddress string `json:"local_address"`
	PeerAddress  string `json:"peer_address"`
	Process      string `json:"process,omitempty"`
}

var allowedNetworkStates = map[string]bool{
	"LISTEN": true, "ESTABLISHED": true, "TIME-WAIT": true, "TIME_WAIT": true,
	"CLOSE-WAIT": true, "CLOSE_WAIT": true, "ALL": true,
}

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

	if !allowedNetworkStates[state] {
		ec := execproxy.DeniedContext("network_connection_inspector", fmt.Sprintf("invalid state %q", state))
		result := NetworkConnectionInspectorResult{
			Connections:      []NetworkConnectionInfo{},
			Count:            0,
			Message:          fmt.Sprintf("invalid state %q: allowed values are LISTEN, ESTABLISHED, TIME-WAIT, CLOSE-WAIT, ALL", state),
			Timestamp:        now,
			ExecutionContext: ecPtr(ec),
		}
		return result, "network_connection_inspector: invalid state rejected", "low", fmt.Errorf("invalid network state %q", state)
	}
	state = normalizeNetworkState(state)

	switch runtime.GOOS {
	case "linux":
		return networkConnectionInspectorLinux(ctx, state, limit, now)
	case "windows":
		return networkConnectionInspectorWindows(ctx, state, limit, now)
	default:
		nec := execproxy.NativeExecutionContext(execproxy.ProfileLowRead, "unsupported_platform", "network_connection_inspector unsupported")
		result := NetworkConnectionInspectorResult{
			Connections:      []NetworkConnectionInfo{},
			Count:            0,
			Message:          fmt.Sprintf("network_connection_inspector is not supported on %s", runtime.GOOS),
			Timestamp:        now,
			ExecutionContext: ecPtr(nec),
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
	exec := execproxy.NewExecutor()
	// Try ss first, then netstat.
	ssResult, ssErr := exec.Execute(ctx, execproxy.CommandSpec{
		ToolName: "network_connection_inspector",
		Profile:  execproxy.ProfileLowRead,
		Command:  "ss",
		Args:     []string{"-tunlp"},
		Reason:   "inspect network listening ports and connections",
	})
	if ssErr == nil && ssResult.Status == "ok" {
		conns := parseSSOutput(ssResult.Stdout, state, limit)
		summary := fmt.Sprintf("network_connection_inspector found %d connections (state=%s)", len(conns), state)
		r := NetworkConnectionInspectorResult{
			Connections:      conns,
			Count:            len(conns),
			Source:           "ss",
			Message:          "network connection inspection completed",
			Timestamp:        now,
			ExecutionContext: ecPtr(ssResult.Context),
		}
		return r, summary, "low", nil
	}
	// Fallback to netstat.
	nsResult, nsErr := exec.Execute(ctx, execproxy.CommandSpec{
		ToolName: "network_connection_inspector",
		Profile:  execproxy.ProfileLowRead,
		Command:  "netstat",
		Args:     []string{"-tunlp"},
		Reason:   "inspect network connections (fallback)",
	})
	if nsErr != nil || nsResult.Status == "error" {
		ec := nsResult.Context
		r := NetworkConnectionInspectorResult{
			Connections:      []NetworkConnectionInfo{},
			Count:            0,
			Source:           "netstat",
			Message:          fmt.Sprintf("ss and netstat failed: ss=%v, netstat=%v", ssErr, nsErr),
			Timestamp:        now,
			ExecutionContext: ecPtr(ec),
		}
		return r, "network_connection_inspector: ss and netstat failed", "review", nsErr
	}
	conns := parseNetstatOutput(nsResult.Stdout, state, limit)
	summary := fmt.Sprintf("network_connection_inspector found %d connections (state=%s)", len(conns), state)
	r := NetworkConnectionInspectorResult{
		Connections:      conns,
		Count:            len(conns),
		Source:           "netstat",
		Message:          "network connection inspection completed",
		Timestamp:        now,
		ExecutionContext: ecPtr(nsResult.Context),
	}
	return r, summary, "low", nil
}

func networkConnectionInspectorWindows(ctx context.Context, state string, limit int, now time.Time) (any, string, string, error) {
	exec := execproxy.NewExecutor()
	result, execErr := exec.Execute(ctx, execproxy.CommandSpec{
		ToolName: "network_connection_inspector",
		Profile:  execproxy.ProfileLowRead,
		Command:  "netstat",
		Args:     []string{"-ano"},
		Reason:   "inspect network connections on Windows",
	})
	if execErr != nil || result.Status == "error" {
		ec := result.Context
		r := NetworkConnectionInspectorResult{
			Connections:      []NetworkConnectionInfo{},
			Count:            0,
			Source:           "netstat",
			Message:          fmt.Sprintf("netstat -ano failed: %v", execErr),
			Timestamp:        now,
			ExecutionContext: ecPtr(ec),
		}
		return r, "network_connection_inspector: netstat failed", "review", execErr
	}
	conns := parseNetstatOutput(result.Stdout, state, limit)
	summary := fmt.Sprintf("network_connection_inspector found %d connections (state=%s)", len(conns), state)
	r := NetworkConnectionInspectorResult{
		Connections:      conns,
		Count:            len(conns),
		Source:           "netstat",
		Message:          "network connection inspection completed",
		Timestamp:        now,
		ExecutionContext: ecPtr(result.Context),
	}
	return r, summary, "low", nil
}

func parseSSOutput(output string, filterState string, limit int) []NetworkConnectionInfo {
	conns := make([]NetworkConnectionInfo, 0, limit)
	lines := strings.Split(output, "\n")
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
		conns = append(conns, NetworkConnectionInfo{Protocol: proto, State: connState, LocalAddress: localAddr, PeerAddress: peerAddr, Process: proc})
		if len(conns) >= limit {
			break
		}
	}
	return conns
}

func parseNetstatOutput(output string, filterState string, limit int) []NetworkConnectionInfo {
	conns := make([]NetworkConnectionInfo, 0, limit)
	lines := strings.Split(output, "\n")
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
			connState = fields[4]
			if filterState != "" && filterState != "ALL" && !strings.EqualFold(connState, filterState) {
				continue
			}
		} else if len(fields) >= 6 {
			connState = fields[5]
			if filterState != "" && filterState != "ALL" && !strings.EqualFold(connState, filterState) {
				continue
			}
			if len(fields) > 6 {
				proc = strings.Join(fields[6:], " ")
			}
		}
		conns = append(conns, NetworkConnectionInfo{Protocol: proto, State: connState, LocalAddress: localAddr, PeerAddress: peerAddr, Process: proc})
		if len(conns) >= limit {
			break
		}
	}
	return conns
}
