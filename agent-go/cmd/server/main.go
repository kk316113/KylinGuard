package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"kylin-guard-agent/agent-go/internal/agent"
	"kylin-guard-agent/agent-go/internal/auditclient"
	"kylin-guard-agent/agent-go/internal/config"
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
	stableAdapter := agent.NewStableRuntimeAdapter(runtime)
	einoAdapter := agent.NewEinoAdapter(cfg.EinoEnabled)
	toolPolicy := security.NewToolPolicy()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/api/os/info", osInfoHandler(registry, traceStore))
	mux.HandleFunc("/api/agent/run", agentRunHandler(runtime))
	mux.HandleFunc("/api/agent/run-eino", agentRunEinoHandler(einoAdapter, stableAdapter))
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

func agentRunEinoHandler(einoAdapter agent.AgentAdapter, fallbackAdapter agent.AgentAdapter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodPost) {
			return
		}

		var req agent.AgentRunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		if einoAdapter.Enabled() {
			resp, err := einoAdapter.Run(r.Context(), req)
			if err == nil {
				writeJSON(w, http.StatusOK, resp)
				return
			}
		}

		resp, err := fallbackAdapter.Run(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		resp.Summary = appendSummary(resp.Summary, einoFallbackSummary(einoAdapter))
		markEinoFallbackReport(&resp, einoFallbackSummary(einoAdapter))
		writeJSON(w, http.StatusOK, resp)
	}
}

func einoFallbackSummary(adapter agent.AgentAdapter) string {
	type fallbackSummarizer interface {
		FallbackSummary() string
	}
	if summarizer, ok := adapter.(fallbackSummarizer); ok {
		return summarizer.FallbackSummary()
	}
	return "eino adapter disabled, stable runtime fallback used"
}

func appendSummary(summary string, detail string) string {
	if summary == "" {
		return detail
	}
	return summary + "; " + detail
}

func markEinoFallbackReport(resp *agent.AgentRunResponse, detail string) {
	if resp == nil || resp.SecurityReport == nil {
		return
	}
	if resp.SecurityReport.AuditMetadata == nil {
		resp.SecurityReport.AuditMetadata = map[string]any{}
	}
	resp.SecurityReport.AuditMetadata["route"] = "eino-fallback"
	resp.SecurityReport.Summary = appendSummary(resp.SecurityReport.Summary, "Eino fallback detail: "+detail+". The fallback path did not bypass intent_guard, planner, semantic trace, or TraceShield audit.")
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
