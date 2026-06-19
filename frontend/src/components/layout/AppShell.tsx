"use client";

import { useCallback, useEffect, useState } from "react";
import { getAcceptanceSummary, getCapabilities, getRuntimeStatus } from "@/lib/api";
import type { AgentRun } from "@/types/agent";
import type { AcceptanceSummary, CapabilitiesResponse, RuntimeStatus } from "@/types/runtime";
import { AgentConsole } from "@/components/agent/AgentConsole";
import { RightInsightPanel } from "./RightInsightPanel";
import { TopStatusBar } from "./TopStatusBar";

export function AppShell() {
  const [runtimeStatus, setRuntimeStatus] = useState<RuntimeStatus>();
  const [capabilities, setCapabilities] = useState<CapabilitiesResponse>();
  const [acceptance, setAcceptance] = useState<AcceptanceSummary>();
  const [statusLoading, setStatusLoading] = useState(false);
  const [statusError, setStatusError] = useState<string | null>(null);
  const [currentRun, setCurrentRun] = useState<AgentRun | null>(null);
  const [selectedStepIndex, setSelectedStepIndex] = useState<number | null>(null);

  const loadShellData = useCallback(async () => {
    setStatusLoading(true);
    setStatusError(null);
    try {
      const [runtime, caps, summary] = await Promise.all([
        getRuntimeStatus(),
        getCapabilities(),
        getAcceptanceSummary(),
      ]);
      setRuntimeStatus(runtime);
      setCapabilities(caps);
      setAcceptance(summary);
    } catch (err) {
      setStatusError(err instanceof Error ? err.message : "状态加载失败");
    } finally {
      setStatusLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadShellData();
  }, [loadShellData]);

  function handleRunUpdate(run: AgentRun) {
    setCurrentRun(run);
    setSelectedStepIndex(run.agent_steps?.length ? 0 : null);
  }

  return (
    <div className="app-shell">
      <TopStatusBar
        status={runtimeStatus}
        loading={statusLoading}
        error={statusError}
        onRefresh={() => void loadShellData()}
      />
      <div className="workspace-grid">
        <AgentConsole
          runtimeStatus={runtimeStatus}
          currentRun={currentRun}
          selectedStepIndex={selectedStepIndex}
          onRunUpdate={handleRunUpdate}
          onSelectStep={setSelectedStepIndex}
        />
        <RightInsightPanel
          run={currentRun}
          selectedStepIndex={selectedStepIndex}
          onSelectStep={setSelectedStepIndex}
          capabilities={capabilities}
          acceptance={acceptance}
        />
      </div>
    </div>
  );
}
