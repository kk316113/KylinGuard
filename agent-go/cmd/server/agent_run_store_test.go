package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kylin-guard-agent/agent-go/internal/agent"
	"kylin-guard-agent/agent-go/internal/auditclient"
	"kylin-guard-agent/agent-go/internal/logtrace"
	"kylin-guard-agent/agent-go/internal/report"
)

func TestAgentRunStorePersistsReloadsAndListsRuns(t *testing.T) {
	dir := t.TempDir()
	store := newAgentRunStoreWithOptions(dir, 10)

	first := store.Save(sampleStoredRun("kg-001", "检查 SSH", "review"))
	second := store.Save(sampleStoredRun("kg-002", "检查资源", "allow"))

	reloaded := newAgentRunStoreWithOptions(dir, 10)
	if err := reloaded.load(); err != nil {
		t.Fatalf("load persisted runs: %v", err)
	}
	if got, ok := reloaded.Get(first.RunID); !ok || got.Task != first.Task {
		t.Fatalf("expected first run after reload, got ok=%v run=%#v", ok, got)
	}
	latest, ok := reloaded.Get("latest")
	if !ok || latest.RunID != second.RunID {
		t.Fatalf("expected latest run %q after reload, got ok=%v run=%#v", second.RunID, ok, latest)
	}
	list := reloaded.List(50, "")
	if list.Count != 2 || list.Runs[0].RunID != second.RunID || list.Runs[1].RunID != first.RunID {
		t.Fatalf("expected newest-first list, got %#v", list)
	}
}

func TestAgentRunSQLiteStorePersistsNormalizedTablesAndReloads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "kylinguard.db")
	store := newAgentRunSQLiteStoreWithOptions(dbPath, 10)
	if err := store.load(); err != nil {
		t.Fatalf("load sqlite store: %v", err)
	}
	defer store.Close()

	run := sampleStoredRun("kg-sqlite", "我的 key 是 sk-testsecret123456", "review")
	run.FinalAnswer = "不要泄露 DEEPSEEK_API_KEY=sk-testsecret123456"
	saved := store.Save(run)

	reloaded := newAgentRunSQLiteStoreWithOptions(dbPath, 10)
	if err := reloaded.load(); err != nil {
		t.Fatalf("reload sqlite store: %v", err)
	}
	defer reloaded.Close()
	got, ok := reloaded.Get(saved.RunID)
	if !ok || got.RunID != saved.RunID || got.Task != saved.Task {
		t.Fatalf("expected saved run after sqlite reload, ok=%v run=%#v", ok, got)
	}
	list := reloaded.List(10, "")
	if list.Count != 1 || list.Runs[0].RunID != saved.RunID {
		t.Fatalf("expected sqlite list with saved run, got %#v", list)
	}

	assertSQLiteCount(t, reloaded, "agent_runs", 1)
	assertSQLiteCount(t, reloaded, "agent_steps", 1)
	assertSQLiteCount(t, reloaded, "tool_traces", 1)
	assertSQLiteCount(t, reloaded, "audit_results", 1)

	var payload string
	if err := reloaded.db.QueryRow(`SELECT sanitized_json FROM agent_runs WHERE run_id = ?`, saved.RunID).Scan(&payload); err != nil {
		t.Fatalf("read sqlite payload: %v", err)
	}
	if strings.Contains(payload, "sk-testsecret123456") {
		t.Fatalf("sqlite payload leaked secret: %s", payload)
	}
	if !strings.Contains(payload, "[REDACTED]") {
		t.Fatalf("expected sqlite payload to include redaction marker: %s", payload)
	}
}

func TestAgentRunSQLiteStoreImportsJSONRuns(t *testing.T) {
	dir := t.TempDir()
	jsonStore := newAgentRunStoreWithOptions(dir, 10)
	jsonStore.Save(sampleStoredRun("kg-json-import", "legacy json run", "allow"))

	sqliteStore := newAgentRunStoreFromConfig(runStoreBackendSQLite, dir, filepath.Join(t.TempDir(), "kylinguard.db"), 10)
	defer sqliteStore.Close()
	got, ok := sqliteStore.Get("kg-json-import")
	if !ok || got.RunID != "kg-json-import" {
		t.Fatalf("expected sqlite store to import legacy JSON run, ok=%v run=%#v", ok, got)
	}
	assertSQLiteCount(t, sqliteStore, "agent_runs", 1)
}

