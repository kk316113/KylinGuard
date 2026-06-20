# deploy

部署脚本目录。

`deploy/kylin/` 提供面向 Linux / 银河麒麟环境的预检查、安装和启动脚本。脚本均使用 bash，并以 `set -euo pipefail` 运行。

## 脚本

- `check_env.sh`：检查架构、系统版本、Go、Python、pip、gcc、systemctl、journalctl、ss/netstat、当前用户和工作目录。
- `install_agent_go.sh`：执行 `go mod tidy`、`go test ./...`，并构建 Go Agent 二进制。
- `run_agent_go.sh`：启动 Go Agent，启动前检查 audit-core-py 是否可访问；不可访问时给出警告但允许启动。
- `install_agent_service.sh`：将已构建二进制安装为专用 `kylinguard` 非 root 账户运行的 systemd 服务。
- `install_audit_core_py.sh`：创建 `.venv` 并安装 Python audit-core 依赖；不安装 torch、transformers、faiss 等重依赖。
- `run_audit_core_py.sh`：检查 `.venv` 和 `TRACESHIELD_CORE_PATH` 后启动 FastAPI 服务。

## 环境变量

- `KYLINGUARD_HOME`：默认当前仓库根目录，部署建议 `/opt/kylin-guard-agent`
- `TRACESHIELD_CORE_PATH`：默认 `/opt/traceshield-core`
- `AUDIT_CORE_URL`：默认 `http://127.0.0.1:8001`
- `AGENT_GO_PORT`：默认 `8080`
- `AUDIT_CORE_PORT`：默认 `8001`
- `EINO_ENABLED`：默认 `false`，仅表示不启用真实 LLM，保留兼容含义
- `EINO_RUNTIME_ENABLED`：默认 `true`
- `EINO_GRAPH_ENABLED`：默认 `true`
- `EINO_LLM_ENABLED`：默认 `false`

Stage 9B 中，`/api/agent/run-eino` 默认进入 CloudWeGo Eino graph runtime，但仍不接真实 LLM、不接模型厂商 SDK、不读取 API key。

脚本不包含 Windows 路径。后续仍需要结合银河麒麟高级服务器版 V11 和 LoongArch 环境继续验证。

赛题点名的文件占用感知依赖 `lsof`。`check_env.sh` 会报告其版本；最终部署环境缺少 `lsof` 时，`scripts/linux/test_os_sensing_tools.sh` 将直接失败，不会用模拟结果替代。

## 最小权限 systemd 部署

先构建，再以 root 安装服务：

```bash
bash deploy/kylin/install_agent_go.sh
sudo bash deploy/kylin/install_agent_service.sh
```

服务默认仅监听 `127.0.0.1:8080`，配置文件为
`/etc/kylin-guard/agent.env`。安装器不会写入任何模型 API Key；真实密钥仍必须由部署环境安全注入。

systemd unit 禁止提权和新增 capability，并启用只读系统目录、私有临时目录、设备隔离、内核保护和命名空间限制。Agent 的进程、网络、日志等读取能力需要在麒麟 V11 实机逐项复核；不得为了让测试通过而整体改回 root 运行。
