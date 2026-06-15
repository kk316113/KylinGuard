import json
from pathlib import Path
import urllib.request


def test_audit_trace_returns_required_fields(server_url):
    sample_path = Path(__file__).resolve().parents[2] / "data" / "sample_traces" / "sample_safe_system_check.json"
    payload = sample_path.read_bytes()

    request = urllib.request.Request(
        f"{server_url}/audit/trace",
        data=payload,
        method="POST",
        headers={"Content-Type": "application/json"},
    )

    with urllib.request.urlopen(request, timeout=5) as response:
        assert response.status == 200
        body = json.loads(response.read().decode("utf-8"))
    for field in ["decision", "risk_score", "violations", "evidence_chain", "method"]:
        assert field in body
    assert body["decision"] in {"allow", "deny", "review"}
    assert isinstance(body["violations"], list)
    assert isinstance(body["evidence_chain"], list)
    assert body["method"] == "traceshield"


def test_audit_trace_preserves_semantic_risk_graph_nodes(server_url):
    sample_path = Path(__file__).resolve().parents[2] / "data" / "sample_traces" / "sample_safe_system_check.json"
    payload = sample_path.read_bytes()

    request = urllib.request.Request(
        f"{server_url}/audit/trace",
        data=payload,
        method="POST",
        headers={"Content-Type": "application/json"},
    )

    with urllib.request.urlopen(request, timeout=5) as response:
        assert response.status == 200
        body = json.loads(response.read().decode("utf-8"))

    assert body["method"] == "traceshield"
    nodes = body["risk_graph"]["nodes"]
    assert nodes
    assert nodes[0]["resource_type"] == "os_info"
    assert nodes[0]["boundary_level"] == "public"
    assert nodes[0]["operation_type"] == "read"
    assert nodes[0]["allowed_by_policy"] is True
    assert nodes[1]["resource_type"] == "network_port"
    assert nodes[1]["boundary_level"] == "low"
    assert body["risk_graph"]["edges"] == [{"from": 1, "to": 2, "type": "sequence"}]
