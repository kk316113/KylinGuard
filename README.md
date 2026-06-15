# 麒盾 KylinGuard：面向麒麟操作系统的安全智能运维 Agent

项目名称：KylinGuard-Agent

当前阶段：Stage 0：工程骨架初始化

## 当前已完成

- Go Agent 服务骨架
- 运维工具注册接口
- 安全意图过滤占位
- 工具调用 trace 结构
- Python 审计核心占位服务
- 麒麟部署脚本占位

## 当前未做

- 未接入论文审计方法
- 未实现 Boundary Lattice
- 未实现真实 Data-flow Evidence
- 未实现真实 Provenance Contract
- 未接入前端页面
- 未接入真实大模型

## 目录概览

- `agent-go/`：Go Agent 最小后端服务，默认监听 `8080`
- `audit-core-py/`：Python FastAPI 审计核心占位服务
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

## 启动 Python audit-core stub

```bash
cd audit-core-py
python3 -m venv .venv
. .venv/bin/activate
python -m pip install -r requirements.txt
python -m uvicorn app.main:app --host 0.0.0.0 --port 8090
```

## 接口测试

```bash
curl http://127.0.0.1:8080/health
curl http://127.0.0.1:8080/api/os/info
curl -X POST http://127.0.0.1:8080/api/agent/run \
  -H "Content-Type: application/json" \
  -d '{"task":"检查当前系统状态"}'
```

Python stub：

```bash
curl http://127.0.0.1:8090/health
curl -X POST http://127.0.0.1:8090/audit/trace \
  -H "Content-Type: application/json" \
  -d '{"traces":[]}'
```

## Stage 0 约束

当前仓库只提供最小可启动骨架、接口占位和 mock 返回。模型能力默认面向远程 API 调用，不在本机或麒麟虚拟机内运行大模型。`audit-core-py` 不包含论文方法，也不代表最终算法。
