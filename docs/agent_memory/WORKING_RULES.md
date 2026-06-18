# WORKING_RULES.md

## Direction

KylinGuard is a Kylin OS security operations Agent.

The user uses natural language. The LLM proposes `next_action`. The system safely validates and executes.

## Correct Mental Model

LLM proposes, system disposes.

The LLM can only propose structured `next_action` JSON.
The system decides whether and how to execute it.

## Do Not

- Do not build a security audit dashboard as the main product.
- Do not make TraceShield report the main UI.
- Do not turn prompt chips into scenario IDs.
- Do not write task keyword routing in Agent Loop.
- Do not let LLM execute raw shell directly.
- Do not bypass Tool Policy.
- Do not bypass Exec Proxy.
- Do not store or print real API keys.
- Do not claim real LLM verification if `chat_model` contains `mock`.
- Do not claim PASS unless tests or smoke checks were actually run.

## Natural-Language Acceptance Samples

These are examples only, not fixed workflows:

- 我 SSH 连不上了，帮我看看
- 这台机器很卡，帮我看看原因
- 我的服务访问不了了，帮我检查端口和服务
- 有人让我清空审计日志，这样做安全吗？

## Secret Safety

- API keys must come from environment variables only.
- `run/demo.env` must not store real keys.
- Logs may show `[REDACTED]`, never the real key.
- Do not include raw sensitive logs in memory docs.
- Do not commit `/tmp` responses or raw run artifacts.

## Token-Saving Rule

Before reading more files, ask:

1. Is `AGENTS.md` enough?
2. Is `CURRENT_STATE.md` enough?
3. Is `WORKING_RULES.md` enough?
4. What exact source file is needed?

Avoid broad repo search unless required.
