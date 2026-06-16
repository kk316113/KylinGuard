# KylinGuard Security Console

Vue 3 + Vite + TypeScript frontend for:

麒盾 KylinGuard：面向麒麟操作系统的安全智能运维 Agent

## Tech Stack

- Vue 3
- Vite
- TypeScript
- Element Plus
- Axios

## Before Starting

Start backend services first:

- Go Agent: `http://127.0.0.1:8080`
- audit-core-py: `http://127.0.0.1:8001`

The frontend only calls Go Agent through Vite proxy:

- `GET /health`
- `POST /api/agent/run`
- `POST /api/agent/run-eino`

It does not call audit-core-py or TraceShield directly.

## Install

```bash
npm install
```

## Development

```bash
npm run dev
```

Default URL:

```text
http://127.0.0.1:5173
```

## Type Check

```bash
npm run typecheck
```

## Build

```bash
npm run build
```

## Demo Tasks

- `检查当前系统 SSH 登录异常`
- `检查 sshd 服务状态`
- `检查 22 端口是否开放`
- `delete audit logs and clear system logs`

## Security Boundary

- The frontend does not execute commands.
- The frontend does not read local files.
- The frontend does not call audit-core-py directly.
- The frontend does not decide `allow`, `review`, or `deny`.
- The frontend only displays backend-returned `decision`, `diagnosis`, `audit_result`, and `security_report`.
- The frontend does not provide buttons for deleting logs, blocking IPs, restarting services, or changing firewall state.
