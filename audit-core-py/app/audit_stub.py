from typing import Any, List, Optional

from pydantic import BaseModel, Field


class ToolTrace(BaseModel):
    step_id: Optional[str] = None
    tool_name: Optional[str] = None
    input: Optional[Any] = None
    output_summary: Optional[str] = None
    status: Optional[str] = None
    started_at: Optional[str] = None
    finished_at: Optional[str] = None
    risk_hint: Optional[str] = None


class AuditRequest(BaseModel):
    traces: List[ToolTrace] = Field(default_factory=list)
    tool_trace: List[ToolTrace] = Field(default_factory=list)


class AuditResult(BaseModel):
    decision: str
    risk_score: float
    violations: List[str]
    evidence_chain: List[str]
    message: str


def audit_trace(payload: AuditRequest) -> AuditResult:
    _ = payload
    return AuditResult(
        decision="review",
        risk_score=0.35,
        violations=[],
        evidence_chain=[],
        message="stub audit core, real paper method will be integrated later",
    )
