"use client";

import { FormEvent, useMemo, useState } from "react";
import { ArrowUp, Loader2, MessageSquarePlus, Sparkles } from "lucide-react";
import { runAgentTask } from "@/lib/api";
import { suggestedPrompts } from "@/lib/constants";
import { compactDate, finalAnswerOf, runtimeModeLabel, sceneTypeLabel } from "@/lib/formatters";
import type { AgentRun, ConversationMessage } from "@/types/agent";
import type { RuntimeStatus } from "@/types/runtime";
import { AgentRunTimeline } from "./AgentRunTimeline";
import { FinalAnswerCard } from "./FinalAnswerCard";

type Props = {
  runtimeStatus?: RuntimeStatus;
  currentRun?: AgentRun | null;
  selectedStepIndex: number | null;
  onRunUpdate: (run: AgentRun) => void;
  onSelectStep: (index: number) => void;
};

export function AgentConsole({ runtimeStatus, currentRun, selectedStepIndex, onRunUpdate, onSelectStep }: Props) {
  const [input, setInput] = useState("");
  const [messages, setMessages] = useState<ConversationMessage[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const modeLabel = useMemo(() => runtimeModeLabel(runtimeStatus?.runtime.chat_model), [runtimeStatus]);

  async function submitTask(taskText: string) {
    const task = taskText.trim();
    if (!task || loading) {
      return;
    }
    setError(null);
    setInput("");
    const userMessage: ConversationMessage = {
      id: crypto.randomUUID(),
      role: "user",
      content: task,
      createdAt: new Date().toISOString(),
    };
    setMessages((prev) => [...prev, userMessage]);
    setLoading(true);
    try {
      const run = await runAgentTask(task);
      onRunUpdate(run);
      const assistantMessage: ConversationMessage = {
        id: crypto.randomUUID(),
        role: "assistant",
        content: finalAnswerOf(run),
        createdAt: new Date().toISOString(),
        run,
      };
      setMessages((prev) => [...prev, assistantMessage]);
    } catch (err) {
      const message = err instanceof Error ? err.message : "请求失败";
      setError(message);
      setMessages((prev) => [
        ...prev,
        {
          id: crypto.randomUUID(),
          role: "assistant",
          content: `后端服务暂不可用或返回异常：${message}`,
          createdAt: new Date().toISOString(),
        },
      ]);
    } finally {
      setLoading(false);
    }
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    void submitTask(input);
  }

  return (
    <main className="agent-console">
      <section className="conversation-panel">
        <div className="console-hero">
          <p className="eyebrow">KylinGuard CopilotKit Agent Console</p>
          <h1>用自然语言发起安全受控的麒麟运维任务</h1>
          <p>
            Agent 会理解你的问题，必要时调用受控工具，并把最终回答、工具证据和安全审计分层展示。
          </p>
        </div>

        <div className="prompt-rail" aria-label="推荐运维入口">
          {suggestedPrompts.map((prompt) => (
            <button key={prompt} type="button" onClick={() => void submitTask(prompt)} disabled={loading}>
              <Sparkles size={14} />
              <span>{prompt}</span>
            </button>
          ))}
        </div>

        <div className="messages">
          {messages.length === 0 ? (
            <div className="empty-dialog">
              <MessageSquarePlus size={28} />
              <h2>从一个真实运维问题开始</h2>
              <p>例如 SSH 无法连接、机器变慢、服务访问不了，或者有人要求执行高风险操作。</p>
            </div>
          ) : (
            messages.map((message) => (
              <article key={message.id} className={`message ${message.role}`}>
                <div className="message-meta">
                  <span>{message.role === "user" ? "你" : "KylinGuard Agent"}</span>
                  <time>{compactDate(message.createdAt)}</time>
                </div>
                {message.run ? <FinalAnswerCard run={message.run} /> : <p>{message.content}</p>}
              </article>
            ))
          )}
          {loading ? (
            <div className="running-state">
              <Loader2 className="spin" size={18} />
              <div>
                <strong>Agent 正在处理任务</strong>
                <span>理解任务、规划 next_action、等待安全护栏和工具 observation。</span>
              </div>
            </div>
          ) : null}
          {error ? <div className="inline-error">{error}</div> : null}
        </div>

        {currentRun ? (
          <div className="run-summary-strip">
            <span>任务会话：{currentRun.task_id || "未分配"}</span>
            <span>场景类型：{sceneTypeLabel(currentRun.scene_type)}</span>
            <span>运行状态：{currentRun.run_status || "unknown"}</span>
            <span>运行模式：{modeLabel}</span>
          </div>
        ) : null}

        {currentRun ? (
          <AgentRunTimeline run={currentRun} selectedStepIndex={selectedStepIndex} onSelectStep={onSelectStep} />
        ) : null}
      </section>

      <form className="task-composer" onSubmit={handleSubmit}>
        <textarea
          value={input}
          onChange={(event) => setInput(event.target.value)}
          placeholder="输入你的运维问题，例如：我 SSH 连不上了，帮我看看"
          rows={2}
        />
        <button type="submit" disabled={!input.trim() || loading} aria-label="发送任务">
          {loading ? <Loader2 className="spin" size={18} /> : <ArrowUp size={18} />}
        </button>
      </form>
    </main>
  );
}
