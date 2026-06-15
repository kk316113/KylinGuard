# Kylin Adaptation

## 目标环境

- 银河麒麟高级服务器版 V11
- Linux systemd 环境
- x86_64、aarch64、LoongArch 等架构

## Stage 2.5 适配原则

- Go 代码使用标准库优先。
- Python audit-core 使用 FastAPI、uvicorn、pydantic、PyYAML 和 TraceShield-Core。
- 部署脚本使用 bash 和 `set -euo pipefail`，避免 Windows 路径。
- Agent 默认调用远程模型 API，不在本机或虚拟机内运行大模型。
- 运维工具默认走只读或低风险路径。
- Go/Eino Agent 只通过 HTTP 调用 audit-core-py，不直接 import TraceShield。

## Linux 环境变量

- `KYLINGUARD_HOME=/opt/kylin-guard-agent`
- `TRACESHIELD_CORE_PATH=/opt/traceshield-core`
- `AUDIT_CORE_URL=http://127.0.0.1:8001`
- `AGENT_GO_PORT=8080`
- `AUDIT_CORE_PORT=8001`

## 后续验证项

- 在目标 Kylin V11 系统上运行 `deploy/kylin/check_env.sh`。
- 验证 Go 编译器版本和 `GOARCH=loong64` 支持情况。
- 验证 systemd、日志路径、权限模型和服务管理策略。
- 验证 Python venv 和 pip 源可用性。
- 验证 `TRACESHIELD_CORE_PATH` 在目标机器上的部署路径。
- 验证 `journalctl`、`ss` 或 `netstat` 是否可用。
- 验证 `/var/log/secure`、`/var/log/auth.log`、`/var/log/audit/audit.log` 等日志路径和权限。
- 在 LoongArch 环境完成最终运行验证。
