"use client";

import { forwardRef, useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  CopilotChatToggleButton,
  CopilotSidebar,
  useCopilotChatConfiguration,
  useFrontendTool,
} from "@copilotkit/react-core/v2";
import { FileText, LayoutDashboard, ListChecks, Settings, ShieldCheck, Wrench } from "lucide-react";
import { z } from "zod";
import { DashboardView, OpsDashboard } from "@/components/dashboard/OpsDashboard";
import { useConsolePreferences } from "@/hooks/useConsolePreferences";
import { getAcceptanceSummary, getAgentRun, getCapabilities, getRuntimeStatus } from "@/lib/api";
import type { AgentRun } from "@/types/agent";
import type { AcceptanceSummary, CapabilitiesResponse, RuntimeStatus } from "@/types/runtime";
import { TopStatusBar } from "./TopStatusBar";

const navItems: Array<{ key: DashboardView; label: string; icon: React.ReactNode }> = [
  { key: "overview", label: "总览", icon: <LayoutDashboard size={17} /> },
  { key: "audit", label: "审计", icon: <ShieldCheck size={17} /> },
  { key: "tools", label: "工具", icon: <Wrench size={17} /> },
  { key: "runs", label: "会话", icon: <FileText size={17} /> },
  { key: "settings", label: "设置", icon: <Settings size={17} /> },
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
  const { preferences, updatePreferences, resetPreferences, hydrated } = useConsolePreferences();

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

  const sidebarKey = hydrated
    ? `${preferences.chatPosition}-${preferences.chatWidth}-${preferences.chatDefaultOpen}`
    : "copilot-sidebar-loading";
  const SidebarToggleButton = useMemo(() => {
    const OfficialSidebarToggle = forwardRef<
      HTMLButtonElement,
      React.ComponentPropsWithoutRef<typeof CopilotChatToggleButton>
    >((props, ref) => (
      <SidebarOpenStateSync
        desiredOpen={preferences.chatDefaultOpen}
        toggleProps={props}
        toggleRef={ref}
      />
    ));
    OfficialSidebarToggle.displayName = "OfficialCopilotSidebarToggle";
    return OfficialSidebarToggle;
  }, [preferences.chatDefaultOpen]);

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
            selectedStepIndex={selectedStepIndex}
            onSelectStep={setSelectedStepIndex}
            preferences={preferences}
            onUpdatePreferences={updatePreferences}
            onResetPreferences={resetPreferences}
          />
        </div>
      </div>

      {hydrated ? (
        <CopilotSidebar
          key={sidebarKey}
          agentId="default"
          defaultOpen={preferences.chatDefaultOpen}
          position={preferences.chatPosition}
          width={preferences.chatWidth}
          toggleButton={SidebarToggleButton}
          labels={{
            modalHeaderTitle: "麒盾",
            chatInputPlaceholder: "输入消息...",
            welcomeMessageText: "你好，我是麒盾。",
            chatDisclaimerText: "回答由智能体生成，请核对重要信息。",
          }}
        />
      ) : null}
    </div>
  );
}

function SidebarOpenStateSync({
  desiredOpen,
  toggleProps,
  toggleRef,
}: {
  desiredOpen: boolean;
  toggleProps: React.ComponentPropsWithoutRef<typeof CopilotChatToggleButton>;
  toggleRef: React.ForwardedRef<HTMLButtonElement>;
}) {
  const configuration = useCopilotChatConfiguration();
  const synchronized = useRef(false);

  useEffect(() => {
    if (!configuration || synchronized.current) {
      return;
    }

    const frame = window.requestAnimationFrame(() => {
      synchronized.current = true;
      configuration.setModalOpen(desiredOpen);
    });
    return () => window.cancelAnimationFrame(frame);
  }, [configuration?.setModalOpen, desiredOpen]);

  return <CopilotChatToggleButton ref={toggleRef} {...toggleProps} />;
}
