package tools

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"kylin-guard-agent/agent-go/internal/execproxy"
	"kylin-guard-agent/agent-go/internal/logtrace"
)

type PortCheckerResult struct {
	Host             string                     `json:"host"`
	Port             int                        `json:"port"`
	Address          string                     `json:"address"`
	Open             bool                       `json:"open"`
	Message          string                     `json:"message"`
	Timestamp        time.Time                  `json:"timestamp"`
	ExecutionContext *logtrace.ExecutionContext `json:"-"`
}

func PortChecker(ctx context.Context, input map[string]any) (any, string, string, error) {
	host := stringValue(input, "host", "127.0.0.1")
	port := intValue(input, "port", 8080)
	if port <= 0 || port > 65535 {
		return nil, "invalid port", "review", fmt.Errorf("invalid port: %d", port)
	}

	address := net.JoinHostPort(host, strconv.Itoa(port))
	dialer := net.Dialer{Timeout: 800 * time.Millisecond}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	open := err == nil
	if conn != nil {
		_ = conn.Close()
	}

	message := "port is closed or unreachable"
	if open {
		message = "port is open"
	}

	nec := execproxy.NativeExecutionContext(execproxy.ProfileLowRead, "native_go:net.Dial", "TCP port connectivity check via Go net.Dialer")
	result := PortCheckerResult{
		Host:             host,
		Port:             port,
		Address:          address,
		Open:             open,
		Message:          message,
		Timestamp:        time.Now().UTC(),
		ExecutionContext: ecPtr(nec),
	}
	return result, fmt.Sprintf("%s: %s", address, message), "low", nil
}
