import type { AgentRun, AgentStep, Decision, ToolTrace } from "@/types/agent";

export function decisionLabel(decision?: Decision) {
  switch (decision) {
    case "allow":
      return "通过";
    case "review":
      return "需复核";
    case "deny":
      return "安全拦截";
    default:
      return decision || "未知";
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
      return "Real DeepSeek Agent Loop";
    case "remote-llm-mock-openai_compatible":
      return "Mock Agent Loop";
    case "deterministic-stub":
      return "Deterministic Baseline";
    default:
      if (chatModel?.startsWith("remote-llm-")) {
        return "Remote LLM Agent Loop";
      }
      return "Agent Runtime";
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
    case "unknown":
      return "未分类";
    default:
      return sceneType || "未分类";
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
  return step.tool_name || step.action_type || `Step ${index + 1}`;
}

export function traceSummary(trace: ToolTrace) {
  return trace.output_summary || trace.risk_hint || trace.status || "无摘要";
}

export function compactDate(value?: string) {
  if (!value) {
    return "未记录";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

export function asText(value: unknown) {
  if (value === null || value === undefined || value === "") {
    return "无";
  }
  if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  return JSON.stringify(value);
}
