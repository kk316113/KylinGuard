export type Decision = "allow" | "review" | "deny" | string;
export type InteractionType = "chat" | "agent_run" | "safe_refusal" | "clarify" | string;

export type UserMessage = {
  title?: string;
  answer?: string;
  status?: string;
  what_i_checked?: string[];
  key_findings?: string[];
  next_steps?: string[];
};

export type AgentStep = {
  step_index?: number;
  action_type?: string;
  tool_name?: string;
  tool_args?: Record<string, unknown>;
  reason?: string;
  user_visible_summary?: string;
  policy_decision?: Decision;
  observation?: {
    ok?: boolean;
    type?: string;
    summary?: string;
    deny_reason?: string;
    data_redacted?: boolean;
    data?: Record<string, unknown>;
    status?: string;
    result?: unknown;
    [key: string]: unknown;
  } | string | null;
  operation_type?: string;
  resource_type?: string;
  resource_path?: string;
  boundary_level?: string;
  allowed_by_policy?: boolean;
  policy_reason?: string;
};

export type ToolTrace = {
  step_id?: string;
  tool_name?: string;
  input?: unknown;
  output_summary?: string;
  status?: string;
  started_at?: string;
  finished_at?: string;
  risk_hint?: string;
  operation_type?: string;
  resource_type?: string;
  resource_path?: string;
  permission_scope?: string;
  boundary_level?: string;
  requires_privilege?: boolean;
  allowed_by_policy?: boolean;
  policy_reason?: string;
  risk_level?: string;
  execution_context?: {
    executor?: string;
    profile?: string;
    command_name?: string;
    shell_used?: boolean;
    sudo_used?: boolean;
    effective_user?: string;
    [key: string]: unknown;
  };
};

export type EvidenceItem = {
  step_id?: string;
  tool_name?: string;
  summary?: string;
  message?: string;
  [key: string]: unknown;
};

export type RiskGraphNode = {
  id?: string;
  type?: string;
  label?: string;
  risk_level?: string;
  step_index?: number;
  [key: string]: unknown;
};

export type RiskGraphEdge = {
  source?: string;
  target?: string;
  type?: string;
  label?: string;
  [key: string]: unknown;
};

export type RiskGraph = {
  nodes?: RiskGraphNode[];
  edges?: RiskGraphEdge[];
  risk_hotspots?: Array<Record<string, unknown>>;
  boundary_crossings?: Array<Record<string, unknown>>;
  decision_path?: Array<Record<string, unknown>>;
  [key: string]: unknown;
} | null;

export type AuditResult = {
  decision?: Decision;
  risk_score?: number;
  violations?: Array<{
    type?: string;
    severity?: string;
    message?: string;
    step_id?: string;
  }>;
  evidence_chain?: EvidenceItem[];
  risk_graph?: RiskGraph;
  method?: string;
  message?: string;
  audit_metadata?: Record<string, unknown>;
};

export type SecurityReport = {
  title?: string;
  decision?: Decision;
  summary?: string;
  executive_summary?: string;
  evidence_chain?: EvidenceItem[];
  risk_explanation?: Array<Record<string, unknown> | string>;
  recommendations?: string[];
  audit_metadata?: Record<string, unknown>;
  risk_graph?: RiskGraph;
  [key: string]: unknown;
};

export type AgentRun = {
  run_id?: string;
  task_id?: string;
  task: string;
  scene_type?: string;
  scene_summary?: string;
  run_status?: string;
  created_at?: string;
  interaction_type?: InteractionType;
  router_source?: string;
  router_confidence?: string;
  needs_tool_execution?: boolean;
  router_reason?: string;
  decision: Decision;
  summary: string;
  agent_mode?: string;
  task_understanding?: Record<string, unknown>;
  agent_steps?: AgentStep[];
  tool_trace: ToolTrace[];
  audit_result?: AuditResult;
  security_report?: SecurityReport | null;
  risk_graph?: RiskGraph;
  final_answer?: string;
  user_message?: UserMessage | null;
};

export type ConversationMessage = {
  id: string;
  role: "user" | "assistant";
  content: string;
  createdAt: string;
  run?: AgentRun;
};
