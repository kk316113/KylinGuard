# audit-core-py

这是 Python FastAPI 审计核心占位服务，只用于 Stage 0 工程骨架联调。

重要说明：

- 当前服务只是 stub。
- 当前服务不包含论文方法。
- 当前服务不代表最终算法。
- 当前服务不会实现 Boundary Lattice、Data-flow Evidence、Provenance Contract 或其他论文审计逻辑。

## 依赖

`requirements.txt` 只包含：

- `fastapi`
- `uvicorn`
- `pydantic`

## 启动

```bash
python3 -m venv .venv
. .venv/bin/activate
python -m pip install -r requirements.txt
python -m uvicorn app.main:app --host 0.0.0.0 --port 8090
```

## 接口

`GET /health`

```json
{
  "status": "ok",
  "service": "audit-core-py",
  "mode": "stub"
}
```

`POST /audit/trace`

```json
{
  "traces": []
}
```

返回：

```json
{
  "decision": "review",
  "risk_score": 0.35,
  "violations": [],
  "evidence_chain": [],
  "message": "stub audit core, real paper method will be integrated later"
}
```
