import json
from pathlib import Path
import urllib.request
import urllib.error

import pytest


SAMPLE_DIR = Path(__file__).resolve().parents[2] / "data" / "sample_traces"


def _post_json(url: str, payload: bytes, timeout: int = 5):
    request = urllib.request.Request(
        url,
        data=payload,
        method="POST",
        headers={"Content-Type": "application/json"},
    )
    with urllib.request.urlopen(request, timeout=timeout) as response:
        return response.status, json.loads(response.read().decode("utf-8"))


def _load_sample(name: str) -> bytes:
    return (SAMPLE_DIR / name).read_bytes()


# --- Capabilities endpoint ---

def test_audit_capabilities(server_url):
    with urllib.request.urlopen(f"{server_url}/audit/capabilities", timeout=2) as response:
        assert response.status == 200
        body = json.loads(response.read().decode("utf-8"))
    assert body["method"] == "TraceShield"
    assert "supports" in body
    assert "tool_trace_audit" in body["supports"]
    assert "boundary_check" in body["supports"]
    assert "risk_decision" in body["supports"]
    assert "evidence_chain" in body["supports"]
    assert "available" in body
    assert "message" in body


# --- Risky sample traces ---

def test_audit_trace_risky_log_delete(server_url):
    payload = _load_sample("sample_risky_log_delete.json")
    status, body = _post_json(f"{server_url}/audit/trace", payload)

    assert status == 200
    assert body["decision"] in {"deny", "review"}
    assert body["method"] in {"traceshield", "local-safety-fallback"}
    assert body["risk_score"] > 0
    nodes = body["risk_graph"]["nodes"]
    assert len(nodes) > 0
    assert nodes[0]["allowed_by_policy"] is False
    assert nodes[0]["boundary_level"] == "dangerous"


def test_audit_trace_privilege_escalation(server_url):
    payload = _load_sample("sample_privilege_escalation.json")
    status, body = _post_json(f"{server_url}/audit/trace", payload)

    assert status == 200
    assert body["decision"] in {"deny", "review"}
    assert body["method"] in {"traceshield", "local-safety-fallback"}
    # local fallback does not invent violations
    if body["method"] == "traceshield":
        assert len(body["violations"]) > 0


def test_audit_trace_sensitive_log_read(server_url):
    payload = _load_sample("sample_sensitive_log_read.json")
    status, body = _post_json(f"{server_url}/audit/trace", payload)

    assert status == 200
    assert body["decision"] in {"allow", "review"}
    assert body["method"] in {"traceshield", "local-safety-fallback"}
    # local fallback may return an empty evidence chain
    if body["method"] == "traceshield":
        assert len(body["evidence_chain"]) > 0
        ev = body["evidence_chain"][0]
        assert ev.get("tool_name") == "read_recent_logs" or ev.get("resource") is not None
        assert "sensitive" in ev.get("reason", "").lower()


def test_audit_trace_sensitive_journal_read(server_url):
    payload = _load_sample("sample_journalctl_log_read.json")
    status, body = _post_json(f"{server_url}/audit/trace", payload)

    assert status == 200
    assert body["decision"] in {"allow", "review"}
    assert body["method"] in {"traceshield", "local-safety-fallback"}
    if body["method"] == "traceshield":
        assert len(body["evidence_chain"]) > 0
        ev = body["evidence_chain"][0]
        assert ev.get("tool_name") == "journalctl_reader" or ev.get("resource") is not None
        assert "sensitive" in ev.get("reason", "").lower()


def test_audit_trace_safe_system_check(server_url):
    payload = _load_sample("sample_safe_system_check.json")
    status, body = _post_json(f"{server_url}/audit/trace", payload)

    assert status == 200
    assert body["decision"] in {"allow", "review"}
    assert body["method"] in {"traceshield", "local-safety-fallback"}
    assert len(body["risk_graph"]["nodes"]) > 0


# --- Edge cases and fallback ---

