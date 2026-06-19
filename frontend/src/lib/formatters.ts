import type { AgentRun, AgentStep, Decision, ToolTrace } from "@/types/agent";

const toolNames: Record<string, string> = {
  disk_memory_checker: "磁盘与内存检查",
  journalctl_reader: "系统日志读取",
  log_reader: "日志读取",
  network_connection_inspector: "网络连接检查",
  os_info: "系统信息检查",
  port_checker: "端口检查",
  process_inspector: "进程检查",
  resource_usage_checker: "资源使用检查",
  safe_shell: "受控命令执行",
  service_status: "服务状态检查",
  ssh_login_analyzer: "登录日志分析",
};

const toolDescriptions: Record<string, string> = {
  disk_memory_checker: "检查磁盘空间和内存使用概况，不修改系统数据。",
  journalctl_reader: "读取指定服务近期的系统日志。",
  log_reader: "读取受控范围内的系统日志。",
  network_connection_inspector: "检查网络连接和监听状态。",
  os_info: "获取操作系统和架构等基础信息。",
  port_checker: "检查本地或远程端口是否可达。",
  process_inspector: "按名称检查进程状态。",
  resource_usage_checker: "检查系统负载和资源使用情况。",
  safe_shell: "执行安全策略允许的只读命令。",
  service_status: "检查系统服务的运行状态。",
  ssh_login_analyzer: "分析登录认证日志中的异常行为。",
};

const operationNames: Record<string, string> = {
  read: "读取",
  inspect: "检查",
  analyze: "分析",
  execute: "执行",
  write: "写入",
  delete: "删除",
};

const resourceNames: Record<string, string> = {
  disk_memory: "磁盘与内存",
  journal_log: "系统日志",
  system_log: "系统日志",
  network_connection: "网络连接",
  os_info: "系统信息",
  network_port: "网络端口",
  process: "系统进程",
  system_resource: "系统资源",
  safe_command: "受控命令",
  system_service: "系统服务",
  ssh_auth_log: "登录认证日志",
};

const boundaryNames: Record<string, string> = {
  public: "公开范围",
  low: "低风险边界",
  sensitive_system_resource: "敏感系统资源",
  restricted: "受限边界",
  high: "高风险边界",
};

export function decisionLabel(decision?: Decision | string) {
  switch (decision) {
    case "allow":
      return "通过";
    case "review":
      return "需复核";
    case "deny":
      return "安全拦截";
    default:
      return "未知";
  }
}

export function decisionTone(decision?: Decision) {
  switch (decision) {
    case "allow":
      return "good";
    case "review":
      return "warn";
    case "deny":
      return "danger";
    default:
      return "neutral";
  }
}

export function runtimeModeLabel(chatModel?: string) {
  switch (chatModel) {
    case "remote-llm-deepseek-openai_compatible":
      return "DeepSeek 智能体模式";
    case "remote-llm-mock-openai_compatible":
      return "模拟智能体模式";
    case "deterministic-stub":
      return "确定性基础模式";
    default:
      return chatModel?.startsWith("remote-llm-") ? "远程大模型模式" : "智能体运行模式";
  }
}

export function serviceStatusLabel(status?: string) {
  switch (status?.toLowerCase()) {
    case "ok":
    case "healthy":
    case "running":
      return "正常";
    case "degraded":
      return "部分可用";
    case "unavailable":
    case "offline":
    case "failed":
      return "不可用";
    default:
      return "未知";
  }
}

export function sceneTypeLabel(sceneType?: string) {
  switch (sceneType) {
    case "diagnosis":
      return "系统诊断";
    case "security_check":
      return "安全检查";
    case "service_recovery":
      return "服务恢复";
    case "system_health":
      return "系统健康";
    case "compliance_review":
      return "合规复核";
    default:
      return "未分类";
  }
}

export function runStatusLabel(status?: string) {
  switch (status) {
    case "completed":
      return "已完成";
    case "blocked":
      return "已阻止";
    case "failed":
      return "失败";
    case "partial":
      return "部分完成";
    default:
      return "未知";
  }
}

export function interactionTypeLabel(type?: string) {
  switch (type) {
    case "agent_run":
      return "智能体执行";
    case "direct_answer":
      return "直接回答";
    default:
      return "普通交互";
  }
}

export function toolNameLabel(name?: string) {
  return name ? toolNames[name] || "受控工具" : "受控工具";
}

export function toolDescriptionLabel(name?: string) {
  return name ? toolDescriptions[name] || "由安全策略约束的系统工具。" : "由安全策略约束的系统工具。";
}

export function operationTypeLabel(operation?: string) {
  return operation ? operationNames[operation] || "其他操作" : "未记录";
}

export function resourceTypeLabel(resource?: string) {
  return resource ? resourceNames[resource] || "其他资源" : "未记录";
}

export function boundaryLevelLabel(boundary?: string) {
  return boundary ? boundaryNames[boundary] || "未分类边界" : "未记录";
}

export function riskLevelLabel(level?: string) {
  switch (level?.toLowerCase()) {
    case "low":
      return "低风险";
    case "medium":
      return "中风险";
    case "high":
    case "critical":
      return "高风险";
    default:
      return "未分级";
  }
}

export function auditMethodLabel(method?: string) {
  switch (method) {
    case "traceshield":
      return "安全审计核心";
    case "intent_guard":
      return "意图安全护栏";
    case "fallback-mock":
      return "降级审计";
    default:
      return "常规审计";
  }
}

export function finalAnswerOf(run?: AgentRun | null) {
  if (!run) {
    return "";
  }
  return run.final_answer || run.user_message?.answer || run.summary || "未返回最终回答。";
}

export function observationSummary(step?: AgentStep) {
  const observation = step?.observation;
  if (!observation) {
    return "";
  }
  if (typeof observation === "string") {
    return observation;
  }
  return observation.summary || observation.deny_reason || "";
}

export function stepTitle(step: AgentStep, index: number) {
  if (step.tool_name) {
    return toolNameLabel(step.tool_name);
  }
  if (step.action_type === "final_answer") {
    return "生成最终回答";
  }
  return `执行步骤 ${index + 1}`;
}

export function traceSummary(trace: ToolTrace) {
  return trace.output_summary || trace.risk_hint || statusText(trace.status) || "无摘要";
}

export function compactDate(value?: string) {
  if (!value) {
    return "未记录";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "未记录";
  }
  return date.toLocaleString("zh-CN");
}

export function statusText(value?: string) {
  switch (value?.toLowerCase()) {
    case "success":
    case "completed":
    case "ok":
      return "成功";
    case "failed":
    case "error":
      return "失败";
    case "blocked":
    case "denied":
      return "已阻止";
    default:
      return value ? "已记录" : "未记录";
  }
}

export function asText(value: unknown) {
  if (value === null || value === undefined || value === "") {
    return "无";
  }
  if (typeof value === "boolean") {
    return value ? "是" : "否";
  }
  if (typeof value === "string" || typeof value === "number") {
    return String(value);
  }
  return JSON.stringify(value);
}
