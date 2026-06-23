# KylinGuard CopilotKit Frontend

This is the rebuilt KylinGuard Agent Console based on Next.js, React, TypeScript, and CopilotKit.

The MVP uses KylinGuard's existing Go Agent APIs:

- `GET /api/agent/runtime-status`
- `GET /api/agent/capabilities`
- `GET /api/agent/acceptance-summary`
- `POST /api/agent/run`

CopilotKit is used as the Agent UX foundation. The current MVP calls the
non-streaming Go Agent endpoint directly; AG-UI event streaming can be added
later if the backend exposes a streaming endpoint.

## Run

```bash
npm install
npm run dev
```

The dev server listens on:

```text
http://127.0.0.1:5173
```

By default, Next.js rewrites `/api/agent/*` to `http://127.0.0.1:8080/api/agent/*`.
Override the Go Agent target with `KYLIN_GUARD_AGENT_API_URL`.

The frontend is only the browser UI. For useful data, start the Go Agent on
`127.0.0.1:8080` first. The audit-core service on `127.0.0.1:8001` is
recommended; if it is unavailable, the Agent returns a clearly marked local
safety fallback rather than a fabricated TraceShield result.

## Safety

- Do not store real API keys in frontend env files.
- Do not expose raw LLM credentials to the browser.
- `decision=deny` is rendered as a safe guardrail outcome, not as a frontend error.
- The frontend only displays backend-provided audit reports and risk graph data.
