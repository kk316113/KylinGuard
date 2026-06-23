# KylinGuard / 麒盾

面向麒麟 V11 的安全智能运维 Agent。

用户用自然语言描述问题，Agent 通过安全受控的只读工具采集证据，返回诊断结论、执行步骤、审计结果和风险图。

## 快速启动

### 1. 启动 audit-core

```powershell
cd E:\KylinGuard\audit-core-py
python -m uvicorn app.main:app --host 127.0.0.1 --port 8001
```

### 2. 启动 Go Agent

```powershell
cd E:\KylinGuard\agent-go
$env:AUDIT_CORE_URL = "http://127.0.0.1:8001"
go run ./cmd/server
```

Agent 地址：

```text
http://127.0.0.1:8080
```

### 3. 启动前端

```powershell
cd E:\KylinGuard\frontend
npm install
npm run dev
```

浏览器打开：

```text
http://127.0.0.1:5173
```

## 真实 DeepSeek 模式

在启动 Go Agent 前设置 DeepSeek/OpenAI-compatible 的 base URL、model 和 API key 环境变量，然后再启动 Go Agent。

不要把真实 API key 写入仓库、README、`.env` 或前端环境变量。

## 麒麟 systemd 部署

在麒麟环境中：

```bash
sudo bash deploy/kylin/install_stack.sh
sudo bash deploy/kylin/check_stack.sh
```

服务：

```text
kylin-guard-agent.service
kylin-guard-audit.service
kylin-guard-web.service
```

前端默认端口：

```text
5173
```

## 关键接口

```text
GET  /health
POST /api/agent/run
POST /api/agent/run-eino
GET  /api/agent/runtime-status
GET  /api/agent/capabilities
GET  /api/agent/acceptance-summary
POST /mcp
```

主任务接口是：

```text
POST /api/agent/run
```

`/api/agent/run-eino` 仅保留为兼容接口。

## 示例请求

```bash
curl -s -X POST http://127.0.0.1:8080/api/agent/run \
  -H "Content-Type: application/json" \
  -d '{"task":"我 SSH 连不上了，帮我看看"}'
```

## 验证

后端：

```bash
cd agent-go
go test ./...
```

前端：

```bash
cd frontend
npm run typecheck
npm run build
```

麒麟验收脚本：

```bash
bash scripts/linux/test_security_guardrails.sh
bash scripts/linux/test_mcp_protocol.sh
bash scripts/linux/test_os_sensing_tools.sh
bash scripts/linux/test_configuration_drift.sh
bash scripts/linux/test_agent_loop_tasks.sh
```

## 项目结构

```text
agent-go/       Go Agent 后端
audit-core-py/  Python 审计服务
frontend/       Next.js 前端
deploy/kylin/   麒麟部署脚本
scripts/linux/  启动、检查和验收脚本
docs/           设计与验证记录
```
