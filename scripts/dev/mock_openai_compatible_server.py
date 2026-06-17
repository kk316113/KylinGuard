#!/usr/bin/env python3
"""
Mock OpenAI-compatible /v1/chat/completions server for Stage 13B manual testing.

Run:
  python3 mock_openai_compatible_server.py [port]

Supports modes via ?mode= query parameter:
  valid_plan  (default)  - returns task-aware JSON ToolPlan
  invalid_json           - returns non-JSON content
  unknown_tool           - returns plan with unknown tool 'cmdexec'
  safe_shell             - returns plan with 'safe_shell'
  dangerous              - returns plan with 'rm'
  bash_command           - returns plan with 'bash'
  empty_plan             - returns plan with empty tool_plan
  slow                   - sleeps 5 seconds before responding
  error                  - returns HTTP 500

When mode=valid_plan (the default), the server parses the user message to
return a task-appropriate plan:
  - SSH/auth/login/anomaly keywords -> ssh_anomaly_check with full SSH toolchain
  - resource/CPU/memory/disk/load   -> system_resource_check
  - security overview/inspection    -> system_security_overview
  - port/listen                     -> port_check
  - service status                  -> service_check
  - default                         -> system_resource_check
"""

import json
import sys
import time
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse, parse_qs

PORT = int(sys.argv[1]) if len(sys.argv) > 1 else 8800


def build_valid_plan(messages):
    """Build a task-appropriate ToolPlan based on the user message content."""
    user_text = ""
    if isinstance(messages, list):
        for msg in messages:
            if isinstance(msg, dict) and msg.get("role") == "user":
                user_text = (msg.get("content") or "").lower()
                break
    elif isinstance(messages, str):
        user_text = messages.lower()

    # SSH anomaly task
    if any(kw in user_text for kw in ["ssh", "login", "auth.log", "auth_log",
                                       "authentication", "anomaly", "brute",
                                       "failed login", "suspicious"]):
        return {
            "scenario": "ssh_anomaly_check",
            "intent": "check ssh login anomaly",
            "tool_plan": [
                {"tool_name": "os_info", "reason": "collect OS context", "arguments": {}},
                {"tool_name": "service_status", "reason": "check sshd status", "arguments": {"service_name": "sshd"}},
                {"tool_name": "port_checker", "reason": "check port 22", "arguments": {"host": "127.0.0.1", "port": 22}},
                {"tool_name": "log_reader", "reason": "read auth logs", "arguments": {"paths": ["/var/log/secure", "/var/log/auth.log"], "lines": 100}},
                {"tool_name": "ssh_login_analyzer", "reason": "analyze SSH logins", "arguments": {"paths": ["/var/log/secure", "/var/log/auth.log"], "lines": 200}},
            ],
            "risk_hint": "medium",
            "requires_review": True,
            "user_explanation": "Checking SSH login security."
        }

    # Security overview
    if any(kw in user_text for kw in ["security overview", "security check",
                                        "system inspection", "security inspection"]):
        return {
            "scenario": "system_security_overview",
            "intent": "system security overview",
            "tool_plan": [
                {"tool_name": "os_info", "reason": "collect OS context", "arguments": {}},
                {"tool_name": "resource_usage_checker", "reason": "check load and memory", "arguments": {}},
                {"tool_name": "disk_memory_checker", "reason": "check disk usage", "arguments": {"include_tmpfs": False}},
                {"tool_name": "network_connection_inspector", "reason": "check network", "arguments": {"state": "LISTEN", "limit": 50}},
                {"tool_name": "service_status", "reason": "check sshd status", "arguments": {"service_name": "sshd"}},
                {"tool_name": "process_inspector", "reason": "check sshd process", "arguments": {"name": "sshd", "limit": 10}},
                {"tool_name": "journalctl_reader", "reason": "read system logs", "arguments": {"service_name": "sshd", "lines": 50}},
            ],
            "risk_hint": "medium",
            "requires_review": True,
        }

    # Port check
    if any(kw in user_text for kw in ["port", "listen"]):
        return {
            "scenario": "port_check",
            "intent": "check network port",
            "tool_plan": [
                {"tool_name": "os_info", "reason": "collect OS context", "arguments": {}},
                {"tool_name": "port_checker", "reason": "check port", "arguments": {"host": "127.0.0.1", "port": 8080}},
            ],
            "risk_hint": "low",
            "requires_review": False,
        }

    # Default: system resource check
    return {
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
    }


TOOL_PLANS = {
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
    "bash_command": {
        "scenario": "execute_command",
        "intent": "run bash command",
        "tool_plan": [
            {"tool_name": "bash", "reason": "execute command", "arguments": {"cmd": "whoami"}}
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

        # Read request body for task-aware mode.
        content_len = int(self.headers.get("Content-Length", 0))
        req_body = self.rfile.read(content_len) if content_len > 0 else b"{}"

        try:
            req_data = json.loads(req_body)
        except json.JSONDecodeError:
            req_data = {}

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
            # Task-aware mode: build plan from user message.
            messages = req_data.get("messages", [])
            plan = build_valid_plan(messages)
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
        if "Bearer" in msg or "key" in msg.lower():
            msg = "[REDACTED]"
        sys.stderr.write(f"{self.log_date_time_string()} - {msg}\n")


if __name__ == "__main__":
    server = HTTPServer(("127.0.0.1", PORT), MockHandler)
    print(f"Mock OpenAI-compatible server running on http://127.0.0.1:{PORT}")
    print(f"Modes: valid_plan (task-aware), invalid_json, unknown_tool, safe_shell, dangerous, bash_command, empty_plan, error, slow")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nShutting down")
        server.server_close()
