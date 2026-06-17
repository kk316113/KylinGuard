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
    # Agent loop step tracking.
    _agent_step_counter = 0
    _agent_step_task = ""

    def do_OPTIONS(self):
        self.send_response(200)
        self.end_headers()

    def do_POST(self):
        parsed = urlparse(self.path)
        qs = parse_qs(parsed.query)
        mode = qs.get("mode", ["valid_plan"])[0]

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

        # Detect agent loop mode: look for "Step history:" in system message.
        messages = req_data.get("messages", [])
        is_agent_loop = False
        step_count = 0
        user_text = ""
        for msg in messages:
            if isinstance(msg, dict):
                content_text = str(msg.get("content", ""))
                if "Step history:" in content_text:
                    is_agent_loop = True
                    # Count actual executed steps: lines matching "Step N:" where N is a number.
                    # "Step history:" itself does not match.
                    import re
                    step_count = len(re.findall(r"Step \d+:", content_text))
                if msg.get("role") == "user" and content_text:
                    user_text = content_text

        if is_agent_loop:
            content = self._agent_loop_response(step_count, user_text, messages)
        elif mode == "invalid_json":
            content = "this is not valid json"
        elif mode in TOOL_PLANS:
            plan = TOOL_PLANS[mode]
            content = json.dumps(plan, ensure_ascii=False)
        else:
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

    def _agent_loop_response(self, step_count, user_text, messages):
        """Return stepwise next_action based on current step count."""
        is_ssh = any(kw in user_text.lower() for kw in ["ssh", "连不上", "连接", "login", "sshd"])

        if step_count == 0:
            if is_ssh:
                return json.dumps({
                    "action_type": "tool_call",
                    "tool_name": "service_status",
                    "tool_args": {"service_name": "sshd"},
                    "reason": "SSH connectivity depends on whether the sshd service is running.",
                    "user_visible_summary": "我先检查 SSH 服务是否正在运行。"
                })
            return json.dumps({
                "action_type": "tool_call",
                "tool_name": "os_info",
                "tool_args": {},
                "reason": "Collect OS context for diagnosis.",
                "user_visible_summary": "正在收集系统信息。"
            })
        elif step_count == 1:
            if is_ssh:
                return json.dumps({
                    "action_type": "tool_call",
                    "tool_name": "port_checker",
                    "tool_args": {"host": "127.0.0.1", "port": 22},
                    "reason": "Even if sshd is running, port 22 must be listening for SSH connections.",
                    "user_visible_summary": "SSH 服务运行后，我继续检查 22 端口是否监听。"
                })
            return json.dumps({
                "action_type": "tool_call",
                "tool_name": "resource_usage_checker",
                "tool_args": {},
                "reason": "Check system resource usage for diagnosis.",
                "user_visible_summary": "正在检查系统资源使用情况。"
            })
        elif step_count == 2:
            if is_ssh:
                return json.dumps({
                    "action_type": "tool_call",
                    "tool_name": "journalctl_reader",
                    "tool_args": {"service_name": "sshd", "lines": 50},
                    "reason": "Service and port status are not enough; recent sshd logs may show authentication or connection errors.",
                    "user_visible_summary": "服务和端口检查后，我再读取最近的 SSH 日志寻找认证或连接异常。"
                })
            return json.dumps({
                "action_type": "tool_call",
                "tool_name": "disk_memory_checker",
                "tool_args": {"include_tmpfs": False},
                "reason": "Check disk and memory for resource diagnosis.",
                "user_visible_summary": "正在检查磁盘和内存状态。"
            })
        else:
            if is_ssh:
                return json.dumps({
                    "action_type": "final_answer",
                    "final_answer": "我已经完成 SSH 连接问题排查。当前 sshd 服务状态、22 端口监听情况和最近 SSH 日志显示如下。更可能的原因是账号认证、来源 IP 限制、防火墙或安全组规则。建议检查 /etc/ssh/sshd_config 中 PermitRootLogin、PasswordAuthentication 等配置。本次排查读取了服务状态、端口状态和最近 SSH 日志，日志属于敏感只读资源，系统已记录证据链并纳入安全审计。",
                    "confidence": "medium",
                    "next_suggestions": [
                        "确认账号密码或密钥是否正确",
                        "检查防火墙或安全组是否限制来源 IP",
                        "检查 /etc/ssh/sshd_config"
                    ]
                })
            return json.dumps({
                "action_type": "final_answer",
                "final_answer": "已完成系统检查。本次检查覆盖了系统信息、资源使用和磁盘内存状态。所有工具调用已通过安全审计。",
                "confidence": "high",
                "next_suggestions": []
            })

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
