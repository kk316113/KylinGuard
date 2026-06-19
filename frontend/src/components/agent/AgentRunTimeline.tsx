import { ListChecks } from "lucide-react";
import { ToolCallStepCard } from "./ToolCallStepCard";
import type { AgentRun } from "@/types/agent";

type Props = {
  run: AgentRun;
  selectedStepIndex: number | null;
  onSelectStep: (index: number) => void;
};

export function AgentRunTimeline({ run, selectedStepIndex, onSelectStep }: Props) {
  const steps = run.agent_steps || [];
  const hasTools = run.interaction_type === "agent_run" || run.needs_tool_execution || steps.length > 0;

  if (!hasTools || steps.length === 0) {
    return (
      <section className="timeline-section empty">
        <div className="section-title">
          <ListChecks size={18} />
          <h3>执行步骤</h3>
        </div>
        <p>本次响应没有执行工具。</p>
      </section>
    );
  }

  return (
    <section className="timeline-section">
      <div className="section-title">
        <ListChecks size={18} />
        <h3>执行步骤</h3>
      </div>
      <div className="step-list">
        {steps.map((step, index) => (
          <ToolCallStepCard
            key={`${step.step_index ?? index}-${step.tool_name ?? step.action_type ?? "step"}`}
            step={step}
            index={index}
            selected={selectedStepIndex === index}
            onSelect={() => onSelectStep(index)}
          />
        ))}
      </div>
    </section>
  );
}
