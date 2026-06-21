import type { AgentRun } from "@/types/agent";
import type { AcceptanceSummary, CapabilitiesResponse, RuntimeStatus } from "@/types/runtime";

const publicBase = (process.env.NEXT_PUBLIC_API_BASE_URL || "").replace(/\/$/, "");

function endpoint(path: string) {
  if (!publicBase) {
    return path;
  }
  if (publicBase.endsWith("/api") && path.startsWith("/api/")) {
    return `${publicBase}${path.slice(4)}`;
  }
  return `${publicBase}${path}`;
}

async function requestJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(endpoint(path), {
    ...init,
    headers: {
      "Content-Type": "application/json; charset=utf-8",
      ...(init?.headers || {}),
    },
  });

  const contentType = response.headers.get("content-type") || "";
  if (!contentType.includes("application/json")) {
    const text = await response.text();
    throw new Error(text.slice(0, 240) || `Unexpected non-JSON response from ${path}`);
  }

  const data = (await response.json()) as T;
  if (!response.ok) {
    const payload = data as { error?: unknown };
    const structured =
      typeof payload?.error === "object" && payload.error !== null
        ? (payload.error as { code?: unknown; message?: unknown })
        : null;
    const message =
      typeof structured?.message === "string"
        ? structured.message
        : typeof payload?.error === "string"
          ? payload.error
          : `HTTP ${response.status}`;
    const code = typeof structured?.code === "string" ? structured.code : `HTTP_${response.status}`;
    throw new Error(`${code}: ${message}`);
  }
  return data;
}

export function getRuntimeStatus() {
  return requestJSON<RuntimeStatus>("/api/agent/runtime-status", { method: "GET" });
}

export function getCapabilities() {
  return requestJSON<CapabilitiesResponse>("/api/agent/capabilities", { method: "GET" });
}

export function getAcceptanceSummary() {
  return requestJSON<AcceptanceSummary>("/api/agent/acceptance-summary", { method: "GET" });
}

export function runAgentTask(task: string) {
  return requestJSON<AgentRun>("/api/agent/run", {
    method: "POST",
    body: JSON.stringify({ task }),
  });
}

export function getAgentRun(runId: string) {
  return requestJSON<AgentRun>(`/api/agent/runs/${encodeURIComponent(runId)}`, { method: "GET" });
}

export function getAgentAuditReports(runId: string) {
  return requestJSON(`/api/agent/runs/${encodeURIComponent(runId)}/audit-reports`, { method: "GET" });
}

export function getAgentRiskGraph(runId: string) {
  return requestJSON(`/api/agent/runs/${encodeURIComponent(runId)}/risk-graph`, { method: "GET" });
}

export function getAgentReport(runId: string) {
  return requestJSON(`/api/agent/runs/${encodeURIComponent(runId)}/report`, { method: "GET" });
}
