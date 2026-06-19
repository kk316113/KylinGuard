import type { AgentRun } from "@/types/agent";
import type { RuntimeStatus } from "@/types/runtime";

type Props = {
  runtimeStatus?: RuntimeStatus;
  currentRun?: AgentRun | null;
  selectedStepIndex: number | null;
  onRunUpdate: (run: AgentRun) => void;
  onSelectStep: (index: number) => void;
};

// Kept only as a compatibility export for older imports. The product UI now
// uses OpsDashboard as the main board and CopilotTaskDrawer as the chat entry.
export function AgentConsole(_props: Props) {
  return null;
}
