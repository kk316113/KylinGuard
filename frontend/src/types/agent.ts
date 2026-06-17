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
  task: string
  decision: Decision
  summary: string
  plan?: Plan | null
  tool_trace: ToolTraceItem[]
  diagnosis?: Diagnosis | null
  audit_result: AuditResult
  risk_graph?: RiskGraph | null
  security_report?: SecurityReport | null
  reasoning_trace?: ReasoningTrace | null
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
  audit_metadata?: Record<string, unknown>
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
