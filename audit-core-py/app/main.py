from fastapi import FastAPI

from .audit_stub import AuditRequest, AuditResult, audit_trace

app = FastAPI(title="KylinGuard Audit Core Stub", version="0.1.0")


@app.get("/health")
async def health() -> dict:
    return {
        "status": "ok",
        "service": "audit-core-py",
        "mode": "stub",
    }


@app.post("/audit/trace", response_model=AuditResult)
async def audit_trace_endpoint(payload: AuditRequest) -> AuditResult:
    return audit_trace(payload)
