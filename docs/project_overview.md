# Project Overview

麒盾 KylinGuard 是面向麒麟操作系统的安全智能运维 Agent。Stage 1 的目标是在 Stage 0 工程骨架之上，将清洗后的 TraceShield 论文审计核心封装为稳定的 audit-core-py HTTP 服务。

## Stage 1 范围

- 保持 Go Agent 服务通过 HTTP 调用 audit-core-py。
- 引入 TraceShield adapter，不直接修改 TraceShield-Core。
- 定义统一审计输入输出结构。
- 保留 fallback mock，保障 audit-core-py 不可用时接口稳定。
- 继续保持跨平台、跨架构实现思路，避免重依赖和 x86-only 二进制依赖。

## 非目标

- 不重写 TraceShield 论文方法。
- 不接入本地大模型。
- 不引入 torch、transformers、faiss、sentence-transformers 等重依赖。
- 不实现前端。
