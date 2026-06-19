"use client";

import { FormEvent, useState } from "react";
import { Bot, ChevronDown, Loader2, MessageCircle, Send, Sparkles, X } from "lucide-react";
import { runAgentTask } from "@/lib/api";
import { suggestedPrompts } from "@/lib/constants";
import { compactDate, finalAnswerOf } from "@/lib/formatters";
import type { AgentRun, ConversationMessage } from "@/types/agent";
import { RiskDecisionBadge } from "@/components/audit/RiskDecisionBadge";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onRunUpdate: (run: AgentRun) => void;
};

export function CopilotTaskDrawer({ open, onOpenChange, onRunUpdate }: Props) {
  const [input, setInput] = useState("");
  const [messages, setMessages] = useState<ConversationMessage[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submitTask(taskText: string) {
    const task = taskText.trim();
    if (!task || loading) {
      return;
    }
    setInput("");
    setError(null);
    setMessages((prev) => [
      ...prev,
      {
        id: crypto.randomUUID(),
        role: "user",
        content: task,
        createdAt: new Date().toISOString(),
      },
    ]);
    setLoading(true);
    try {
      const run = await runAgentTask(task);
      onRunUpdate(run);
      setMessages((prev) => [
        ...prev,
        {
          id: crypto.randomUUID(),
          role: "assistant",
          content: finalAnswerOf(run),
          createdAt: new Date().toISOString(),
          run,
        },
      ]);
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
    <>
      {!open ? (
        <button className="copilot-launcher" type="button" onClick={() => onOpenChange(true)}>
          <MessageCircle size={18} />
          <span>打开 Copilot</span>
        </button>
      ) : null}

      <aside className={`copilot-drawer ${open ? "open" : ""}`} aria-label="KylinGuard Copilot Chat">
        <header className="copilot-drawer-header">
          <div className="drawer-agent-mark">
            <Bot size={18} />
          </div>
          <div>
            <strong>KylinGuard Copilot</strong>
            <span>自然语言运维入口</span>
          </div>
          <button className="icon-button" type="button" onClick={() => onOpenChange(false)} aria-label="收起 Copilot">
            <ChevronDown size={16} />
          </button>
          <button className="icon-button close-mobile" type="button" onClick={() => onOpenChange(false)} aria-label="关闭 Copilot">
            <X size={16} />
          </button>
        </header>

        <div className="copilot-messages">
          {messages.length === 0 ? (
            <div className="copilot-welcome">
              <Bot size={26} />
              <h2>直接描述你的运维问题</h2>
              <p>我会调用受控工具完成排查，并把结果同步到左侧看板。</p>
            </div>
          ) : (
            messages.map((message) => (
              <article key={message.id} className={`chat-bubble ${message.role}`}>
                <div className="chat-bubble-meta">
                  <span>{message.role === "user" ? "你" : "KylinGuard"}</span>
                  <time>{compactDate(message.createdAt)}</time>
                  {message.run ? <RiskDecisionBadge decision={message.run.decision || message.run.audit_result?.decision} /> : null}
                </div>
                <p>{message.content}</p>
              </article>
            ))
          )}
          {loading ? (
            <div className="chat-bubble assistant pending">
              <Loader2 className="spin" size={16} />
              <p>Agent 正在理解任务并等待受控工具返回 observation...</p>
            </div>
          ) : null}
          {error ? <div className="copilot-error">{error}</div> : null}
        </div>

        <div className="copilot-suggestions" aria-label="自然语言示例">
          {suggestedPrompts.slice(0, 3).map((prompt) => (
            <button key={prompt} type="button" onClick={() => void submitTask(prompt)} disabled={loading}>
              <Sparkles size={13} />
              {prompt}
            </button>
          ))}
        </div>

        <form className="copilot-composer" onSubmit={handleSubmit}>
          <textarea
            value={input}
            onChange={(event) => setInput(event.target.value)}
            placeholder="输入自然语言运维任务..."
            rows={2}
          />
          <button type="submit" disabled={!input.trim() || loading} aria-label="发送任务">
            {loading ? <Loader2 className="spin" size={16} /> : <Send size={16} />}
          </button>
        </form>
      </aside>
    </>
  );
}
