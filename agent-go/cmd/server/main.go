package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"kylin-guard-agent/agent-go/internal/agent"
	"kylin-guard-agent/agent-go/internal/auditclient"
	"kylin-guard-agent/agent-go/internal/config"
	einoruntime "kylin-guard-agent/agent-go/internal/eino"
	"kylin-guard-agent/agent-go/internal/logtrace"
	"kylin-guard-agent/agent-go/internal/security"
	"kylin-guard-agent/agent-go/internal/tools"
)

const serviceVersion = "0.1.0"

func main() {
	cfg := config.Load()
	registry := tools.NewDefaultRegistry()
	traceStore := logtrace.NewStore()
	auditor := auditclient.NewHTTPClient(cfg.AuditCoreURL)
	runtime := agent.NewRuntime(registry, auditor, traceStore)
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
	toolPolicy := security.NewToolPolicy()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/api/os/info", osInfoHandler(registry, traceStore))
	mux.HandleFunc("/api/agent/run", agentRunHandler(runtime))
	mux.HandleFunc("/api/agent/run-eino", agentRunEinoHandler(einoAdapter))
	mux.HandleFunc("/api/agent/runtime-status", runtimeStatusHandler(cfg))
	mux.HandleFunc("/api/agent/capabilities", capabilitiesHandler(registry))
	mux.HandleFunc("/api/agent/acceptance-summary", acceptanceSummaryHandler())
	mux.HandleFunc("/api/tools", toolsListHandler(registry))
	mux.HandleFunc("/api/tools/call", toolCallHandler(registry, auditor, traceStore, toolPolicy))
	mux.HandleFunc("/api/tools/", toolDetailHandler(registry))

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
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

func agentRunHandler(runtime *agent.Runtime) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodPost) {
			return
		}

		var req agent.RunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		resp, err := runtime.Run(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

func agentRunEinoHandler(einoAdapter agent.AgentAdapter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodPost) {
			return
		}

		var req agent.AgentRunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		if !einoAdapter.Enabled() {
			writeError(w, http.StatusServiceUnavailable, "eino graph runtime is disabled")
			return
		}

		resp, err := einoAdapter.Run(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
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
	writeJSON(w, status, map[string]string{"error": message})
}
