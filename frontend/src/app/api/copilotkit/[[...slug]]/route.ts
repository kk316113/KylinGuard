import { AbstractAgent } from "@ag-ui/client";
import { EventType, type BaseEvent, type RunAgentInput } from "@ag-ui/core";
import { CopilotRuntime, InMemoryAgentRunner, createCopilotEndpoint } from "@copilotkit/runtime/v2";
import { handle } from "hono/vercel";
import { Observable } from "rxjs";
import type { AgentRun } from "@/types/agent";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

const agentBackendBase = (
  process.env.KYLIN_GUARD_AGENT_API_URL ||
  process.env.KYLINGUARD_AGENT_API_URL ||
  "http://127.0.0.1:8080"
).replace(/\/$/, "");

const backendTimeoutMs = 120_000;

class KylinGuardAgent extends AbstractAgent {
  constructor(private readonly apiBase: string) {
    super({
      agentId: "default",
      description: "KylinGuard Agent runtime adapter",
    });
  }

  run(input: RunAgentInput): Observable<BaseEvent> {
    return new Observable<BaseEvent>((subscriber) => {
      const controller = new AbortController();
      const runId = input.runId || crypto.randomUUID();
      const threadId = input.threadId || crypto.randomUUID();

      subscriber.next({ type: EventType.RUN_STARTED, runId, threadId });

      void this.execute(input, controller.signal)
        .then((run) => {
          if (controller.signal.aborted) {
            return;
          }

          const messageId = crypto.randomUUID();
          const syncToolCallId = crypto.randomUUID();
          subscriber.next({ type: EventType.STATE_SNAPSHOT, snapshot: { run } });
          if (run.run_id) {
            subscriber.next({
              type: EventType.TOOL_CALL_START,
              toolCallId: syncToolCallId,
              toolCallName: "syncKylinGuardRun",
            });
            subscriber.next({
              type: EventType.TOOL_CALL_ARGS,
              toolCallId: syncToolCallId,
              delta: JSON.stringify({ runId: run.run_id }),
            });
            subscriber.next({
              type: EventType.TOOL_CALL_END,
              toolCallId: syncToolCallId,
            });
          }
          subscriber.next({
            type: EventType.TEXT_MESSAGE_START,
            messageId,
            role: "assistant",
          });
          subscriber.next({
            type: EventType.TEXT_MESSAGE_CONTENT,
            messageId,
            delta: finalAnswer(run),
          });
          subscriber.next({ type: EventType.TEXT_MESSAGE_END, messageId });
          subscriber.next({
            type: EventType.RUN_FINISHED,
            runId,
            threadId,
            result: run,
            outcome: { type: "success" },
          });
          subscriber.complete();
        })
        .catch((error: unknown) => {
          if (controller.signal.aborted) {
            return;
          }
          subscriber.next({
            type: EventType.RUN_ERROR,
            message: error instanceof Error ? error.message : "KylinGuard Agent request failed",
            code: "KYLIN_GUARD_AGENT_ERROR",
          });
          subscriber.complete();
        });

      return () => controller.abort();
    });
  }

  override clone(): KylinGuardAgent {
    const cloned = new KylinGuardAgent(this.apiBase);
    cloned.threadId = this.threadId;
    cloned.messages = structuredClone(this.messages);
    cloned.state = structuredClone(this.state);
    return cloned;
  }

  private async execute(input: RunAgentInput, signal: AbortSignal): Promise<AgentRun> {
    const task = latestUserText(input);
    if (!task) {
      throw new Error("Message cannot be empty");
    }

    let response: Response;
    try {
      response = await fetch(`${this.apiBase}/api/agent/run`, {
        method: "POST",
        headers: { "Content-Type": "application/json; charset=utf-8" },
        body: JSON.stringify({ task, messages: conversationHistory(input) }),
        cache: "no-store",
        signal: AbortSignal.any([signal, AbortSignal.timeout(backendTimeoutMs)]),
      });
    } catch (error) {
      if (signal.aborted) {
        throw error;
      }
      throw new Error("无法连接 KylinGuard Agent 后端，请确认 Go 服务已启动。", { cause: error });
    }

    const contentType = response.headers.get("content-type") || "";
    if (!contentType.includes("application/json")) {
      const text = await response.text();
      throw new Error(text.slice(0, 240) || `Agent returned HTTP ${response.status}`);
    }

    const run = (await response.json()) as AgentRun & { error?: string };
    if (!response.ok) {
      throw new Error(run.error || `Agent returned HTTP ${response.status}`);
    }
    return run;
  }
}

function latestUserText(input: RunAgentInput): string {
  for (let index = input.messages.length - 1; index >= 0; index -= 1) {
    const message = input.messages[index];
    if (message.role === "user") {
      return messageText(message.content);
    }
  }
  return "";
}

function conversationHistory(input: RunAgentInput): Array<{ role: "user" | "assistant"; content: string }> {
  return input.messages
    .filter((message) => message.role === "user" || message.role === "assistant")
    .map((message) => ({
      role: message.role as "user" | "assistant",
      content: messageText(message.content).slice(0, 4000),
    }))
    .filter((message) => message.content.length > 0)
    .slice(-20);
}

function messageText(content: unknown): string {
  if (typeof content === "string") {
    return content.trim();
  }
  if (!Array.isArray(content)) {
    return "";
  }
  return content
    .map((part) => {
      if (typeof part === "string") {
        return part;
      }
      if (part && typeof part === "object" && "text" in part && typeof part.text === "string") {
        return part.text;
      }
      return "";
    })
    .join("\n")
    .trim();
}

function finalAnswer(run: AgentRun): string {
  return run.final_answer?.trim() || run.summary?.trim() || "未返回回答";
}

const copilotRuntime = new CopilotRuntime({
  agents: {
    default: new KylinGuardAgent(agentBackendBase),
  },
  runner: new InMemoryAgentRunner(),
});

const app = createCopilotEndpoint({
  runtime: copilotRuntime,
  basePath: "/api/copilotkit",
});

export const GET = handle(app);
export const POST = handle(app);
