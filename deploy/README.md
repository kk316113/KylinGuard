# deploy

部署脚本目录。

`deploy/kylin/` 当前提供面向麒麟/Linux 环境的预检查、安装和启动脚本。脚本均使用 bash，并以 `set -euo pipefail` 运行。

- `check_env.sh`：检查架构、系统版本、Go、Python、pip、gcc、systemctl、journalctl、ss/netstat、用户和工作目录
- `install_agent_go.sh`：执行 `go mod tidy`、`go test ./...` 并构建 Go Agent
- `run_agent_go.sh`：启动 Go Agent，启动前检查 audit-core-py 是否可访问
- `install_audit_core_py.sh`：创建 `.venv` 并安装 Python audit-core 依赖，拒绝重模型依赖
- `run_audit_core_py.sh`：检查 `.venv` 和 `TRACESHIELD_CORE_PATH` 后启动 FastAPI 服务

## 环境变量

- `KYLINGUARD_HOME`：默认当前仓库根目录，部署建议 `/opt/kylin-guard-agent`
- `TRACESHIELD_CORE_PATH`：默认 `/opt/traceshield-core`
- `AUDIT_CORE_URL`：默认 `http://127.0.0.1:8001`
- `AGENT_GO_PORT`：默认 `8080`
- `AUDIT_CORE_PORT`：默认 `8001`

脚本不包含 Windows 路径。后续需要结合银河麒麟高级服务器版 V11 和 LoongArch 环境继续验证。
