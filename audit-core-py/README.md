# audit-core-py

这是 Python FastAPI 审计服务，用于把清洗后的 TraceShield 论文核心方法封装为稳定 HTTP API。

重要说明：

- 当前通过 `TraceShieldAdapter` 调用 `traceshield_experiment_core.TraceShieldEvaluator`。
- Go/Eino Agent 不直接 import TraceShield，只调用本服务 HTTP API。
- 本服务不改写 TraceShield 内部方法逻辑，只做输入输出适配。
- 如果 TraceShield 不可用，会返回 fallback mock，并在 `message` 中说明原因。

## 依赖

- `fastapi`
- `uvicorn`
- `pydantic>=2.0`
- `PyYAML>=6.0`
- `pytest`

TraceShield-Core 自身依赖 `pydantic>=2.0` 和 `PyYAML>=6.0`。Stage 1 检查未发现 torch、transformers、faiss、sentence-transformers 等重依赖。

## TraceShield 来源

- 源仓库：`git@github.com:kk316113/TraceShield.git`
- 本机源码路径：`D:\code\2026\TraceShield-Core`
- 接入策略：策略 A，通过 `TRACESHIELD_CORE_PATH` 动态加入 `sys.path`
- 已识别入口：`TraceShieldEvaluator.evaluate_tool_events(...)`

当前 Windows 环境已验证真实 TraceShield 核心可以 import，并能完成最小审计调用。

## 启动

```bash
python3 -m venv .venv
. .venv/bin/activate
python -m pip install -r requirements.txt
TRACESHIELD_CORE_PATH=/path/to/TraceShield-Core python -m uvicorn app.main:app --host 0.0.0.0 --port 8001
```

## 接口

`GET /health`

```json
{
  "status": "ok",
  "service": "audit-core-py",
  "mode": "traceshield-adapter",
  "traceshield_available": true
}
```

`GET /audit/capabilities`

返回当前审计核心能力和可用性。

`POST /audit/trace`

```json
{
  "task_id": "optional-task-id",
  "user_goal": "检查当前系统 SSH 登录异常",
  "source": "kylin-guard-agent",
  "steps": [],
  "metadata": {
    "os": "Kylin V11",
    "arch": "loongarch64",
    "agent": "KylinGuard"
  }
}
```

`steps` 支持 Go Agent Stage 2 语义字段：`operation_type`、`resource_type`、`resource_path`、`permission_scope`、`boundary_level`、`tool_semantic`、`requires_privilege`、`allowed_by_policy`、`policy_reason`。这些字段是 optional，旧版 trace 仍可提交。

返回：

```json
{
  "decision": "allow",
  "risk_score": 0.1,
  "violations": [],
  "evidence_chain": [],
  "risk_graph": {
    "nodes": [
      {
        "step_id": 1,
        "tool_name": "os_info",
        "operation_type": "read",
        "resource_type": "os_info",
        "resource_path": "system:os",
        "boundary_level": "public",
        "risk_hint": "low",
        "status": "success",
        "allowed_by_policy": true
      }
    ],
    "edges": [
      {
        "from": 1,
        "to": 2,
        "type": "sequence"
      }
    ]
  },
  "method": "traceshield",
  "message": "audit completed by TraceShield adapter"
}
```

## Fallback

当 `TRACESHIELD_CORE_PATH` 缺失、TraceShield import 失败或真实审计调用抛出异常时，服务返回：

```json
{
  "decision": "review",
  "method": "fallback-mock",
  "message": "fallback mock used: ..."
}
```
