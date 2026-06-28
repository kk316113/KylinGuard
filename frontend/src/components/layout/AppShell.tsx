"use client";

import { useCallback, useEffect, useState } from "react";
import { useFrontendTool } from "@copilotkit/react-core/v2";
import { FileText, LayoutDashboard, ListChecks, ShieldCheck, Wrench } from "lucide-react";
import { z } from "zod";
import { OpsDashboard, type DashboardView } from "@/components/dashboard/OpsDashboard";
import { getAcceptanceSummary, getAgentRun, getCapabilities, getRuntimeStatus } from "@/lib/api";
import type { AgentRun } from "@/types/agent";
import type { AcceptanceSummary, CapabilitiesResponse, RuntimeStatus } from "@/types/runtime";
import { TopStatusBar } from "./TopStatusBar";
import { AppDrawer } from "./AppDrawer";

const navItems: Array<{ key: DashboardView; label: string; icon: React.ReactNode }> = [
  { key: "overview", label: "总览", icon: <LayoutDashboard size={17} /> },
  { key: "audit", label: "审计", icon: <ShieldCheck size={17} /> },
  { key: "tools", label: "工具", icon: <Wrench size={17} /> },
  { key: "runs", label: "会话", icon: <FileText size={17} /> },
];

export function AppShell() {
  const [runtimeStatus, setRuntimeStatus] = useState<RuntimeStatus>();
  const [capabilities, setCapabilities] = useState<CapabilitiesResponse>();
  const [acceptance, setAcceptance] = useState<AcceptanceSummary>();
  const [statusLoading, setStatusLoading] = useState(false);
  const [statusError, setStatusError] = useState<string | null>(null);
  const [currentRun, setCurrentRun] = useState<AgentRun | null>(null);
  const [selectedStepIndex, setSelectedStepIndex] = useState<number | null>(null);
  const [activeView, setActiveView] = useState<DashboardView>("overview");

  useFrontendTool({
    name: "syncKylinGuardRun",
    description: "Synchronize the completed KylinGuard run with the dashboard.",
    parameters: z.object({
      runId: z.string().describe("Completed KylinGuard run identifier."),
    }),
    followUp: false,
    render: () => <></>,
    handler: async ({ runId }) => {
      const run = await getAgentRun(runId);
      setCurrentRun(run);
      setSelectedStepIndex(run.agent_steps?.length ? 0 : null);
      setActiveView("overview");
      return "Dashboard synchronized";
    },
  });

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

  return (
    <div className="copilot-product-root" data-copilotkit>
      <div className="app-shell">
        <TopStatusBar
          status={runtimeStatus}
          loading={statusLoading}
          error={statusError}
          onRefresh={() => void loadShellData()}
        />

        <div className="product-workspace">
          <aside className="left-sidebar">
            <div className="sidebar-title">
              <span>安全智能体</span>
              <strong>麒盾控制台</strong>
            </div>

            <nav className="sidebar-nav" aria-label="控制台导航">
              {navItems.map((item) => (
                <button
                  key={item.key}
                  type="button"
                  className={activeView === item.key ? "active" : ""}
                  onClick={() => setActiveView(item.key)}
                >
                  {item.icon}
                  <span>{item.label}</span>
                </button>
              ))}
            </nav>

            <div className="sidebar-footer">
              <div className="sidebar-note">
                <ListChecks size={14} />
                <span>执行记录、安全审计和工具证据会在任务完成后同步到控制台。</span>
              </div>
            </div>
          </aside>

          <OpsDashboard
            activeView={activeView}
            runtimeStatus={runtimeStatus}
            capabilities={capabilities}
            acceptance={acceptance}
            currentRun={currentRun}
            onSelectRun={(run) => {
              setCurrentRun(run);
              setSelectedStepIndex(run.agent_steps?.length ? 0 : null);
              setActiveView("overview");
            }}
            selectedStepIndex={selectedStepIndex}
            onSelectStep={setSelectedStepIndex}
          />
        </div>
      </div>

      <AppDrawer />
    </div>
  );
}
