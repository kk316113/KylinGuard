# KylinGuard 比赛演示推荐流程

## 准备

```bash
cd /opt/kylin-guard-agent
git pull
bash deploy/kylin/install_agent_go.sh
bash scripts/linux/start_demo.sh
bash scripts/linux/check_demo.sh
```

## 演示步骤

### 1. SSH 登录异常检查 (review)

在 Agent Chat 页面：

- 点击「SSH 登录异常检查」场景卡片
- 或输入：`check SSH login anomaly`
- 选择 Eino Runtime

观察：

- ✅ Decision: review（需要审查）
- ✅ 涉及敏感日志读取（auth.log）
- ✅ Tool Policy / Exec Proxy / TraceShield 审计
- ✅ Reasoning Trace 完整展示
- 点开「执行过程」查看工具调用链
- 打开 Inspector 查看 evidence chain

### 2. 系统资源检查 (allow)

- 点击「系统资源检查」
- Stable Runtime

观察：

- ✅ Decision: allow
- ✅ 只涉及低风险只读工具
- ✅ Execution Summary 展示

### 3. 危险任务拦截 (deny)

- 点击「危险任务拦截」
- Stable Runtime

观察：

- ✅ Decision: deny
- ✅ tool_trace = 0（无工具执行）
- ✅ intent_guard 前置阻断
- ✅ Assistant 自然语言说明

### 4. Eino Runtime / Reasoning Trace

- 使用 Eino Runtime 执行任意任务
- 打开 Inspector 查看 Reasoning Trace tab
- 展示 chat_model span（provider, model, llm_enabled）
- 展示 tool_policy / exec_proxy / audit / decision_normalizer spans

### 5. Mock Remote LLM (可选)

```bash
bash scripts/linux/stop_demo.sh
DEMO_MOCK_LLM=true bash scripts/linux/start_demo.sh
```

- 在 Agent Chat 页面使用 Eino Runtime
- 观察到 chat_model → remote-llm-openai_compatible
- LLM 只生成结构化 tool plan，不直接执行
- 仍经过 Tool Policy / Exec Proxy / TraceShield

### 6. Inspector Drawer

- 在任意 assistant 消息中点击「打开 Inspector」
- Evidence tab：查看证据链
- Reasoning Trace tab：查看完整推理链路
- Raw JSON tab：查看脱敏后的原始响应

## 关键说明点

| 能力 | 说明 |
|------|------|
| Intent Guard | 危险任务前置阻断，不执行工具 |
| Tool Policy | 参数白名单校验 |
| Exec Proxy | 命令白名单、禁止 shell/sudo |
| TraceShield | 工具调用链安全审计 |
| Decision Normalizer | 只读低风险 → allow，敏感 → review |
| Reasoning Trace | 完整推理溯源 |
| Chat Model Adapter | 可切换 deterministic / remote LLM |
| Remote LLM | 只输出结构化 plan，不直接执行 |
