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
                    "operation_type": step.operation_type,
                    "resource_type": step.resource_type,
                    "resource_path": step.resource_path,
                    "boundary_level": step.boundary_level,
                    "risk_hint": step.risk_hint,
                    "status": step.status,
                    "allowed_by_policy": step.allowed_by_policy,
                }
                for step in payload.steps
            ],
            edges=[
                {
                    "from": previous.step_id,
                    "to": current.step_id,
                    "type": "sequence",
                }
                for previous, current in zip(payload.steps, payload.steps[1:])
            ],
        ),
        method="fallback-mock",
        message=f"fallback mock used: {reason}",
    )
