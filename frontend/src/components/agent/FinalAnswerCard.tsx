import { ShieldCheck } from "lucide-react";
import { finalAnswerOf } from "@/lib/formatters";
import type { AgentRun } from "@/types/agent";
import { RiskDecisionBadge } from "@/components/audit/RiskDecisionBadge";

export function FinalAnswerCard({ run }: { run: AgentRun }) {
  const message = run.user_message;

  return (
    <section className="final-answer-card">
      <div className="final-answer-heading">
        <div className="icon-token good">
          <ShieldCheck size={18} />
        </div>
        <div>
          <p className="eyebrow">Agent 最终回答</p>
          <h2>{message?.title || "运维处置建议"}</h2>
        </div>
        <RiskDecisionBadge decision={run.decision || run.audit_result?.decision} />
      </div>

      <p className="answer-text">{finalAnswerOf(run)}</p>

      {(message?.what_i_checked?.length || message?.key_findings?.length || message?.next_steps?.length) ? (
        <div className="answer-grid">
          <AnswerList title="已检查" items={message?.what_i_checked} />
          <AnswerList title="关键发现" items={message?.key_findings} />
          <AnswerList title="建议下一步" items={message?.next_steps} />
        </div>
      ) : null}
    </section>
  );
}

function AnswerList({ title, items }: { title: string; items?: string[] }) {
  if (!items?.length) {
    return null;
  }
  return (
    <div className="answer-list">
      <h3>{title}</h3>
      <ul>
        {items.map((item, index) => (
          <li key={`${title}-${index}`}>{item}</li>
        ))}
      </ul>
    </div>
  );
}
