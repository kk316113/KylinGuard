from fastapi import FastAPI

from .schemas import AuditTraceRequest, AuditTraceResponse
from .traceshield_adapter import TraceShieldAdapter

app = FastAPI(title="KylinGuard Audit Core", version="0.2.0")
adapter = TraceShieldAdapter()


@app.get("/health")
async def health() -> dict:
    return {
        "status": "ok",
        "service": "audit-core-py",
        "mode": "traceshield-adapter",
        "traceshield_available": adapter.is_available(),
    }


@app.get("/audit/capabilities")
async def capabilities() -> dict:
    return {
        "method": "TraceShield",
        "supports": [
            "tool_trace_audit",
            "boundary_check",
            "risk_decision",
            "evidence_chain",
        ],
        "available": adapter.is_available(),
        "message": "TraceShield core available" if adapter.is_available() else adapter.unavailable_reason,
    }


@app.post("/audit/trace", response_model=AuditTraceResponse)
async def audit_trace_endpoint(payload: AuditTraceRequest) -> AuditTraceResponse:
    return adapter.audit_trace(payload)
