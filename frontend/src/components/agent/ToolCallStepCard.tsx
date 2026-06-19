import { CheckCircle2, Lock, MousePointer2 } from "lucide-react";
import { RiskDecisionBadge } from "@/components/audit/RiskDecisionBadge";
import {
  boundaryLevelLabel,
  observationSummary,
  operationTypeLabel,
  resourceTypeLabel,
  stepTitle,
} from "@/lib/formatters";
import type { AgentStep } from "@/types/agent";

type Props = {
  step: AgentStep;
  index: number;
  selected: boolean;
  onSelect: () => void;
};

export function ToolCallStepCard({ step, index, selected, onSelect }: Props) {
  const summary = step.user_visible_summary || step.reason || observationSummary(step) || "智能体完成了一个受控步骤。";
  const meta = [
    step.operation_type ? `操作：${operationTypeLabel(step.operation_type)}` : "",
    step.resource_type ? `资源：${resourceTypeLabel(step.resource_type)}` : "",
    step.boundary_level ? `边界：${boundaryLevelLabel(step.boundary_level)}` : "",
  ].filter(Boolean);

  return (
    <button className={`step-card ${selected ? "selected" : ""}`} onClick={onSelect} type="button">
      <div className="step-index">第 {step.step_index ?? index + 1} 步</div>
      <div className="step-body">
        <div className="step-title-row">
          <strong>{stepTitle(step, index)}</strong>
          <RiskDecisionBadge decision={step.policy_decision} />
        </div>
        <p>{summary}</p>
        {observationSummary(step) ? (
          <div className="observation-line">
            <CheckCircle2 size={14} />
            <span>{observationSummary(step)}</span>
          </div>
        ) : null}
        {step.policy_reason ? (
          <div className="observation-line">
            <Lock size={14} />
            <span>{step.policy_reason}</span>
          </div>
        ) : null}
        {meta.length ? <div className="step-meta">{meta.join(" / ")}</div> : null}
      </div>
      <MousePointer2 className="step-pointer" size={16} />
    </button>
  );
}
