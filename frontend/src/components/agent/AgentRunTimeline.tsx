import { ListChecks } from "lucide-react";
import type { AgentRun } from "@/types/agent";
import { ToolCallStepCard } from "./ToolCallStepCard";

type Props = {
  run: AgentRun;
  selectedStepIndex: number | null;
  onSelectStep: (index: number) => void;
};

export function AgentRunTimeline({ run, selectedStepIndex, onSelectStep }: Props) {
  const steps = run.agent_steps || [];
  const isToolRun = run.interaction_type === "agent_run" || run.needs_tool_execution;

  if (!isToolRun || steps.length === 0) {
    return (
      <section className="timeline-section empty">
        <div className="section-title">
          <ListChecks size={18} />
          <h3>执行过程</h3>
        </div>
        <p>本次交互没有调用系统工具，因此没有工具执行时间线。</p>
      </section>
    );
  }

  return (
    <section className="timeline-section">
      <div className="section-title">
        <ListChecks size={18} />
        <h3>Agent 执行步骤</h3>
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
