package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"kylin-guard-agent/agent-go/internal/agent"
	"kylin-guard-agent/agent-go/internal/auditclient"
)

const (
	defaultStoredAgentRuns = 200
	runStoreIndexFile      = "index.json"
	runStoreBackendMemory  = "memory"
	runStoreBackendJSON    = "json"
	runStoreBackendSQLite  = "sqlite"
)

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)sk-[a-z0-9_\-]{8,}`),
	regexp.MustCompile(`(?i)(api[_-]?key|token|secret|password)(\s*[:=]\s*)[^\s"',;]+`),
	regexp.MustCompile(`(?i)(OPENAI_COMPATIBLE_API_KEY|OPENAI_API_KEY|DEEPSEEK_API_KEY|EINO_LLM_API_KEY)(\s*=\s*)[^\s"',;]+`),
}

type agentRunStore struct {
	mu      sync.RWMutex
	runs    map[string]agent.AgentRunResponse
	order   []string
	latest  string
	dir     string
	backend string
	dbPath  string
	db      *sql.DB
	limit   int
}

type agentRunSummary struct {
	RunID          string `json:"run_id"`
	TaskID         string `json:"task_id,omitempty"`
	Task           string `json:"task"`
	SceneType      string `json:"scene_type,omitempty"`
	RunStatus      string `json:"run_status,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
	Decision       string `json:"decision,omitempty"`
	AgentMode      string `json:"agent_mode,omitempty"`
	ChatModel      string `json:"chat_model,omitempty"`
	ToolTraceCount int    `json:"tool_trace_count"`
	AgentStepCount int    `json:"agent_step_count"`
}

type agentRunListResponse struct {
	Runs       []agentRunSummary `json:"runs"`
	Count      int               `json:"count"`
	Limit      int               `json:"limit"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

type auditReportsResponse struct {
	RunID        string             `json:"run_id"`
	AuditReports []map[string]any   `json:"audit_reports"`
	AuditResult  auditclient.Result `json:"audit_result"`
}

type riskGraphResponse struct {
	RunID     string                 `json:"run_id"`
	RiskGraph *auditclient.RiskGraph `json:"risk_graph"`
}

type runReportResponse struct {
	RunID       string         `json:"run_id"`
	TaskID      string         `json:"task_id"`
	Task        string         `json:"task"`
	SceneType   string         `json:"scene_type"`
	RunStatus   string         `json:"run_status"`
	CreatedAt   string         `json:"created_at"`
	Decision    string         `json:"decision"`
	Summary     string         `json:"summary"`
	FinalAnswer string         `json:"final_answer"`
	Counts      map[string]int `json:"counts"`
}

type riskGraphArtifact struct {
	SchemaVersion string                 `json:"schema_version"`
	Run           agentRunSummary        `json:"run"`
	Nodes         []map[string]any       `json:"nodes"`
	Edges         []map[string]any       `json:"edges"`
	Hotspots      []map[string]any       `json:"hotspots"`
	DecisionPath  []map[string]any       `json:"decision_path"`
	EvidenceIndex []map[string]any       `json:"evidence_index"`
	ToolTraceRefs []map[string]any       `json:"tool_trace_refs"`
	AuditSummary  map[string]any         `json:"audit_summary"`
	Metadata      map[string]interface{} `json:"metadata"`
}

func newAgentRunStore() *agentRunStore {
	return newAgentRunStoreWithOptions("", defaultStoredAgentRuns)
}

func newAgentRunStoreFromConfig(backend, dir, dbPath string, limit int) *agentRunStore {
	backend = normalizeRunStoreBackend(backend)
	if backend == runStoreBackendSQLite {
		store := newAgentRunSQLiteStoreWithOptions(dbPath, limit)
		if err := store.load(); err != nil {
			log.Printf("sqlite agent run store load failed, falling back to JSON store: %v", err)
			jsonStore := newAgentRunStoreWithOptions(dir, limit)
			if loadErr := jsonStore.load(); loadErr != nil {
				log.Printf("json agent run store fallback load failed, continuing with empty history: %v", loadErr)
			}
			return jsonStore
		}
		if dir != "" {
			store.importJSONRuns(dir)
		}
		return store
	}
	store := newAgentRunStoreWithOptions(dir, limit)
	if err := store.load(); err != nil {
		log.Printf("agent run store load failed, continuing with empty history: %v", err)
	}
	return store
}

func newAgentRunStoreWithOptions(dir string, limit int) *agentRunStore {
	if limit <= 0 {
		limit = defaultStoredAgentRuns
	}
	return &agentRunStore{
		runs:    make(map[string]agent.AgentRunResponse),
		dir:     strings.TrimSpace(dir),
		backend: backendForDir(dir),
		limit:   limit,
	}
}

func newAgentRunSQLiteStoreWithOptions(dbPath string, limit int) *agentRunStore {
	if limit <= 0 {
		limit = defaultStoredAgentRuns
	}
	return &agentRunStore{
		runs:    make(map[string]agent.AgentRunResponse),
		backend: runStoreBackendSQLite,
		dbPath:  strings.TrimSpace(dbPath),
		limit:   limit,
	}
}

func normalizeRunStoreBackend(backend string) string {
	switch strings.ToLower(strings.TrimSpace(backend)) {
	case "", runStoreBackendSQLite:
		return runStoreBackendSQLite
	case runStoreBackendJSON, "file", "files":
		return runStoreBackendJSON
	case runStoreBackendMemory:
		return runStoreBackendMemory
	default:
		log.Printf("unknown KYLIN_GUARD_RUN_STORE_BACKEND=%q, using sqlite", backend)
		return runStoreBackendSQLite
	}
}

func backendForDir(dir string) string {
	if strings.TrimSpace(dir) == "" {
		return runStoreBackendMemory
	}
	return runStoreBackendJSON
}

func (s *agentRunStore) Save(resp agent.AgentRunResponse) agent.AgentRunResponse {
	if s == nil {
		return resp
	}
	resp = sanitizeRunResponse(resp)
	if resp.RunID == "" || resp.TaskID == "" || resp.CreatedAt == "" {
		status := resp.RunStatus
		if status == "" {
			status = statusFromDecision(resp.Decision)
		}
		agent.AttachScenarioWorkspaceMetadata(&resp, resp.Task, status)
	}
	if resp.RunID == "" {
		resp.RunID = resp.TaskID
	}
	if resp.TaskID == "" {
		resp.TaskID = resp.RunID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.runs[resp.RunID]; !exists {
		s.order = append(s.order, resp.RunID)
	}
	s.runs[resp.RunID] = resp
	s.latest = resp.RunID
	s.enforceLimitLocked()
	if err := s.persistRunLocked(resp); err != nil {
		log.Printf("agent run store persist failed for %s: %v", resp.RunID, err)
	}
	if err := s.persistIndexLocked(); err != nil {
		log.Printf("agent run store index persist failed: %v", err)
	}
	return resp
}

func (s *agentRunStore) Get(runID string) (agent.AgentRunResponse, bool) {
	if s == nil {
		return agent.AgentRunResponse{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if runID == "latest" {
		runID = s.latest
	}
	resp, ok := s.runs[runID]
	return resp, ok
}

func (s *agentRunStore) List(limit int, cursor string) agentRunListResponse {
	if s == nil {
		return agentRunListResponse{Runs: []agentRunSummary{}, Limit: limit}
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := 0
	if cursor != "" {
		if parsed, err := strconv.Atoi(cursor); err == nil && parsed > 0 {
			offset = parsed
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	total := len(s.order)
	summaries := make([]agentRunSummary, 0, limit)
	for i := total - 1 - offset; i >= 0 && len(summaries) < limit; i-- {
		if resp, ok := s.runs[s.order[i]]; ok {
			summaries = append(summaries, summarizeRun(resp))
		}
	}
	next := ""
	if offset+len(summaries) < total {
		next = strconv.Itoa(offset + len(summaries))
	}
	return agentRunListResponse{
		Runs:       summaries,
		Count:      len(summaries),
		Limit:      limit,
		NextCursor: next,
	}
}

func (s *agentRunStore) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

func (s *agentRunStore) load() error {
	if s.backend == runStoreBackendSQLite {
		return s.loadSQLite()
	}
	if s == nil || s.dir == "" {
		return nil
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	indexPath := filepath.Join(s.dir, runStoreIndexFile)
	data, err := os.ReadFile(indexPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		var order []string
		if err := json.Unmarshal(data, &order); err != nil {
			log.Printf("agent run store index is invalid, rebuilding from run files: %v", err)
		} else {
			for _, runID := range order {
				s.loadRunFile(runID)
			}
			s.pruneMissingLocked()
			if len(s.order) > 0 {
				s.latest = s.order[len(s.order)-1]
			}
			return nil
		}
	}
	return s.rebuildFromFiles()
}

func (s *agentRunStore) loadRunFile(runID string) {
	resp, err := readRunFile(filepath.Join(s.dir, runID+".json"))
	if err != nil {
		log.Printf("agent run store skipped %s: %v", runID, err)
		return
	}
	if resp.RunID == "" {
		resp.RunID = runID
	}
	s.runs[resp.RunID] = sanitizeRunResponse(resp)
	s.order = append(s.order, resp.RunID)
}

func (s *agentRunStore) rebuildFromFiles() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}
	type loadedRun struct {
		run       agent.AgentRunResponse
		sortValue string
	}
	loaded := []loadedRun{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || entry.Name() == runStoreIndexFile {
			continue
		}
		resp, err := readRunFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			log.Printf("agent run store skipped %s: %v", entry.Name(), err)
			continue
		}
		sortValue := resp.CreatedAt
		if sortValue == "" {
			sortValue = strings.TrimSuffix(entry.Name(), ".json")
		}
		loaded = append(loaded, loadedRun{run: sanitizeRunResponse(resp), sortValue: sortValue})
	}
	sort.SliceStable(loaded, func(i, j int) bool { return loaded[i].sortValue < loaded[j].sortValue })
	for _, item := range loaded {
		if item.run.RunID == "" {
			continue
		}
		s.runs[item.run.RunID] = item.run
		s.order = append(s.order, item.run.RunID)
	}
	if len(s.order) > 0 {
		s.latest = s.order[len(s.order)-1]
	}
	s.enforceLimitLocked()
	return s.persistIndexLocked()
}

func readRunFile(path string) (agent.AgentRunResponse, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return agent.AgentRunResponse{}, err
	}
	var resp agent.AgentRunResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return agent.AgentRunResponse{}, err
	}
	return resp, nil
}

func (s *agentRunStore) enforceLimitLocked() {
	if s.limit <= 0 {
		s.limit = defaultStoredAgentRuns
	}
	for len(s.order) > s.limit {
		oldest := s.order[0]
		s.order = s.order[1:]
		delete(s.runs, oldest)
		if s.backend == runStoreBackendSQLite {
			if err := s.deleteSQLiteRunLocked(oldest); err != nil {
				log.Printf("agent run sqlite store failed to remove old run %s: %v", oldest, err)
			}
		} else if s.dir != "" {
			if err := os.Remove(filepath.Join(s.dir, oldest+".json")); err != nil && !os.IsNotExist(err) {
				log.Printf("agent run store failed to remove old run %s: %v", oldest, err)
			}
		}
	}
}

func (s *agentRunStore) pruneMissingLocked() {
	filtered := s.order[:0]
	seen := map[string]bool{}
	for _, runID := range s.order {
		if _, ok := s.runs[runID]; ok && !seen[runID] {
			filtered = append(filtered, runID)
			seen[runID] = true
		}
	}
	s.order = filtered
}

func (s *agentRunStore) persistRunLocked(resp agent.AgentRunResponse) error {
	if s.backend == runStoreBackendSQLite {
		return s.persistSQLiteRunLocked(resp)
	}
	if s.dir == "" {
		return nil
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	return writeJSONFileAtomic(filepath.Join(s.dir, resp.RunID+".json"), resp)
}

func (s *agentRunStore) persistIndexLocked() error {
	if s.backend == runStoreBackendSQLite || s.backend == runStoreBackendMemory {
		return nil
	}
	if s.dir == "" {
		return nil
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	return writeJSONFileAtomic(filepath.Join(s.dir, runStoreIndexFile), s.order)
}

func writeJSONFileAtomic(path string, payload any) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func statusFromDecision(decision string) string {
	switch decision {
	case "deny":
		return agent.RunStatusBlocked
	default:
		return agent.RunStatusCompleted
	}
}

func agentRunsHandler(store *agentRunStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}

		path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/agent/runs/"), "/")
		if path == "" {
			agentRunListHandler(store).ServeHTTP(w, r)
			return
		}
		parts := strings.Split(path, "/")
		runID := parts[0]
		resp, ok := store.Get(runID)
		if !ok {
			writeError(w, http.StatusNotFound, "agent run not found")
			return
		}
		if len(parts) == 1 {
			writeJSON(w, http.StatusOK, resp)
			return
		}

		switch {
		case len(parts) == 2 && parts[1] == "audit-reports":
			writeJSON(w, http.StatusOK, buildAuditReportsResponse(resp))
		case len(parts) == 2 && parts[1] == "risk-graph":
			writeJSON(w, http.StatusOK, riskGraphResponse{
				RunID:     resp.RunID,
				RiskGraph: riskGraphFromResponse(resp),
			})
		case len(parts) == 3 && parts[1] == "risk-graph" && parts[2] == "artifact":
			writeJSON(w, http.StatusOK, buildRiskGraphArtifact(resp))
		case len(parts) == 2 && parts[1] == "report":
			writeJSON(w, http.StatusOK, buildRunReportResponse(resp))
		case len(parts) == 2 && parts[1] == "report.md":
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-report.md"`, safeFilename(resp.RunID)))
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte(buildRunMarkdownReport(resp))); err != nil {
				log.Printf("write markdown report failed: %v", err)
			}
		default:
			writeError(w, http.StatusNotFound, "agent run resource not found")
		}
	}
}

func agentRunListHandler(store *agentRunStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		writeJSON(w, http.StatusOK, store.List(limit, r.URL.Query().Get("cursor")))
	}
}

func buildAuditReportsResponse(resp agent.AgentRunResponse) auditReportsResponse {
	reports := make([]map[string]any, 0, len(resp.AgentSteps)+1)
	for index, step := range resp.AgentSteps {
		if raw, ok := step["audit_report"]; ok {
			if reportMap, ok := raw.(map[string]any); ok {
				reports = append(reports, sanitizeMap(reportMap))
				continue
			}
		}
		if toolName, ok := step["tool_name"].(string); ok && toolName != "" {
			reports = append(reports, map[string]any{
				"audit_id":   "audit-step-" + strconv.Itoa(index+1),
				"run_id":     resp.RunID,
				"step_index": index + 1,
				"tool_name":  toolName,
				"decision":   sanitizeValue(step["policy_decision"]),
				"message":    sanitizeValue(step["user_visible_summary"]),
			})
		}
	}
	if len(reports) == 0 && resp.AuditResult.Method != "" {
		reports = append(reports, map[string]any{
			"audit_id":       "audit-aggregate",
			"run_id":         resp.RunID,
			"step_index":     0,
			"tool_name":      "agent_run",
			"decision":       resp.AuditResult.Decision,
			"risk_score":     resp.AuditResult.RiskScore,
			"violations":     resp.AuditResult.Violations,
			"evidence_chain": resp.AuditResult.EvidenceChain,
			"method":         resp.AuditResult.Method,
			"message":        resp.AuditResult.Message,
		})
	}
	return auditReportsResponse{
		RunID:        resp.RunID,
		AuditReports: reports,
		AuditResult:  resp.AuditResult,
	}
}

func riskGraphFromResponse(resp agent.AgentRunResponse) *auditclient.RiskGraph {
	if resp.RiskGraph != nil {
		return resp.RiskGraph
	}
	if resp.AuditResult.RiskGraph != nil {
		return resp.AuditResult.RiskGraph
	}
	return &auditclient.RiskGraph{Nodes: []map[string]any{}, Edges: []map[string]any{}}
}

func buildRunReportResponse(resp agent.AgentRunResponse) runReportResponse {
	return runReportResponse{
		RunID:       resp.RunID,
		TaskID:      resp.TaskID,
		Task:        resp.Task,
		SceneType:   resp.SceneType,
		RunStatus:   resp.RunStatus,
		CreatedAt:   resp.CreatedAt,
		Decision:    resp.Decision,
		Summary:     resp.Summary,
		FinalAnswer: resp.FinalAnswer,
		Counts: map[string]int{
			"agent_steps": len(resp.AgentSteps),
			"tool_trace":  len(resp.ToolTrace),
			"evidence":    len(resp.AuditResult.EvidenceChain),
			"violations":  len(resp.AuditResult.Violations),
		},
	}
}

func buildRunMarkdownReport(resp agent.AgentRunResponse) string {
	resp = sanitizeRunResponse(resp)
	graph := riskGraphFromResponse(resp)
	var b strings.Builder
	fmt.Fprintf(&b, "# KylinGuard 会话报告\n\n")
	fmt.Fprintf(&b, "- 运行编号：`%s`\n", markdownInline(resp.RunID))
	fmt.Fprintf(&b, "- 创建时间：%s\n", markdownInline(resp.CreatedAt))
	fmt.Fprintf(&b, "- 运行状态：%s\n", markdownInline(resp.RunStatus))
	fmt.Fprintf(&b, "- 安全结论：%s\n", markdownInline(resp.Decision))
	fmt.Fprintf(&b, "- Agent 模式：%s\n", markdownInline(resp.AgentMode))
	fmt.Fprintf(&b, "- 模型：%s\n\n", markdownInline(chatModelFromRun(resp)))

	fmt.Fprintf(&b, "## 用户任务\n\n%s\n\n", markdownBlock(resp.Task))
	fmt.Fprintf(&b, "## 最终回答\n\n%s\n\n", markdownBlock(firstNonEmptyText(resp.FinalAnswer, resp.Summary)))

	fmt.Fprintf(&b, "## 执行摘要\n\n")
	fmt.Fprintf(&b, "- Agent steps：%d\n", len(resp.AgentSteps))
	fmt.Fprintf(&b, "- Tool trace：%d\n", len(resp.ToolTrace))
	fmt.Fprintf(&b, "- 审计方法：%s\n", markdownInline(resp.AuditResult.Method))
	fmt.Fprintf(&b, "- 审计消息：%s\n\n", markdownInline(resp.AuditResult.Message))

	if len(resp.AgentSteps) > 0 {
		fmt.Fprintf(&b, "## Agent Steps\n\n")
		for i, step := range resp.AgentSteps {
			fmt.Fprintf(&b, "%d. `%s` - %s\n", i+1, markdownInline(asString(step["tool_name"])), markdownInline(asString(step["user_visible_summary"])))
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(resp.ToolTrace) > 0 {
		fmt.Fprintf(&b, "## 工具证据\n\n")
		for i, trace := range resp.ToolTrace {
			fmt.Fprintf(&b, "%d. `%s`：%s（状态：%s，边界：%s）\n", i+1, markdownInline(trace.ToolName), markdownInline(trace.OutputSummary), markdownInline(trace.Status), markdownInline(trace.BoundaryLevel))
		}
		fmt.Fprintf(&b, "\n")
	}

	fmt.Fprintf(&b, "## 风险图摘要\n\n")
	fmt.Fprintf(&b, "- 节点数：%d\n", len(graph.Nodes))
	fmt.Fprintf(&b, "- 边数：%d\n", len(graph.Edges))
	fmt.Fprintf(&b, "- 风险热点：%d\n\n", len(riskHotspots(resp, graph)))
	fmt.Fprintf(&b, "> 本报告由 KylinGuard 根据真实执行证据生成；敏感字段已脱敏。\n")
	return b.String()
}

func buildRiskGraphArtifact(resp agent.AgentRunResponse) riskGraphArtifact {
	resp = sanitizeRunResponse(resp)
	graph := riskGraphFromResponse(resp)
	return riskGraphArtifact{
		SchemaVersion: "kylin-guard-risk-graph-artifact-v1",
		Run:           summarizeRun(resp),
		Nodes:         sanitizeMapSlice(graph.Nodes),
		Edges:         normalizeRiskEdges(graph.Edges),
		Hotspots:      riskHotspots(resp, graph),
		DecisionPath:  decisionPath(resp),
		EvidenceIndex: evidenceIndex(resp),
		ToolTraceRefs: toolTraceRefs(resp),
		AuditSummary: map[string]any{
			"decision":         resp.AuditResult.Decision,
			"risk_score":       resp.AuditResult.RiskScore,
			"method":           resp.AuditResult.Method,
			"message":          resp.AuditResult.Message,
			"violations_count": len(resp.AuditResult.Violations),
			"evidence_count":   len(resp.AuditResult.EvidenceChain),
			"fallback_marked":  strings.Contains(strings.ToLower(resp.AuditResult.Method), "fallback"),
		},
		Metadata: map[string]interface{}{
			"generated_at": time.Now().UTC().Format(time.RFC3339),
			"source":       "kylin-guard-agent-run",
			"redacted":     true,
		},
	}
}

func summarizeRun(resp agent.AgentRunResponse) agentRunSummary {
	return agentRunSummary{
		RunID:          resp.RunID,
		TaskID:         resp.TaskID,
		Task:           resp.Task,
		SceneType:      resp.SceneType,
		RunStatus:      resp.RunStatus,
		CreatedAt:      resp.CreatedAt,
		Decision:       resp.Decision,
		AgentMode:      resp.AgentMode,
		ChatModel:      chatModelFromRun(resp),
		ToolTraceCount: len(resp.ToolTrace),
		AgentStepCount: len(resp.AgentSteps),
	}
}

func chatModelFromRun(resp agent.AgentRunResponse) string {
	if resp.SecurityReport != nil && resp.SecurityReport.AuditMetadata != nil {
		if model, ok := resp.SecurityReport.AuditMetadata["chat_model"].(string); ok {
			return model
		}
	}
	return ""
}

func riskHotspots(resp agent.AgentRunResponse, graph *auditclient.RiskGraph) []map[string]any {
	hotspots := []map[string]any{}
	for _, node := range graph.Nodes {
		risk := strings.ToLower(asString(node["risk_level"]))
		decision := strings.ToLower(asString(node["decision"]))
		if risk == "high" || risk == "critical" || decision == "deny" {
			hotspots = append(hotspots, map[string]any{
				"type":        "risk_graph_node",
				"node_id":     firstNonEmptyText(asString(node["id"]), asString(node["step_id"])),
				"summary":     firstNonEmptyText(asString(node["label"]), asString(node["tool_name"]), "风险节点"),
				"risk_level":  firstNonEmptyText(risk, "unknown"),
				"decision":    decision,
				"evidence_id": asString(node["step_id"]),
			})
		}
	}
	for _, violation := range resp.AuditResult.Violations {
		hotspots = append(hotspots, map[string]any{
			"type":        "audit_violation",
			"summary":     violation.Message,
			"risk_level":  violation.Severity,
			"decision":    resp.AuditResult.Decision,
			"evidence_id": violation.StepID,
		})
	}
	return sanitizeMapSlice(hotspots)
}

func decisionPath(resp agent.AgentRunResponse) []map[string]any {
	path := make([]map[string]any, 0, len(resp.AgentSteps)+1)
	for i, step := range resp.AgentSteps {
		path = append(path, map[string]any{
			"index":           i + 1,
			"step_index":      step["step_index"],
			"tool_name":       step["tool_name"],
			"policy_decision": step["policy_decision"],
			"summary":         step["user_visible_summary"],
		})
	}
	path = append(path, map[string]any{
		"index":    len(path) + 1,
		"type":     "final_decision",
		"decision": resp.Decision,
		"summary":  firstNonEmptyText(resp.FinalAnswer, resp.Summary),
	})
	return sanitizeMapSlice(path)
}

func evidenceIndex(resp agent.AgentRunResponse) []map[string]any {
	items := make([]map[string]any, 0, len(resp.AuditResult.EvidenceChain))
	for i, evidence := range resp.AuditResult.EvidenceChain {
		items = append(items, map[string]any{
			"id":        fmt.Sprintf("evidence-%d", i+1),
			"step_id":   evidence.StepID,
			"tool_name": evidence.ToolName,
			"resource":  evidence.Resource,
			"reason":    evidence.Reason,
		})
	}
	if len(items) == 0 {
		for i, trace := range resp.ToolTrace {
			items = append(items, map[string]any{
				"id":        fmt.Sprintf("trace-%d", i+1),
				"step_id":   trace.StepID,
				"tool_name": trace.ToolName,
				"summary":   trace.OutputSummary,
				"status":    trace.Status,
			})
		}
	}
	return sanitizeMapSlice(items)
}

func toolTraceRefs(resp agent.AgentRunResponse) []map[string]any {
	refs := make([]map[string]any, 0, len(resp.ToolTrace))
	for i, trace := range resp.ToolTrace {
		refs = append(refs, map[string]any{
			"index":             i + 1,
			"step_id":           trace.StepID,
			"tool_name":         trace.ToolName,
			"status":            trace.Status,
			"operation_type":    trace.OperationType,
			"resource_type":     trace.ResourceType,
			"resource_path":     trace.ResourcePath,
			"boundary_level":    trace.BoundaryLevel,
			"allowed_by_policy": trace.AllowedByPolicy,
			"output_summary":    trace.OutputSummary,
		})
	}
	return sanitizeMapSlice(refs)
}

func normalizeRiskEdges(edges []map[string]any) []map[string]any {
	normalized := make([]map[string]any, 0, len(edges))
	for i, edge := range edges {
		item := sanitizeMap(edge)
		if item["source"] == nil && item["from"] != nil {
			item["source"] = item["from"]
		}
		if item["target"] == nil && item["to"] != nil {
			item["target"] = item["to"]
		}
		if item["id"] == nil {
			item["id"] = fmt.Sprintf("edge-%d", i+1)
		}
		normalized = append(normalized, item)
	}
	return normalized
}

func sanitizeRunResponse(resp agent.AgentRunResponse) agent.AgentRunResponse {
	data, err := json.Marshal(resp)
	if err != nil {
		return resp
	}
	var generic any
	if err := json.Unmarshal(data, &generic); err != nil {
		return resp
	}
	generic = sanitizeValue(generic)
	data, err = json.Marshal(generic)
	if err != nil {
		return resp
	}
	var sanitized agent.AgentRunResponse
	if err := json.Unmarshal(data, &sanitized); err != nil {
		return resp
	}
	return sanitized
}

func sanitizeValue(value any) any {
	switch typed := value.(type) {
	case string:
		return redactString(typed)
	case map[string]any:
		return sanitizeMap(typed)
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, sanitizeValue(item))
		}
		return out
	default:
		return value
	}
}

func sanitizeMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for key, value := range input {
		if isSensitiveKey(key) {
			out[key] = "[REDACTED]"
			continue
		}
		out[key] = sanitizeValue(value)
	}
	return out
}

