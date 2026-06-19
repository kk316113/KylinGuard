import type { AgentRun } from "@/types/agent";
import type { RuntimeStatus } from "@/types/runtime";

type Props = {
  runtimeStatus?: RuntimeStatus;
  currentRun?: AgentRun | null;
  selectedStepIndex: number | null;
  onRunUpdate: (run: AgentRun) => void;
  onSelectStep: (index: number) => void;
};

// Kept only as a compatibility export for older imports. The current product
// UI uses OpsDashboard as the board and CopilotKit's native sidebar for chat.
export function AgentConsole(_props: Props) {
  return null;
}