func TestAgentRunStoreEnforcesLimitAndSkipsCorruptFiles(t *testing.T) {
	dir := t.TempDir()
	store := newAgentRunStoreWithOptions(dir, 2)
	store.Save(sampleStoredRun("kg-001", "one", "review"))
	store.Save(sampleStoredRun("kg-002", "two", "review"))
	store.Save(sampleStoredRun("kg-003", "three", "review"))
	if _, err := os.Stat(filepath.Join(dir, "kg-001.json")); !os.IsNotExist(err) {
		t.Fatalf("expected oldest run file to be removed, stat err=%v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "corrupt.json"), []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("write corrupt run: %v", err)
	}

	reloaded := newAgentRunStoreWithOptions(dir, 2)
	if err := reloaded.rebuildFromFiles(); err != nil {
		t.Fatalf("rebuild from files should skip corrupt entries: %v", err)
	}
	if list := reloaded.List(50, ""); list.Count != 2 {
		t.Fatalf("expected two retained runs after rebuild, got %#v", list)
	}
}

func TestAgentRunStoreRedactsSecretsInPersistenceAndMarkdown(t *testing.T) {
	dir := t.TempDir()
	store := newAgentRunStoreWithOptions(dir, 10)
	run := sampleStoredRun("kg-secret", "我的 key 是 sk-testsecret123456", "review")
	run.FinalAnswer = "请不要泄露 OPENAI_COMPATIBLE_API_KEY=sk-testsecret123456"
	run.SecurityReport.AuditMetadata["api_key"] = "sk-testsecret123456"
	saved := store.Save(run)

	data, err := os.ReadFile(filepath.Join(dir, saved.RunID+".json"))
	if err != nil {
		t.Fatalf("read persisted run: %v", err)
	}
	if strings.Contains(string(data), "sk-testsecret123456") {
		t.Fatalf("persisted run leaked secret: %s", string(data))
	}
	if strings.Contains(buildRunMarkdownReport(saved), "sk-testsecret123456") {
		t.Fatal("markdown report leaked secret")
	}
	if !strings.Contains(buildRunMarkdownReport(saved), "[REDACTED]") {
		t.Fatal("expected markdown report to include redaction marker")
	}
}

func TestAgentRunQueryEndpointsListMarkdownAndArtifact(t *testing.T) {
	store := newAgentRunStore()
	run := store.Save(sampleStoredRun("kg-artifact", "检查配置漂移", "review"))
	handler := agentRunsHandler(store)

	var list agentRunListResponse
	getJSON(t, agentRunListHandler(store), "/api/agent/runs?limit=5", http.StatusOK, &list)
	if list.Count != 1 || list.Runs[0].RunID != run.RunID {
		t.Fatalf("expected run list, got %#v", list)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/agent/runs/"+run.RunID+"/report.md", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), "text/markdown") {
		t.Fatalf("expected markdown report response, code=%d headers=%v body=%s", rec.Code, rec.Header(), rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "KylinGuard 会话报告") {
		t.Fatalf("expected markdown report body, got %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/agent/runs/"+run.RunID+"/risk-graph/artifact", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected artifact HTTP 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var artifact riskGraphArtifact
	if err := json.Unmarshal(rec.Body.Bytes(), &artifact); err != nil {
		t.Fatalf("decode artifact: %v", err)
	}
	if artifact.SchemaVersion == "" || artifact.Run.RunID != run.RunID || len(artifact.ToolTraceRefs) == 0 {
		t.Fatalf("unexpected artifact: %#v", artifact)
	}
}

func sampleStoredRun(runID, task, decision string) agent.AgentRunResponse {
	now := time.Now().UTC()
	return agent.AgentRunResponse{
		RunID:     runID,
		TaskID:    runID,
		Task:      task,
		RunStatus: agent.RunStatusCompleted,
		CreatedAt: now.Format(time.RFC3339),
		Decision:  decision,
		Summary:   "agent run completed",
		AgentMode: "agent_loop",
		AgentSteps: []map[string]any{
			{
				"step_index":           1,
				"tool_name":            "configuration_drift_detector",
				"policy_decision":      "allow",
				"user_visible_summary": "检查 RPM 配置漂移",
			},
		},
		ToolTrace: []logtrace.ToolTrace{
			{
				StepID:          "step-001",
				ToolName:        "configuration_drift_detector",
				OutputSummary:   "checked 1 package",
				Status:          "ok",
				StartedAt:       now,
				FinishedAt:      now,
				OperationType:   "read",
				ResourceType:    "rpm_database",
				BoundaryLevel:   "sensitive_system_resource",
				AllowedByPolicy: true,
			},
		},
		AuditResult: auditclient.Result{
			Decision:      decision,
			RiskScore:     0.2,
			Violations:    []auditclient.Violation{},
			EvidenceChain: []auditclient.EvidenceItem{{StepID: "step-001", ToolName: "configuration_drift_detector", Reason: "drift evidence"}},
			RiskGraph: &auditclient.RiskGraph{
				Nodes: []map[string]any{{"id": "step-001", "step_id": "step-001", "tool_name": "configuration_drift_detector", "risk_level": "low", "decision": decision}},
				Edges: []map[string]any{},
			},
			Method:  "local-safety-fallback",
			Message: "local safety fallback",
		},
		SecurityReport: &report.SecurityReport{
			Title:           "KylinGuard Security Report",
			OverallDecision: decision,
			Summary:         "安全审计摘要",
			AuditMetadata: map[string]any{
				"chat_model": "remote-llm-deepseek-openai_compatible",
			},
		},
		FinalAnswer: "已完成检查。",
	}
}

func assertSQLiteCount(t *testing.T, store *agentRunStore, table string, expected int) {
	t.Helper()
	if store == nil || store.db == nil {
		t.Fatal("sqlite store is not open")
	}
	var count int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if count != expected {
		t.Fatalf("expected %s count %d, got %d", table, expected, count)
	}
}
