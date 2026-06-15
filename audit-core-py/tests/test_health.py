import json
import urllib.request


def test_health_is_accessible(server_url):
    with urllib.request.urlopen(f"{server_url}/health", timeout=2) as response:
        assert response.status == 200
        body = json.loads(response.read().decode("utf-8"))
    assert body["status"] == "ok"
    assert body["service"] == "audit-core-py"
    assert body["mode"] == "traceshield-adapter"
    assert "traceshield_available" in body
