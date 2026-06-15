# deploy

部署脚本目录。

`deploy/kylin/` 当前提供面向麒麟/Linux 环境的占位脚本：

- `check_env.sh`：检查系统架构、系统版本、Go、Python、pip、gcc、systemctl
- `install_agent_go.sh`：构建 Go Agent
- `run_agent_go.sh`：启动 Go Agent
- `install_audit_core_py.sh`：安装 Python audit-core 依赖
- `run_audit_core_py.sh`：启动 Python FastAPI audit-core 服务，默认端口 `8001`

脚本不包含 Windows 路径。后续需要结合银河麒麟高级服务器版 V11 和 LoongArch 环境继续验证。
