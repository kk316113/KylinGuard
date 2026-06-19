"use client";

import { useCallback, useEffect, useState } from "react";
import { FileText, LayoutDashboard, ListChecks, MessageCircle, Settings, ShieldCheck, Wrench } from "lucide-react";
import { getAcceptanceSummary, getCapabilities, getRuntimeStatus } from "@/lib/api";
import type { AgentRun } from "@/types/agent";
import type { AcceptanceSummary, CapabilitiesResponse, RuntimeStatus } from "@/types/runtime";
import { CopilotTaskDrawer } from "@/components/agent/CopilotTaskDrawer";
import { DashboardView, OpsDashboard } from "@/components/dashboard/OpsDashboard";
import { TopStatusBar } from "./TopStatusBar";

const navItems: Array<{ key: DashboardView; label: string; icon: React.ReactNode }> = [
  { key: "overview", label: "态势看板", icon: <LayoutDashboard size={17} /> },
  { key: "audit", label: "安全审计", icon: <ShieldCheck size={17} /> },
  { key: "tools", label: "工具能力", icon: <Wrench size={17} /> },
  { key: "runs", label: "任务会话", icon: <FileText size={17} /> },
  { key: "settings", label: "运行设置", icon: <Settings size={17} /> },
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
  const [copilotOpen, setCopilotOpen] = useState(false);

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
    setActiveView("overview");
  }

  return (
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
            <span>Workspace</span>
            <strong>麒盾工作台</strong>
          </div>
          <nav className="sidebar-nav" aria-label="工作台导航">
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
            <button className="copilot-nav-button" type="button" onClick={() => setCopilotOpen(true)}>
              <MessageCircle size={16} />
              打开 Copilot
            </button>
            <div className="sidebar-note">
              <ListChecks size={14} />
              <span>自然语言任务不会变成固定 workflow。</span>
            </div>
          </div>
        </aside>

        <OpsDashboard
          activeView={activeView}
          runtimeStatus={runtimeStatus}
          capabilities={capabilities}
          acceptance={acceptance}
          currentRun={currentRun}
          selectedStepIndex={selectedStepIndex}
          onSelectStep={setSelectedStepIndex}
          onOpenCopilot={() => setCopilotOpen(true)}
        />
      </div>

      <CopilotTaskDrawer open={copilotOpen} onOpenChange={setCopilotOpen} onRunUpdate={handleRunUpdate} />
    </div>
  );
}
