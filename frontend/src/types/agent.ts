export type RuntimeMode = 'stable' | 'eino'

export type Decision = 'allow' | 'review' | 'deny' | string

export interface HealthResponse {
  status: string
  service: string
  version?: string
}

export interface AgentRunRequest {
  task: string
}

export interface AgentRunResponse {
  task_id?: string
  task: string
  scene_type?: SceneType
  scene_summary?: string
  run_status?: RunStatus
  created_at?: string
  interaction_type?: InteractionType
  router_source?: string
  router_confidence?: string
  needs_tool_execution?: boolean
  router_reason?: string
  decision: Decision
  summary: string
  plan?: Plan | null
  tool_trace: ToolTraceItem[]
  diagnosis?: Diagnosis | null
  audit_result: AuditResult
  risk_graph?: RiskGraph | null
  security_report?: SecurityReport | null
  reasoning_trace?: ReasoningTrace | null
  agent_mode?: string
  task_understanding?: {
    user_goal?: string
    intent_type?: string
    risk_level?: string
  }
  agent_steps?: AgentStep[]
  final_answer?: string
  chat_model?: string
  user_message?: UserMessage
}

export interface UserMessage {
  title: string
  answer: string
  status: RunStatus
  what_i_checked: string[]
  key_findings: string[]
  next_steps: string[]
}

export type SceneType = 'diagnosis' | 'security_check' | 'service_recovery' | 'system_health' | 'compliance_review' | 'unknown' | string
export type RunStatus = 'completed' | 'blocked' | 'failed' | 'partial' | string
export type InteractionType = 'chat' | 'agent_run' | 'safe_refusal' | 'clarify' | string

export interface RuntimeStatusResponse {
  ok: boolean
  runtime: {
    agent_mode: string
    current_mode: string
    llm_enabled: boolean
    remote_llm_used: boolean
    chat_model: string
    provider: string
    endpoint_kind: string
    model: string
  }
  services: {
    go_agent: RuntimeServiceStatus
    audit_core: RuntimeServiceStatus
    frontend: RuntimeServiceStatus
  }
  security_layers: Record<string, string>
  secret_safety: {
    api_key_present: boolean
    api_key_display: string
  }
  updated_at?: string
}

export interface RuntimeServiceStatus {
  status: string
  port?: number
  url?: string
}

export interface AgentCapabilitiesResponse {
  available_tools: AgentCapabilityTool[]
  tool_policy: {
    enabled: boolean
    default_mode: string
    dangerous_actions_blocked: boolean
    unknown_tools_default_denied?: boolean
    raw_shell_execution?: string
  }
  agent_loop: {
    next_action_schema: string[]
    max_steps: number
  }
}

export interface AgentCapabilityTool {
  tool_name: string
  display_name: string
  description: string
  operation_type: string
  resource_type: string
  boundary_level: string
  requires_privilege: boolean
  read_only: boolean
  policy_controlled: boolean
  traceshield_mapped: boolean
  enabled: boolean
}

export interface AcceptanceSummaryResponse {
  stages: Array<{
    name: string
    title: string
    status: string
    evidence?: Record<string, unknown>
  }>
  commands: string[]
  notes: string[]
}

export interface AgentStep {
  step_index?: number
  action_type?: string
  tool_name?: string
  tool_args?: Record<string, unknown>
  reason?: string
  user_visible_summary?: string
  policy_decision?: string
  observation?: Record<string, unknown>
  operation_type?: string
  resource_type?: string
  resource_path?: string
  boundary_level?: string
  allowed_by_policy?: boolean
  policy_reason?: string
  audit_report?: AuditReport
}

// AuditReport is the per-tool-call audit conclusion produced by the backend
// StepAuditor (one audit_report per tool_call). All reports in a run aggregate
// into the top-level RiskGraph.
export interface AuditReport {
  step_id?: string
  step_index?: number
  tool_name?: string
  decision?: Decision
  risk_score?: number
  violations?: string[]
  evidence?: string[]
  method?: string
  message?: string
}

export interface ReasoningTrace {
  trace_id: string
  runtime: string
  task_hash: string
  task_summary: string
  started_at: string
  ended_at: string
  duration_ms: number
  spans: ReasoningSpan[]
}

export interface ReasoningSpan {
  span_id: string
  parent_span_id?: string
  type: string
  name: string
  status: string
  started_at: string
  ended_at: string
  duration_ms: number
  attributes?: Record<string, unknown>
  events?: ReasoningEvent[]
  _open?: boolean
}

export interface ReasoningEvent {
  name: string
  timestamp: string
  attributes?: Record<string, unknown>
}

export interface Plan {
  task: string
  scenario: string
  summary: string
  steps: PlanStep[]
}

export interface PlanStep {
  step_id: string
  tool_name: string
  input: Record<string, unknown>
  reason: string
  tool_category?: string
  risk_level?: string
  permission_scope?: string
}

export interface ToolTraceItem {
  step_id: string
  tool_name: string
  input: Record<string, unknown> | unknown
  output_summary: string
  status: string
  started_at?: string
  finished_at?: string
  risk_hint?: string
  operation_type: string
  resource_type: string
  resource_path?: string
  permission_scope?: string
  boundary_level: string
  tool_semantic?: string
  requires_privilege?: boolean
  allowed_by_policy?: boolean
  policy_reason?: string
  execution_context?: {
    executor?: string
    profile?: string
    command_name?: string
    shell_used?: boolean
    sudo_used?: boolean
    allowed_by_exec_policy?: boolean
    policy_reason?: string
  }
}

export interface Diagnosis {
  scenario: string
  risk_level: string
  findings: string[]
  recommendations: string[]
  details?: Record<string, unknown>
}

export interface AuditResult {
  decision: Decision
  risk_score?: number
  violations: AuditViolation[]
  evidence_chain: AuditEvidenceItem[]
  risk_graph?: RiskGraph | null
  method: string
  message: string
  audit_metadata?: Record<string, unknown> & {
    chat_model?: string
  }
}

export interface AuditViolation {
  type: string
  severity: string
  message: string
  step_id?: string | number | null
}

export interface AuditEvidenceItem {
  step_id?: string | number | null
  tool_name?: string
  resource?: string
  reason: string
}

export interface RiskGraph {
  nodes: Array<Record<string, unknown>>
  edges: Array<Record<string, unknown>>
}

export interface SecurityReport {
  title: string
  scenario: string
  overall_decision: Decision
  risk_level: string
  summary: string
  evidence_chain: EvidenceItem[]
  risk_explanation: RiskExplanationItem[]
  recommendations: RecommendationItem[]
  sensitive_resources: SensitiveResourceItem[]
  audit_metadata?: Record<string, unknown> & {
    llm_enabled?: boolean
    chat_model?: string
    chat_model_adapter?: string
    remote_llm_used?: boolean
    fallback_used?: boolean
    fallback_reason?: string
  }
}

export interface EvidenceItem {
  evidence_id: string
  step_id?: string
  tool_name: string
  operation_type: string
  resource_type: string
  resource_path?: string
  boundary_level: string
  status: string
  summary: string
  why_relevant: string
  audit_meaning: string
}

export interface RiskExplanationItem {
  reason_id: string
  severity: string
  category: string
  description: string
  evidence_ids?: string[]
}

export interface SensitiveResourceItem {
  resource_type: string
  resource_path?: string
  boundary_level: string
  access_reason: string
  allowed_by_policy: boolean
}

export interface RecommendationItem {
  recommendation_id: string
  priority: string
  action: string
  rationale: string
  is_destructive: boolean
}
