from typing import Any, Dict, List, Literal, Optional, Union

from pydantic import BaseModel, Field, model_validator


StepID = Union[int, str]


class ToolTraceStep(BaseModel):
    step_id: StepID
    tool_name: str
    input: Dict[str, Any] = Field(default_factory=dict)
    output_summary: str = ""
    status: str = "success"
    started_at: Optional[str] = None
    finished_at: Optional[str] = None
    risk_hint: Optional[str] = None
    operation_type: Optional[str] = None
    resource_type: Optional[str] = None
    resource_path: Optional[str] = None
    permission_scope: Optional[str] = None
    boundary_level: Optional[str] = None
    tool_semantic: Optional[str] = None
    requires_privilege: Optional[bool] = None
    allowed_by_policy: Optional[bool] = None
    policy_reason: Optional[str] = None


class AuditTraceRequest(BaseModel):
    task_id: Optional[str] = None
    user_goal: str = ""
    source: str = "kylin-guard-agent"
    steps: List[ToolTraceStep] = Field(default_factory=list)
    metadata: Dict[str, Any] = Field(default_factory=dict)

    # Backward-compatible Stage 0 field names. They are normalized into steps.
    traces: List[ToolTraceStep] = Field(default_factory=list)
    tool_trace: List[ToolTraceStep] = Field(default_factory=list)

    @model_validator(mode="after")
    def normalize_steps(self) -> "AuditTraceRequest":
        if not self.steps:
            if self.traces:
                self.steps = self.traces
            elif self.tool_trace:
                self.steps = self.tool_trace
        return self


class AuditViolation(BaseModel):
    type: str
    severity: str
    message: str
    step_id: Optional[StepID] = None


class EvidenceItem(BaseModel):
    step_id: Optional[StepID] = None
    tool_name: Optional[str] = None
    resource: Optional[str] = None
    reason: str


class RiskGraph(BaseModel):
    nodes: List[Dict[str, Any]] = Field(default_factory=list)
    edges: List[Dict[str, Any]] = Field(default_factory=list)


class AuditTraceResponse(BaseModel):
    decision: Literal["allow", "deny", "review"]
    risk_score: float
    violations: List[AuditViolation] = Field(default_factory=list)
    evidence_chain: List[EvidenceItem] = Field(default_factory=list)
    risk_graph: RiskGraph = Field(default_factory=RiskGraph)
    method: str = "traceshield"
    message: str = "audit completed by TraceShield adapter"
