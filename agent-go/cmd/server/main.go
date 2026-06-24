package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"kylin-guard-agent/agent-go/internal/agent"
	"kylin-guard-agent/agent-go/internal/auditclient"
	"kylin-guard-agent/agent-go/internal/config"
	einoruntime "kylin-guard-agent/agent-go/internal/eino"
	"kylin-guard-agent/agent-go/internal/logtrace"
	"kylin-guard-agent/agent-go/internal/mcpserver"
	"kylin-guard-agent/agent-go/internal/security"
	"kylin-guard-agent/agent-go/internal/tools"
)

const serviceVersion = "0.1.0"
const maxAgentRequestBytes = 1 << 20
const agentRequestTimeout = 180 * time.Second

func main() {
	cfg := config.Load()
	registry := tools.NewDefaultRegistry()
	traceStore := logtrace.NewStore()
	auditor := auditclient.NewHTTPClient(cfg.AuditCoreURL)
	einoAdapter := einoruntime.NewRuntimeAdapter(einoruntime.NewRuntime(registry, auditor, traceStore, einoruntime.RuntimeConfig{
		RuntimeEnabled: cfg.EinoRuntimeEnabled,
		GraphEnabled:   cfg.EinoGraphEnabled,
		LLMEnabled:     cfg.EinoLLMEnabled,
		RuntimeName:    einoruntime.DefaultRuntimeName,
		Route:          einoruntime.DefaultRoute,
		ToolProtocol:   tools.ToolProtocol,
		Version:        einoruntime.RuntimeVersion,
		LLMProvider:    cfg.EinoLLMProvider,
		LLMEndpoint:    cfg.EinoLLMEndpoint,
		LLMModel:       cfg.EinoLLMModel,
		LLMAPIKey:      cfg.EinoLLMAPIKey,
	}))
	runStore := newAgentRunStoreFromConfig(cfg.RunStoreDir, cfg.RunStoreLimit)
	toolPolicy := security.NewToolPolicy()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/api/os/info", osInfoHandler(registry, traceStore))
	mux.HandleFunc("/api/agent/run", agentRunHandler(einoAdapter, runStore))
	mux.HandleFunc("/api/agent/run-eino", agentRunEinoHandler(einoAdapter, runStore))
	mux.HandleFunc("/api/agent/runs", agentRunListHandler(runStore))
	mux.HandleFunc("/api/agent/runs/", agentRunsHandler(runStore))
	mux.HandleFunc("/api/agent/runtime-status", runtimeStatusHandler(cfg))
	mux.HandleFunc("/api/agent/capabilities", capabilitiesHandler(registry))
	mux.HandleFunc("/api/agent/acceptance-summary", acceptanceSummaryHandler())
	mux.HandleFunc("/api/tools", toolsListHandler(registry))
	mux.HandleFunc("/api/tools/call", toolCallHandler(registry, auditor, traceStore, toolPolicy))
	mux.HandleFunc("/api/tools/", toolDetailHandler(registry))
	mux.Handle("/mcp", mcpserver.NewHTTPHandler(mcpserver.Dependencies{
		Registry:   registry,
		Policy:     toolPolicy,
		TraceStore: traceStore,
		Auditor:    auditor,
	}))

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      100 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	log.Printf("kylin-guard-agent listening on %s", cfg.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "kylin-guard-agent",
		"version": serviceVersion,
	})
}

func osInfoHandler(registry *tools.Registry, traceStore *logtrace.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}

		result, err := registry.Invoke(r.Context(), "os_info", map[string]any{
			"source": "/api/os/info",
		})
		traceStore.Add(result.Trace)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, result.Output)
	}
}

func agentRunHandler(agentAdapter agent.AgentAdapter, store *agentRunStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodPost) {
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), agentRequestTimeout)
		defer cancel()
		r = r.WithContext(ctx)
		r.Body = http.MaxBytesReader(w, r.Body, maxAgentRequestBytes)
		var req agent.RunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body", nil)
			return
		}
		req.Task = strings.TrimSpace(req.Task)
		if req.Task == "" {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "task is required", nil)
			return
		}

		if !agentAdapter.Enabled() {
			writeAPIError(w, http.StatusServiceUnavailable, "AGENT_UNAVAILABLE", agentAdapter.Name()+" is disabled", nil)
			return
		}

		resp, err := agentAdapter.Run(r.Context(), req)
		if err != nil {
			code := "AGENT_RUN_FAILED"
			status := http.StatusInternalServerError
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				code = "AGENT_TIMEOUT"
				status = http.StatusGatewayTimeout
			}
			writeAPIError(w, status, code, err.Error(), nil)
			return
		}
		resp = store.Save(resp)

		writeJSON(w, http.StatusOK, resp)
	}
}

func agentRunEinoHandler(einoAdapter agent.AgentAdapter, store *agentRunStore) http.HandlerFunc {
	return agentRunHandler(einoAdapter, store)
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	w.Header().Set("Allow", method)
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	return false
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("write json response failed: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeAPIError(w, status, defaultErrorCode(status), message, nil)
}

func writeAPIError(w http.ResponseWriter, status int, code, message string, details map[string]any) {
	if details == nil {
		details = map[string]any{}
	}
	writeJSON(w, status, map[string]any{"error": map[string]any{
		"code": code, "message": message, "details": details,
	}})
}

func defaultErrorCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "INVALID_REQUEST"
	case http.StatusNotFound:
		return "RUN_NOT_FOUND"
	case http.StatusMethodNotAllowed:
		return "METHOD_NOT_ALLOWED"
	default:
		return "INTERNAL_ERROR"
	}
}
