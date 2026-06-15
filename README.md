# 麒盾 KylinGuard：面向麒麟操作系统的安全智能运维 Agent

项目名称：KylinGuard-Agent

当前阶段：Stage 1：接入清洗后的 TraceShield 审计核心

## 当前已完成

- Go Agent 服务骨架
- 运维工具注册接口
- 安全意图过滤占位
- 工具调用 trace 结构
- Python audit-core-py 独立审计服务
- TraceShield adapter 接入层
- Go Agent 通过 HTTP 调用 audit-core-py
- 麒麟部署脚本占位

## 当前未做

- 未改写 TraceShield 论文核心方法
- 未实现 Boundary Lattice
- 未实现真实 Data-flow Evidence
- 未实现真实 Provenance Contract
- 未接入前端页面
- 未接入真实大模型

## 目录概览

- `agent-go/`：Go Agent 最小后端服务，默认监听 `8080`
- `audit-core-py/`：Python FastAPI 审计服务，封装 TraceShield adapter
- `frontend/`：前端占位目录
- `data/`：样例 trace、审计 case、报告输出目录
- `deploy/kylin/`：麒麟/Linux 部署脚本占位
- `docs/`：架构、工具、安全和 TODO 文档

## 启动 Go 服务

```bash
cd agent-go
go run ./cmd/server
```

默认端口为 `8080`，也可以通过环境变量调整：

```bash
KYLIN_GUARD_AGENT_PORT=8081 go run ./cmd/server
```

## 启动 Python audit-core

```bash
cd audit-core-py
python3 -m venv .venv
. .venv/bin/activate
python -m pip install -r requirements.txt
TRACESHIELD_CORE_PATH=/path/to/TraceShield-Core python -m uvicorn app.main:app --host 0.0.0.0 --port 8001
```

## 接口测试

```bash
curl http://127.0.0.1:8080/health
curl http://127.0.0.1:8080/api/os/info
curl -X POST http://127.0.0.1:8080/api/agent/run \
  -H "Content-Type: application/json" \
  -d '{"task":"检查当前系统状态"}'
```

Python audit-core：

```bash
curl http://127.0.0.1:8001/health
curl http://127.0.0.1:8001/audit/capabilities
curl -X POST http://127.0.0.1:8001/audit/trace \
  -H "Content-Type: application/json" \
  -d @data/sample_traces/sample_safe_system_check.json
```

## Stage 1 接入说明

TraceShield 是清洗后的论文核心方法来源，源码目录默认位于 `D:\code\2026\TraceShield-Core`。当前采用策略 A：`audit-core-py` 通过 `TRACESHIELD_CORE_PATH` 动态加入 Python import 路径并调用 `traceshield_experiment_core.TraceShieldEvaluator`，不复制整个 TraceShield 仓库，也不修改其内部逻辑。

Go/Eino Agent 不直接依赖 TraceShield，只调用 `AUDIT_CORE_URL` 指向的 HTTP API。后续 LoongArch 部署只需要保证 Python、FastAPI、PyYAML、Pydantic 和 `audit-core-py` 可运行。

如果 TraceShield 无法 import 或运行失败，`audit-core-py` 会返回 `method=fallback-mock`，并在 `message` 中说明 fallback 原因。
