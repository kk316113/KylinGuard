# Stage 15A: One-click Demo Runtime

## 1. 目标

提供稳定、可重复、适合 Kylin VM 演示的一键运行方案，让项目可以以"完整 Agent 产品"的形式启动。

## 2. 默认演示 (deterministic)

```bash
cd /opt/kylin-guard-agent
bash scripts/linux/start_demo.sh
```

启动：
- audit-core-py (port 8001)
- Go Agent (port 8080)
- Frontend (port 5173)

默认 EINO_LLM_ENABLED=false，使用 deterministic-stub。

## 3. Mock LLM 演示

```bash
DEMO_MOCK_LLM=true bash scripts/linux/start_demo.sh
```

额外启动：
- Mock OpenAI-compatible server (port 8800)
- EINO_LLM_ENABLED=true
- EINO_LLM_PROVIDER=openai_compatible
- 使用 mock API_KEY（sk-mock-key）

## 4. 停止演示

```bash
bash scripts/linux/stop_demo.sh
```

## 5. 健康检查

```bash
bash scripts/linux/check_demo.sh
```

输出 audit-core-py、Go Agent、frontend、mock LLM 的状态。

## 6. 端口配置

| 服务 | 默认端口 | 环境变量 |
|------|---------|---------|
| audit-core-py | 8001 | AUDIT_CORE_PORT |
| Go Agent | 8080 | AGENT_GO_PORT |
| Frontend | 5173 | FRONTEND_PORT |
| Mock LLM | 8800 | MOCK_LLM_PORT |

## 7. 前端启动

Node.js 18+ 需要预先安装。

如果 Node 不存在：

```bash
# 后端已启动，可手动启动前端
cd /opt/kylin-guard-agent/frontend
npm run dev
```

## 8. 推荐演示流程

见 [demo_script_for_competition.md](demo_script_for_competition.md)

## 9. 常见问题

### npm not found

```bash
# Ubuntu / Kylin
sudo apt install nodejs npm
# 或使用 nvm
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash
nvm install 18
```

### Frontend port occupied

```bash
FRONTEND_PORT=5174 bash scripts/linux/start_demo.sh
```

### Go Agent unhealthy

检查日志：

```bash
tail -n 50 logs/agent-go.log
```

### audit-core-py unhealthy

检查日志：

```bash
tail -n 50 logs/audit-core.log
```

### Mock LLM port occupied

```bash
MOCK_LLM_PORT=8801 DEMO_MOCK_LLM=true bash scripts/linux/start_demo.sh
```

## 10. 安全说明

- 不使用真实 LLM API_KEY
- Mock LLM 使用 `sk-mock-key`，不连接真实 API
- 默认 deterministic-stub 路径不需要任何 LLM 配置
