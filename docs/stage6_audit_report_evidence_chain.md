# Stage 6: Audit Report and Evidence Chain Enhancement

## 目标

Stage 6 在现有 `plan`、`tool_trace`、`diagnosis` 和 `audit_result` 基础上新增确定性的 `security_report`。它面向用户、评委和后续前端展示，把诊断过程、审计证据、敏感资源访问、风险解释和人工建议组织成稳定 JSON。

## 为什么引入 security_report

Stage 5 已经能完成 SSH 登录异常诊断，但接口返回仍偏工程 JSON。`security_report` 将底层工具调用链解释成报告结构，让 KylinGuard 不只“能诊断、能审计”，也能“能解释、能展示、能写报告”。

## 字段说明

- `title`：报告标题。
- `scenario`：Planner 场景，例如 `ssh_anomaly_check`。
- `overall_decision`：现有主流程决策，来自 intent_guard / TraceShield。
- `risk_level`：报告解释层风险等级，优先来自 diagnosis。
- `summary`：确定性摘要，不依赖 LLM。
- `evidence_chain`：由 tool_trace 转换得到的证据链。
- `risk_explanation`：按 planner、diagnosis、sensitive_resource、boundary_audit、dangerous_intent 等类别解释风险。
- `recommendations`：人工建议，不自动执行。
- `sensitive_resources`：敏感资源访问清单。
- `audit_metadata`：报告版本、审计方法、trace 数量、route 等元信息。

## Evidence Chain

每个 tool trace 映射为一个 evidence item，ID 形如 `E-001`。当前 SSH 异常任务会覆盖：

- `os_info`
- `service_status`
- `port_checker`
- `log_reader`
- `ssh_login_analyzer`

每条 evidence 包含该工具为什么相关、在审计中意味着什么。

## Sensitive Resources

Report Builder 会从 tool trace 中抽取：

- `boundary_level=sensitive_system_resource`
- `boundary_level=dangerous`
- `boundary_level=privileged`
- `resource_type=system_log`
- `resource_type=ssh_auth_log`
- `resource_type=audit_log`
- `resource_type` 包含 `secret` 或 `credential`

当前 SSH 任务通常会抽取 `system_log` 和 `ssh_auth_log`。

## Risk Explanation

当前会生成以下类别：

- `planner`：说明 Planner 选择了哪个诊断场景。
- `diagnosis`：解释 SSH 登录异常诊断风险等级。
- `sensitive_resource`：说明任务访问了敏感系统资源。
- `boundary_audit`：说明 TraceShield 已审计语义工具调用链。
- `dangerous_intent`：说明 intent_guard 在工具执行前拦截危险任务。

## Recommendations

Recommendations 只给人工建议，不执行处置。例如：

- 继续监控 SSH authentication logs。
- 检查重复失败来源 IP 是否符合预期。
- 检查 `/var/log/secure`、`/var/log/auth.log`、`journalctl -u sshd` 可用性。
- 不要通过 Agent 执行日志删除或审计清理请求。

所有建议当前均为 `is_destructive=false`。

## diagnosis 与 audit_result

`diagnosis` 是运维诊断结果，说明日志层面发现了什么。

`audit_result` 是 audit-core-py / TraceShield 对工具调用链的审计结果。

`security_report` 只是解释两者，不覆盖 `audit_result`，也不改变 `decision`。

## 不负责最终裁决

最重要原则：

```text
security_report 只能解释，不负责裁决。
```

最终 `decision` 仍来自 intent_guard / TraceShield / 现有 decision flow。`diagnosis.risk_level` 不会被当成最终安全决策。

## 为什么不接 LLM

Stage 6 需要稳定、可复现、可测试的报告结构。当前使用确定性规则生成报告，不依赖提示词、不调用远程模型，也不引入本地大模型依赖。

## dangerous task 报告

危险任务被 intent_guard deny 时也会生成 `security_report`：

- `overall_decision=deny`
- `tool_trace=[]`
- `risk_explanation` 包含 `dangerous_intent`
- `summary` 说明工具执行前已拦截
- `recommendations` 提醒不要通过 Agent 执行日志删除或审计清理请求

## run-eino fallback 报告

`/api/agent/run-eino` 当前仍 fallback 到稳定 runtime。报告会在 `audit_metadata.route=eino-fallback` 中记录该路径，并在 summary 中说明 fallback 没有绕过：

- intent_guard
- Planner
- semantic trace
- audit-core-py / TraceShield

## 当前限制

- 报告文本为第一版固定模板。
- Evidence chain 主要来自 tool_trace，尚未融合更多 TraceShield 原生 evidence。
- 还没有前端可视化。
- LoongArch 仍需最终验证。

## 下一步

- 将 TraceShield 原生 evidence 映射到更细的报告段落。
- 增加最终报告导出。
- 在前端展示 plan、diagnosis、security_report 和 risk graph。
- 在 Kylin V11 / LoongArch 上持续验证报告字段稳定性。
