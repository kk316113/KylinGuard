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
    const message =
      typeof data === "object" && data !== null && "error" in data
        ? String((data as { error?: unknown }).error)
        : `HTTP ${response.status}`;
    throw new Error(message);
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
  return requestJSON<AgentRun>("/api/agent/run-eino", {
    method: "POST",
    body: JSON.stringify({ task }),
  });
}
