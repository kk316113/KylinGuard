from .schemas import AuditTraceRequest, AuditTraceResponse, RiskGraph


def audit_trace(payload: AuditTraceRequest, reason: str = "TraceShield adapter unavailable") -> AuditTraceResponse:
    return AuditTraceResponse(
        decision="review",
        risk_score=0.35,
        violations=[],
        evidence_chain=[],
        risk_graph=RiskGraph(
            nodes=[
                {
                    "step_id": step.step_id,
                    "tool_name": step.tool_name,
                    "status": step.status,
                }
                for step in payload.steps
            ],
            edges=[],
        ),
        method="fallback-mock",
        message=f"fallback mock used: {reason}",
    )
