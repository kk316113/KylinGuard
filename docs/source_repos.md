# Source Repositories

## TraceShield-Core

- Source: `git@github.com:kk316113/TraceShield.git`
- Local path: `D:\code\2026\TraceShield-Core`
- Role: 清洗后的 TraceShield 论文核心方法来源。
- Current integration strategy: 策略 A，通过 `TRACESHIELD_CORE_PATH` 动态引用，不复制整个仓库。

## Identified Core Entry

- Python package: `traceshield_experiment_core`
- Main evaluator: `TraceShieldEvaluator`
- Callable entries:
  - `TraceShieldEvaluator.evaluate_trace(sample)`
  - `TraceShieldEvaluator.evaluate_tool_events(sample_id, intent, trace)`
- Schemas:
  - `DatasetSample`
  - `IntentFrame`
  - `ToolEvent`
  - `AuditResult`

## Dependency Review

TraceShield-Core currently declares:

- `pydantic>=2.0`
- `PyYAML>=6.0`

No `torch`, `transformers`, `faiss`, or `sentence-transformers` dependency was found during Stage 1 inspection.

## Boundary Rule

KylinGuard-Agent must not modify TraceShield-Core history or internal method logic. The integration layer only performs input/output adaptation inside `audit-core-py/app/traceshield_adapter.py`.
