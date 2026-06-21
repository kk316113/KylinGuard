# 中国软件杯 A2 赛题交付规划

## 1. 目标与验收口径

权威赛题来源：[A2-面向麒麟操作系统的安全智能运维 Agent 设计与实现](https://www.cnsoftbei.com/content-15-1247-1.html)，发布时间 2026-03-23。

KylinGuard 的最终验收环境为 LoongArch 架构的麒麟高级服务器操作系统 V11。产品必须以 B/S 方式提供自然语言运维 Agent，并证明以下闭环真实成立：

```text
自然语言任务 -> OS 实时感知 -> LLM 结构化决策 -> 安全校验
-> 最小权限执行 -> 观察反馈 -> 根因分析/最终回答 -> 全链路溯源
```

评分优先级按赛题原文执行：功能完整性 55%，创新与实用性 25%，文档与演示 20%。任何仅在 Windows 或 mock 模式下通过的能力都不能作为最终平台验收证据。

## 2. 要求追踪与当前差距

| 赛题要求 | 当前证据 | 当前结论 | 完成门槛 |
|---|---|---|---|
| OS 深度感知 | 已有进程、网络、journal、服务、磁盘/内存、SSH 等 Tools | 基础具备 | 在麒麟 V11 实机覆盖典型故障并保存脱敏验收摘要 |
| MCP 运维插件化 | 已有 Tool Registry；新增官方 Go SDK Streamable HTTP `/mcp` | 本地实现 | MCP 客户端完成 initialize、tools/list、tools/call；调用全程经过 Tool Policy、Exec Proxy、trace/audit |
| 安全意图校验 | intent_guard、Tool Policy、参数白名单、危险命令禁止 | 基础具备 | 补齐提示词注入语料与未授权配置修改的自动化拒绝测试 |
| 最小权限代理 | Exec Proxy 禁 shell/sudo、命令白名单、超时和输出限制 | 部分具备 | 以专用非 root 账户运行；systemd sandbox 与文件/能力白名单可验证 |
| 推理链路溯源 | reasoning_trace、tool_trace、逐步审计、风险图 | 基础具备 | 每个真实工具调用均可由 run_id 查询，拒绝调用也有未执行证据 |
| 确定性与可靠性 | 当前工具以只读诊断为主 | 部分具备 | 关键配置写入默认禁止；失败、超时、重试、并发均有测试 |
| 抗提示词注入 | 参数注入和部分危险意图已拦截 | 明显缺口 | 建立直接/间接注入测试集，证明不能绕过工具 schema、Policy、Exec Proxy |
| LoongArch + 麒麟 V11 | Go 后端可交叉编译 `linux/loong64` | 待实机证明 | 安装、启动、核心场景、资源占用、重启恢复均在指定 VM 通过 |
| 根因分析能力 | LLM Agent Loop 可组合多步观察 | 部分具备 | 僵尸进程、磁盘 I/O、配置漂移至少三类场景给出证据化根因与建议 |
| 文档与演示 | 有产品/API/阶段文档 | 未齐套 | 完成赛题要求的九类提交物及不超过 7 分钟视频 |

## 3. 分阶段实施

### M1：标准 MCP 与安全调用闭环

- 使用官方 `modelcontextprotocol/go-sdk` 提供 Streamable HTTP `/mcp`。
- MCP 只发布 Registry 中启用、非危险且允许直调的 Tool；输入由 JSON Schema 校验。
- 每次调用必须先过 Tool Policy；允许调用沿用现有 Tool/Exec Proxy；允许与拒绝结果都进入 trace/audit。
- 验收：官方客户端可列举/调用工具；注入参数在处理器执行前拒绝；`go test ./...` 和 `linux/loong64` 交叉编译通过。

### M2：最小权限部署与抗注入强化

- 提供专用 `kylinguard` 系统账户、systemd unit、安全目录及最小可读资源清单。
- 配置 `NoNewPrivileges`、私有临时目录、受限写路径和必要 capability；默认不授予 root/sudo。
- 建立提示词注入与越权测试集，覆盖对话注入、Tool 参数注入、日志内间接注入和关键配置写入诱导。
- 验收：攻击样例无危险命令执行、无关键文件变化，拒绝原因和证据可追溯。

### M3：OS 感知与智能根因分析

- 补齐赛题点名或场景需要的 lsof/文件占用、僵尸进程、磁盘 I/O、配置漂移只读感知工具。
- 由 LLM 根据实时观察自主选择工具，不增加关键词固定工作流。
- 将跨工具时间线、资源关联和风险边界汇总进最终回答与风险图。
- 验收：至少三类故障注入场景在真实麒麟节点完成“发现—定位—建议—安全说明”闭环。

### M4：LoongArch/V11 发布与质量门禁

- 产出可复现构建脚本、版本化安装包、离线依赖说明、启动/停止/升级/卸载流程。
- 在指定 VM 验证后端、前端、DeepSeek/Qwen 国产模型接入和一键演示。
- 测量核心接口延迟、Agent 完成时延、并发稳定性、内存/CPU、超时与恢复，并固定测试环境和样本量。
- 验收：麒麟 V11 实机 smoke、功能测试、性能测试、重启恢复全部通过，无真实密钥进入仓库或日志。

### M5：初赛交付与答辩

- 形成需求分析、功能设计、产品说明、功能测试、性能测试、部署文档六类正式文档。
- 生成安装包、源代码压缩包、演示 PPT 和不超过 7 分钟的 MP4 视频。
- 演示脚本按评分权重突出自然语言准确性、真实 OS 感知、MCP 插件、护栏拦截、根因分析和 LoongArch 实机证据。
- 验收：九项材料逐项检查，可在全新麒麟 V11 环境按文档复现。

## 4. 首批实现记录

本轮已完成 M1 的代码落地：

- 后端增加官方 MCP Streamable HTTP 入口 `/mcp`。
- 动态复用现有 Tool Registry JSON Schema，不发布 `safe_shell` 等禁止直调工具。
- MCP 工具调用复用 Tool Policy、Registry/Exec Proxy、trace store 和 TraceShield audit client。
- 自动化测试覆盖工具发现、允许调用、HTTP 协议协商以及注入参数执行前拒绝。
- Go 基线升级到 1.23；Windows 本地全量测试通过，`CGO_ENABLED=0 GOOS=linux GOARCH=loong64` 构建通过。

仍需在麒麟 V11/LoongArch VM 上执行运行时验证后，M1 才能标记最终 PASS。

M2 已开始：仓库新增专用 `kylinguard` 非 root 账户的 systemd unit 和安装器，默认仅监听 loopback，并启用 capability、文件系统、设备、内核及命名空间限制。当前仅完成脚本语法验证；必须在麒麟 V11 上确认所有只读感知工具在该沙箱内正常工作，才能把最小权限要求判定为达成。

抗注入第一批加固也已完成：Intent Guard 将直接提示词注入标记为独立 `prompt_injection` 威胁；历史消息和工具观察中的注入指令在进入下一轮 LLM 前被中和，但原始证据仍保留于 audit trace；Tool Policy 拒绝未知参数与嵌套 shell 注入；Exec Proxy 对 `systemctl` 强制只读子命令，并将 `cat` 限定到少量公开系统事实文件。Linux 实机验收入口为 `scripts/linux/test_security_guardrails.sh`。

M3 已开始：`open_file_inspector` 补齐赛题点名的 lsof 感知，`process_inspector` 增加僵尸进程过滤与堆积风险，`disk_io_checker` 增加真实 `/proc/diskstats` 双采样指标。三者共享 Registry、Tool Policy、Exec Proxy、trace/audit 和 MCP 发布链路，未增加关键词固定工作流。麒麟实机验收入口为 `scripts/linux/test_os_sensing_tools.sh`。

2026-06-21 已在麒麟高级服务器 V11（Swan25）x86_64 VM 完成标准 MCP、四类护栏攻击、真实 lsof 文件持有者、僵尸进程和磁盘 I/O 采样验收，后端以普通用户运行且临时服务已停止。该结果证明 Kylin V11 兼容性，但不能替代赛题指定的 LoongArch 最终验收；真实 DeepSeek 多步 Agent 也需在目标 VM 环境另行复验。