func sanitizeMapSlice(input []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(input))
	for _, item := range input {
		out = append(out, sanitizeMap(item))
	}
	return out
}

func redactString(value string) string {
	redacted := value
	for _, pattern := range secretPatterns {
		redacted = pattern.ReplaceAllStringFunc(redacted, func(match string) string {
			if strings.Contains(match, "=") {
				parts := strings.SplitN(match, "=", 2)
				return parts[0] + "=[REDACTED]"
			}
			if strings.Contains(match, ":") {
				parts := strings.SplitN(match, ":", 2)
				return parts[0] + ":[REDACTED]"
			}
			return "[REDACTED]"
		})
	}
	return redacted
}

func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", "_"), " ", "_"))
	return strings.Contains(normalized, "api_key") ||
		strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "password")
}

func firstNonEmptyText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func markdownBlock(value string) string {
	value = strings.TrimSpace(redactString(value))
	if value == "" {
		return "无"
	}
	return strings.ReplaceAll(value, "\r\n", "\n")
}

func markdownInline(value string) string {
	value = strings.TrimSpace(redactString(value))
	if value == "" {
		return "无"
	}
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "`", "'")
	return html.EscapeString(value)
}

func safeFilename(value string) string {
	if value == "" {
		return "kylin-guard-run"
	}
	builder := strings.Builder{}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			builder.WriteRune(r)
		}
	}
	if builder.Len() == 0 {
		return "kylin-guard-run"
	}
	return builder.String()
}

func asString(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprint(typed)
	}
}