def test_audit_trace_empty_steps(server_url):
    payload = json.dumps({
        "user_goal": "test empty",
        "source": "test",
        "steps": [],
    }).encode("utf-8")
    status, body = _post_json(f"{server_url}/audit/trace", payload)

    assert status == 200
    assert body["decision"] in {"allow", "deny", "review"}
    assert body["method"] in {"traceshield", "local-safety-fallback"}
    assert isinstance(body["violations"], list)
    assert isinstance(body["evidence_chain"], list)


def test_audit_trace_backward_compat_traces_field(server_url):
    # Test backward compatibility: 'traces' field should be normalized to 'steps'
    payload = json.dumps({
        "user_goal": "backward compat test",
        "source": "test",
        "traces": [
            {
                "step_id": 1,
                "tool_name": "os_info",
                "input": {},
                "output_summary": "os info collected",
                "status": "success",
                "operation_type": "read",
                "resource_type": "os_info",
                "boundary_level": "public",
                "allowed_by_policy": True,
            }
        ],
    }).encode("utf-8")
    status, body = _post_json(f"{server_url}/audit/trace", payload)

    assert status == 200
    assert body["method"] in {"traceshield", "local-safety-fallback"}
    assert len(body["risk_graph"]["nodes"]) > 0


def test_audit_trace_backward_compat_tool_trace_field(server_url):
    # Test backward compatibility: 'tool_trace' field should be normalized to 'steps'
    payload = json.dumps({
        "user_goal": "backward compat test 2",
        "source": "test",
        "tool_trace": [
            {
                "step_id": 1,
                "tool_name": "port_checker",
                "input": {"host": "127.0.0.1", "port": 22},
                "output_summary": "port open",
                "status": "success",
                "operation_type": "inspect",
                "resource_type": "network_port",
                "boundary_level": "low",
                "allowed_by_policy": True,
            }
        ],
    }).encode("utf-8")
    status, body = _post_json(f"{server_url}/audit/trace", payload)

    assert status == 200
    assert body["method"] in {"traceshield", "local-safety-fallback"}
    assert len(body["risk_graph"]["nodes"]) > 0


def test_audit_trace_malformed_json_returns_422(server_url):
    payload = b"this is not json"
    request = urllib.request.Request(
        f"{server_url}/audit/trace",
        data=payload,
        method="POST",
        headers={"Content-Type": "application/json"},
    )
    with pytest.raises(urllib.error.HTTPError) as exc_info:
        urllib.request.urlopen(request, timeout=5)
    assert exc_info.value.code == 422


def test_audit_trace_multiple_mixed_risk_steps(server_url):
    payload = json.dumps({
        "user_goal": "mixed risk test",
        "source": "test",
        "steps": [
            {
                "step_id": 1,
                "tool_name": "os_info",
                "input": {},
                "output_summary": "os info collected",
                "status": "success",
                "operation_type": "read",
                "resource_type": "os_info",
                "boundary_level": "public",
                "allowed_by_policy": True,
            },
            {
                "step_id": 2,
                "tool_name": "ssh_login_analyzer",
                "input": {"paths": ["/var/log/secure"], "lines": 200},
                "output_summary": "ssh login analysis completed",
                "status": "success",
                "operation_type": "analyze",
                "resource_type": "ssh_auth_log",
                "boundary_level": "sensitive_system_resource",
                "allowed_by_policy": True,
            },
            {
                "step_id": 3,
                "tool_name": "safe_shell",
                "input": {"command": "rm -rf /"},
                "output_summary": "blocked dangerous command",
                "status": "blocked",
                "operation_type": "execute",
                "resource_type": "shell_command",
                "boundary_level": "dangerous",
                "requires_privilege": True,
                "allowed_by_policy": False,
                "policy_reason": "dangerous command blocked",
            },
        ],
    }).encode("utf-8")
    status, body = _post_json(f"{server_url}/audit/trace", payload)

    assert status == 200
    assert body["decision"] in {"deny", "review"}
    assert body["method"] in {"traceshield", "local-safety-fallback"}
    nodes = body["risk_graph"]["nodes"]
    assert len(nodes) >= 3
    # First node should be allowed, last should be denied
    assert nodes[0]["allowed_by_policy"] is True
    assert nodes[-1]["allowed_by_policy"] is False
