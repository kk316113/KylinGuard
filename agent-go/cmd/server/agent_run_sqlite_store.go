package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kylin-guard-agent/agent-go/internal/agent"
	"kylin-guard-agent/agent-go/internal/logtrace"

	_ "modernc.org/sqlite"
)

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS agent_runs (
	run_id TEXT PRIMARY KEY,
	task_id TEXT NOT NULL,
	task TEXT NOT NULL,
	scene_type TEXT,
	run_status TEXT,
	created_at TEXT NOT NULL,
	decision TEXT,
	agent_mode TEXT,
	chat_model TEXT,
	tool_trace_count INTEGER NOT NULL DEFAULT 0,
	agent_step_count INTEGER NOT NULL DEFAULT 0,
	sanitized_json TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_agent_runs_created_at ON agent_runs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_runs_decision ON agent_runs(decision);
CREATE INDEX IF NOT EXISTS idx_agent_runs_scene_type ON agent_runs(scene_type);

CREATE TABLE IF NOT EXISTS agent_steps (
	run_id TEXT NOT NULL,
	step_index INTEGER NOT NULL,
	tool_name TEXT,
	policy_decision TEXT,
	operation_type TEXT,
	resource_type TEXT,
	boundary_level TEXT,
	summary TEXT,
	audit_decision TEXT,
	payload_json TEXT NOT NULL,
	PRIMARY KEY (run_id, step_index),
	FOREIGN KEY (run_id) REFERENCES agent_runs(run_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_agent_steps_tool ON agent_steps(tool_name);
CREATE INDEX IF NOT EXISTS idx_agent_steps_boundary ON agent_steps(boundary_level);

CREATE TABLE IF NOT EXISTS tool_traces (
	run_id TEXT NOT NULL,
	step_id TEXT NOT NULL,
	step_index INTEGER NOT NULL,
	tool_name TEXT,
	status TEXT,
	operation_type TEXT,
	resource_type TEXT,
	resource_path TEXT,
	boundary_level TEXT,
	allowed_by_policy INTEGER NOT NULL DEFAULT 0,
	risk_hint TEXT,
	started_at TEXT,
	finished_at TEXT,
	output_summary TEXT,
	payload_json TEXT NOT NULL,
	PRIMARY KEY (run_id, step_id),
	FOREIGN KEY (run_id) REFERENCES agent_runs(run_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_tool_traces_tool ON tool_traces(tool_name);
CREATE INDEX IF NOT EXISTS idx_tool_traces_resource ON tool_traces(resource_type, resource_path);

CREATE TABLE IF NOT EXISTS audit_results (
	run_id TEXT PRIMARY KEY,
	decision TEXT,
	risk_score REAL,
	method TEXT,
	message TEXT,
	violations_json TEXT,
	evidence_chain_json TEXT,
	risk_graph_json TEXT,
	updated_at TEXT NOT NULL,
	FOREIGN KEY (run_id) REFERENCES agent_runs(run_id) ON DELETE CASCADE
);
`

func (s *agentRunStore) loadSQLite() error {
	if s == nil {
		return nil
	}
	db, err := s.openSQLite()
	if err != nil {
		return err
	}
	s.db = db

	rows, err := db.Query(`SELECT run_id, sanitized_json FROM agent_runs ORDER BY created_at ASC, updated_at ASC, run_id ASC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var runID string
		var raw string
		if err := rows.Scan(&runID, &raw); err != nil {
			return err
		}
		var resp agent.AgentRunResponse
		if err := json.Unmarshal([]byte(raw), &resp); err != nil {
			log.Printf("sqlite agent run store skipped %s: %v", runID, err)
			continue
		}
		if resp.RunID == "" {
			resp.RunID = runID
		}
		resp = sanitizeRunResponse(resp)
		s.runs[resp.RunID] = resp
		s.order = append(s.order, resp.RunID)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	s.pruneMissingLocked()
	if len(s.order) > 0 {
		s.latest = s.order[len(s.order)-1]
	}
	s.enforceLimitLocked()
	return nil
}

func (s *agentRunStore) openSQLite() (*sql.DB, error) {
	if s.db != nil {
		return s.db, nil
	}
	if strings.TrimSpace(s.dbPath) == "" {
		return nil, errors.New("empty sqlite run store path")
	}
	if err := ensureSQLiteParentDir(s.dbPath); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", s.dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON; PRAGMA busy_timeout = 5000;`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(sqliteSchema); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func ensureSQLiteParentDir(path string) error {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(path)), "file:") {
		return nil
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o700)
}

func (s *agentRunStore) persistSQLiteRunLocked(resp agent.AgentRunResponse) error {
	db, err := s.openSQLite()
	if err != nil {
		return err
	}
	s.db = db

	resp = sanitizeRunResponse(resp)
	payload, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	summary := summarizeRun(resp)

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
INSERT INTO agent_runs (
	run_id, task_id, task, scene_type, run_status, created_at, decision,
	agent_mode, chat_model, tool_trace_count, agent_step_count, sanitized_json, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(run_id) DO UPDATE SET
	task_id=excluded.task_id,
	task=excluded.task,
	scene_type=excluded.scene_type,
	run_status=excluded.run_status,
	created_at=excluded.created_at,
	decision=excluded.decision,
	agent_mode=excluded.agent_mode,
	chat_model=excluded.chat_model,
	tool_trace_count=excluded.tool_trace_count,
	agent_step_count=excluded.agent_step_count,
	sanitized_json=excluded.sanitized_json,
	updated_at=excluded.updated_at
`, summary.RunID, summary.TaskID, summary.Task, summary.SceneType, summary.RunStatus, summary.CreatedAt,
		summary.Decision, summary.AgentMode, summary.ChatModel, summary.ToolTraceCount, summary.AgentStepCount,
		string(payload), now); err != nil {
		return err
	}

	if _, err := tx.Exec(`DELETE FROM agent_steps WHERE run_id = ?`, resp.RunID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM tool_traces WHERE run_id = ?`, resp.RunID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM audit_results WHERE run_id = ?`, resp.RunID); err != nil {
		return err
	}

	for index, step := range resp.AgentSteps {
		if err := insertSQLiteAgentStep(tx, resp.RunID, index+1, step); err != nil {
			return err
		}
	}
	for index, trace := range resp.ToolTrace {
		if err := insertSQLiteToolTrace(tx, resp.RunID, index+1, trace); err != nil {
			return err
		}
	}
	if err := insertSQLiteAuditResult(tx, resp, now); err != nil {
		return err
	}

	return tx.Commit()
}

func insertSQLiteAgentStep(tx *sql.Tx, runID string, fallbackIndex int, step map[string]any) error {
	payload, err := json.Marshal(sanitizeMap(step))
	if err != nil {
		return err
	}
	stepIndex := intFromAny(step["step_index"])
	if stepIndex <= 0 {
		stepIndex = fallbackIndex
	}
	_, err = tx.Exec(`
INSERT INTO agent_steps (
	run_id, step_index, tool_name, policy_decision, operation_type,
	resource_type, boundary_level, summary, audit_decision, payload_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, runID, stepIndex, asString(step["tool_name"]), asString(step["policy_decision"]),
		asString(step["operation_type"]), asString(step["resource_type"]), asString(step["boundary_level"]),
		firstNonEmptyText(asString(step["user_visible_summary"]), asString(step["reason"])),
		auditDecisionFromStep(step), string(payload))
	return err
}

func insertSQLiteToolTrace(tx *sql.Tx, runID string, fallbackIndex int, trace logtrace.ToolTrace) error {
	payload, err := json.Marshal(sanitizeValue(trace))
	if err != nil {
		return err
	}
	stepID := trace.StepID
	if stepID == "" {
		stepID = fmt.Sprintf("trace-%03d", fallbackIndex)
	}
	_, err = tx.Exec(`
INSERT INTO tool_traces (
	run_id, step_id, step_index, tool_name, status, operation_type,
	resource_type, resource_path, boundary_level, allowed_by_policy,
	risk_hint, started_at, finished_at, output_summary, payload_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, runID, stepID, fallbackIndex, trace.ToolName, trace.Status, trace.OperationType,
		trace.ResourceType, trace.ResourcePath, trace.BoundaryLevel, boolToSQLiteInt(trace.AllowedByPolicy),
		trace.RiskHint, formatOptionalTime(trace.StartedAt), formatOptionalTime(trace.FinishedAt),
		trace.OutputSummary, string(payload))
	return err
}

func insertSQLiteAuditResult(tx *sql.Tx, resp agent.AgentRunResponse, updatedAt string) error {
	violations, err := json.Marshal(sanitizeValue(resp.AuditResult.Violations))
	if err != nil {
		return err
	}
	evidence, err := json.Marshal(sanitizeValue(resp.AuditResult.EvidenceChain))
	if err != nil {
		return err
	}
	graph, err := json.Marshal(sanitizeValue(riskGraphFromResponse(resp)))
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
INSERT INTO audit_results (
	run_id, decision, risk_score, method, message, violations_json,
	evidence_chain_json, risk_graph_json, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(run_id) DO UPDATE SET
	decision=excluded.decision,
	risk_score=excluded.risk_score,
	method=excluded.method,
	message=excluded.message,
	violations_json=excluded.violations_json,
	evidence_chain_json=excluded.evidence_chain_json,
	risk_graph_json=excluded.risk_graph_json,
	updated_at=excluded.updated_at
`, resp.RunID, resp.AuditResult.Decision, resp.AuditResult.RiskScore, resp.AuditResult.Method,
		resp.AuditResult.Message, string(violations), string(evidence), string(graph), updatedAt)
	return err
}

func (s *agentRunStore) deleteSQLiteRunLocked(runID string) error {
	if s.db == nil {
		return nil
	}
	_, err := s.db.Exec(`DELETE FROM agent_runs WHERE run_id = ?`, runID)
	return err
}

func (s *agentRunStore) importJSONRuns(dir string) {
	jsonStore := newAgentRunStoreWithOptions(dir, s.limit)
	if err := jsonStore.load(); err != nil {
		return
	}
	for _, runID := range jsonStore.order {
		if _, exists := s.runs[runID]; exists {
			continue
		}
		if resp, ok := jsonStore.Get(runID); ok {
			s.Save(resp)
		}
	}
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		parsed, _ := v.Int64()
		return int(parsed)
	default:
		return 0
	}
}

func auditDecisionFromStep(step map[string]any) string {
	if raw, ok := step["audit_report"].(map[string]any); ok {
		return asString(raw["decision"])
	}
	return ""
}

func boolToSQLiteInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
