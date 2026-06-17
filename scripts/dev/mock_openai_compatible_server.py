#!/usr/bin/env python3
"""
Mock OpenAI-compatible /v1/chat/completions server for Stage 13B manual testing.

Run:
  python3 mock_openai_compatible_server.py [port]

Supports modes via ?mode= query parameter:
  valid_plan  (default)  - returns valid JSON ToolPlan
  invalid_json           - returns non-JSON content
  unknown_tool           - returns plan with unknown tool 'cmdexec'
  safe_shell             - returns plan with 'safe_shell'
  dangerous              - returns plan with 'rm'
  empty_plan             - returns plan with empty tool_plan
  slow                   - sleeps 5 seconds before responding
  error                  - returns HTTP 500
"""

import json
import sys
import time
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse, parse_qs

PORT = int(sys.argv[1]) if len(sys.argv) > 1 else 8800

TOOL_PLANS = {
    "valid_plan": {
        "scenario": "system_resource_check",
        "intent": "check system resource usage",
        "tool_plan": [
            {"tool_name": "os_info", "reason": "collect OS context", "arguments": {}},
            {"tool_name": "resource_usage_checker", "reason": "check load and memory", "arguments": {}},
            {"tool_name": "disk_memory_checker", "reason": "check disk and memory", "arguments": {"include_tmpfs": False}},
        ],
        "risk_hint": "low",
        "requires_review": False,
        "user_explanation": "Checking system resources."
    },
    "unknown_tool": {
        "scenario": "custom_task",
        "intent": "execute custom command",
        "tool_plan": [
            {"tool_name": "cmdexec", "reason": "execute arbitrary command", "arguments": {"cmd": "whoami"}}
        ],
        "risk_hint": "high",
        "requires_review": True,
    },
    "safe_shell": {
        "scenario": "diagnostic",
        "intent": "run safe shell command",
        "tool_plan": [
            {"tool_name": "safe_shell", "reason": "execute diagnostic command", "arguments": {"command": "whoami"}}
        ],
        "risk_hint": "low",
        "requires_review": False,
    },
    "dangerous": {
        "scenario": "system_cleanup",
        "intent": "remove temporary files",
        "tool_plan": [
            {"tool_name": "rm", "reason": "delete files", "arguments": {"path": "/tmp"}}
        ],
        "risk_hint": "high",
        "requires_review": True,
    },
    "empty_plan": {
        "scenario": "unknown",
        "intent": "empty task",
        "tool_plan": [],
        "risk_hint": "low",
        "requires_review": False,
    },
}


class MockHandler(BaseHTTPRequestHandler):
    def do_OPTIONS(self):
        self.send_response(200)
        self.end_headers()

    def do_POST(self):
        parsed = urlparse(self.path)
        qs = parse_qs(parsed.query)
        mode = qs.get("mode", ["valid_plan"])[0]

        if mode == "error":
            self.send_response(500)
            self.end_headers()
            self.wfile.write(json.dumps({"error": "internal error"}).encode())
            return

        if mode == "slow":
            time.sleep(5)

        if mode == "invalid_json":
            content = "this is not valid json"
        elif mode in TOOL_PLANS:
            plan = TOOL_PLANS[mode]
            content = json.dumps(plan, ensure_ascii=False)
        else:
            plan = TOOL_PLANS["valid_plan"]
            content = json.dumps(plan, ensure_ascii=False)

        resp = {
            "choices": [
                {
                    "message": {
                        "content": content
                    }
                }
            ]
        }

        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(resp, ensure_ascii=False).encode())

    def log_message(self, format, *args):
        # Do not log Authorization header or API keys.
        msg = format % args
        # Redact any sensitive-looking parts.
        if "Bearer" in msg or "key" in msg.lower():
            msg = "[REDACTED]"
        sys.stderr.write(f"{self.log_date_time_string()} - {msg}\n")


if __name__ == "__main__":
    server = HTTPServer(("127.0.0.1", PORT), MockHandler)
    print(f"Mock OpenAI-compatible server running on http://127.0.0.1:{PORT}")
    print(f"Test with: curl http://127.0.0.1:{PORT}/v1/chat/completions ...")
    print(f"Modes: valid_plan, invalid_json, unknown_tool, safe_shell, dangerous, empty_plan, error, slow")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nShutting down")
        server.server_close()
