import { decisionLabel, decisionTone } from "@/lib/formatters";
import type { Decision } from "@/types/agent";

export function RiskDecisionBadge({ decision }: { decision?: Decision }) {
  return (
    <span className={`decision-badge ${decisionTone(decision)}`}>
      {decisionLabel(decision)}
    </span>
  );
}
