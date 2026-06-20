package mcpserver

import (
	"context"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"kylin-guard-agent/agent-go/internal/auditclient"
	"kylin-guard-agent/agent-go/internal/logtrace"
	"kylin-guard-agent/agent-go/internal/security"
	"kylin-guard-agent/agent-go/internal/tools"
)

func TestServerListsOnlyDirectPolicyToolsAndExecutesThroughTraceChain(t *testing.T) {
	registry := tools.NewRegistry()
	registry.RegisterWithMetadata("os_info", func(context.Context, map[string]any) (any, string, string, error) {
		return map[string]any{"platform": "test/loong64"}, "test OS inspected", "low", nil
	}, testMetadata("os_info", map[string]any{}))

	disabled := testMetadata("disabled_tool", map[string]any{})
	disabled.DirectCallAllowed = false
	registry.RegisterWithMetadata("disabled_tool", func(context.Context, map[string]any) (any, string, string, error) {
		t.Fatal("disabled tool must not execute")
		return nil, "", "", nil
	}, disabled)

	traceStore := logtrace.NewStore()
	server := NewServer(Dependencies{
		Registry:   registry,
		Policy:     security.NewToolPolicy(),
		TraceStore: traceStore,
		Auditor:    auditclient.NewMockClient(),
	})
	clientSession, serverSession := connectTestClient(t, server)
	defer clientSession.Close()
	defer serverSession.Close()

	listed, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(listed.Tools) != 1 || listed.Tools[0].Name != "os_info" {
		t.Fatalf("unexpected MCP tools: %#v", listed.Tools)
	}
	if !listed.Tools[0].Annotations.ReadOnlyHint {
		t.Fatal("expected read-only MCP annotation")
	}

	result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{Name: "os_info"})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected successful MCP result: %#v", result)
	}
	traces := traceStore.List()
	if len(traces) != 1 || traces[0].ToolName != "os_info" || traces[0].Status != "ok" {
		t.Fatalf("unexpected trace chain: %#v", traces)
	}
}

func TestServerDeniesInjectedArgumentsBeforeToolExecution(t *testing.T) {
	var calls atomic.Int32
	registry := tools.NewRegistry()
	registry.RegisterWithMetadata("process_inspector", func(context.Context, map[string]any) (any, string, string, error) {
		calls.Add(1)
		return map[string]any{}, "must not execute", "low", nil
	}, testMetadata("process_inspector", map[string]any{
		"name": map[string]any{"type": "string"},
	}))

	traceStore := logtrace.NewStore()
	server := NewServer(Dependencies{
		Registry:   registry,
		Policy:     security.NewToolPolicy(),
		TraceStore: traceStore,
		Auditor:    auditclient.NewMockClient(),
	})
	clientSession, serverSession := connectTestClient(t, server)
	defer clientSession.Close()
	defer serverSession.Close()

	result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "process_inspector",
		Arguments: map[string]any{"name": "sshd; rm -rf /"},
	})
	if err != nil {
		t.Fatalf("policy denial should be returned as an MCP tool result: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected MCP tool error for policy denial")
	}
	if calls.Load() != 0 {
		t.Fatalf("denied handler executed %d times", calls.Load())
	}
	traces := traceStore.List()
	if len(traces) != 1 || traces[0].Status != "denied" || traces[0].AllowedByPolicy {
		t.Fatalf("unexpected denied trace: %#v", traces)
	}
}

func TestStreamableHTTPHandlerNegotiatesMCPAndListsTools(t *testing.T) {
	registry := tools.NewRegistry()
	registry.RegisterWithMetadata("os_info", func(context.Context, map[string]any) (any, string, string, error) {
		return map[string]any{"platform": "test/loong64"}, "test OS inspected", "low", nil
	}, testMetadata("os_info", map[string]any{}))

	httpServer := httptest.NewServer(NewHTTPHandler(Dependencies{
		Registry:   registry,
		Policy:     security.NewToolPolicy(),
		TraceStore: logtrace.NewStore(),
		Auditor:    auditclient.NewMockClient(),
	}))
	defer httpServer.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "kylin-guard-http-test", Version: "0.1.0"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:   httpServer.URL,
		MaxRetries: -1,
	}, nil)
	if err != nil {
		t.Fatalf("streamable HTTP connect failed: %v", err)
	}
	defer session.Close()

	listed, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("streamable HTTP ListTools failed: %v", err)
	}
	if len(listed.Tools) != 1 || listed.Tools[0].Name != "os_info" {
		t.Fatalf("unexpected HTTP MCP tools: %#v", listed.Tools)
	}
}

func TestDefaultServerPublishesCompetitionSensingTools(t *testing.T) {
	server := NewServer(Dependencies{
		Registry:   tools.NewDefaultRegistry(),
		Policy:     security.NewToolPolicy(),
		TraceStore: logtrace.NewStore(),
		Auditor:    auditclient.NewMockClient(),
	})
	clientSession, serverSession := connectTestClient(t, server)
	defer clientSession.Close()
	defer serverSession.Close()

	listed, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	names := make(map[string]bool, len(listed.Tools))
	for _, tool := range listed.Tools {
		names[tool.Name] = true
	}
	for _, required := range []string{"open_file_inspector", "process_inspector", "disk_io_checker"} {
		if !names[required] {
			t.Fatalf("MCP tool list missing %s: %#v", required, names)
		}
	}
	if names["safe_shell"] {
		t.Fatal("safe_shell must not be published by MCP")
	}
}

func connectTestClient(t *testing.T, server *mcp.Server) (*mcp.ClientSession, *mcp.ServerSession) {
	t.Helper()
	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect failed: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "kylin-guard-test", Version: "0.1.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		serverSession.Close()
		t.Fatalf("client connect failed: %v", err)
	}
	return clientSession, serverSession
}

func testMetadata(name string, properties map[string]any) tools.ToolMetadata {
	return tools.ToolMetadata{
		Name:              name,
		Description:       "test MCP tool",
		InputSchema:       map[string]any{"type": "object", "properties": properties},
		OutputSchema:      map[string]any{"type": "object"},
		RiskLevel:         "low",
		PermissionScope:   "test_read",
		OperationType:     "read",
		ResourceType:      "test_resource",
		BoundaryLevel:     "low",
		AllowedByPolicy:   true,
		Enabled:           true,
		DirectCallAllowed: true,
	}
}
