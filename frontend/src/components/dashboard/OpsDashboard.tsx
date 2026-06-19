import { Activity, FileText, GitBranch, LayoutDashboard, ListChecks, Settings, ShieldCheck, Wrench } from "lucide-react";
import type { AgentRun } from "@/types/agent";
import type { AcceptanceSummary, CapabilitiesResponse, RuntimeStatus } from "@/types/runtime";
import { compactDate, finalAnswerOf, runtimeModeLabel, sceneTypeLabel, traceSummary } from "@/lib/formatters";
import { AgentRunTimeline } from "@/components/agent/AgentRunTimeline";
import { FinalAnswerCard } from "@/components/agent/FinalAnswerCard";
import { RiskDecisionBadge } from "@/components/audit/RiskDecisionBadge";
import { RiskGraphPanel } from "@/components/risk-graph/RiskGraphPanel";
import { RightInsightPanel } from "@/components/layout/RightInsightPanel";

export type DashboardView = "overview" | "audit" | "tools" | "runs" | "settings";

type Props = {
  activeView: DashboardView;
  runtimeStatus?: RuntimeStatus;
  capabilities?: CapabilitiesResponse;
  acceptance?: AcceptanceSummary;
  currentRun?: AgentRun | null;
  selectedStepIndex: number | null;
  onSelectStep: (index: number) => void;
  onOpenCopilot: () => void;
};

export function OpsDashboard({
  activeView,
  runtimeStatus,
  capabilities,
  acceptance,
  currentRun,
  selectedStepIndex,
  onSelectStep,
  onOpenCopilot,
}: Props) {
  return (
    <main className="dashboard-board">
      {activeView === "overview" ? (
        <OverviewBoard
          runtimeStatus={runtimeStatus}
          capabilities={capabilities}
          acceptance={acceptance}
          currentRun={currentRun}
          onOpenCopilot={onOpenCopilot}
        />
      ) : null}
      {activeView === "audit" ? (
        <AuditBoard
          currentRun={currentRun}
          selectedStepIndex={selectedStepIndex}
          onSelectStep={onSelectStep}
          onOpenCopilot={onOpenCopilot}
        />
      ) : null}
      {activeView === "tools" ? <ToolsBoard capabilities={capabilities} /> : null}
      {activeView === "runs" ? <RunsBoard currentRun={currentRun} onOpenCopilot={onOpenCopilot} /> : null}
      {activeView === "settings" ? <SettingsBoard runtimeStatus={runtimeStatus} acceptance={acceptance} /> : null}
    </main>
  );
}

function OverviewBoard({
  runtimeStatus,
  capabilities,
  acceptance,
  currentRun,
  onOpenCopilot,
}: {
  runtimeStatus?: RuntimeStatus;
  capabilities?: CapabilitiesResponse;
  acceptance?: AcceptanceSummary;
  currentRun?: AgentRun | null;
  onOpenCopilot: () => void;
}) {
  const stagesPassed = acceptance?.stages.filter((stage) => stage.status === "PASS").length || 0;
  const tools = capabilities?.available_tools || [];

  return (
    <div className="board-stack">
      <section className="board-hero">
        <div>
          <p className="eyebrow">KylinGuard Scenario Workspace</p>
          <h1>麒麟安全运维态势看板</h1>
          <p>主画布聚焦系统状态、执行证据和安全审计。自然语言对话在右下角 Copilot 抽屉中发起。</p>
        </div>
        <button className="primary-action" type="button" onClick={onOpenCopilot}>
          打开 Copilot 处理任务
        </button>
      </section>

      <section className="metric-grid">
        <MetricCard icon={<Activity size={18} />} label="运行模式" value={runtimeModeLabel(runtimeStatus?.runtime.chat_model)} />
        <MetricCard icon={<ShieldCheck size={18} />} label="安全护栏" value={`${Object.keys(runtimeStatus?.security_layers || {}).length || 0} layers`} />
        <MetricCard icon={<Wrench size={18} />} label="受控工具" value={`${tools.length || 0}`} />
        <MetricCard icon={<ListChecks size={18} />} label="验收基线" value={`${stagesPassed}/${acceptance?.stages.length || 0} PASS`} />
      </section>

      {currentRun ? (
        <section className="board-section">
          <SectionHeading icon={<FileText size={18} />} title="最近任务结果" />
          <div className="run-overview-grid">
            <div className="run-overview-main">
              <RiskDecisionBadge decision={currentRun.decision || currentRun.audit_result?.decision} />
              <h2>{currentRun.scene_summary || currentRun.task}</h2>
              <p>{finalAnswerOf(currentRun)}</p>
            </div>
            <div className="run-overview-meta">
              <Detail label="run_id" value={currentRun.run_id || currentRun.task_id} />
              <Detail label="场景" value={sceneTypeLabel(currentRun.scene_type)} />
              <Detail label="步骤" value={currentRun.agent_steps?.length || 0} />
              <Detail label="证据" value={currentRun.tool_trace?.length || 0} />
            </div>
          </div>
        </section>
      ) : (
        <section className="empty-board-state">
          <GitBranch size={28} />
          <h2>等待第一条运维任务</h2>
          <p>点击右下角 Copilot，像正常聊天一样描述问题。看板会随着 Agent 响应实时更新。</p>
          <button className="primary-action" type="button" onClick={onOpenCopilot}>
            打开 Copilot
          </button>
        </section>
      )}

      <section className="board-section">
        <SectionHeading icon={<ShieldCheck size={18} />} title="安全审计概览" />
        <RiskGraphPanel run={currentRun} />
      </section>
    </div>
  );
}

function AuditBoard({
  currentRun,
  selectedStepIndex,
  onSelectStep,
  onOpenCopilot,
}: {
  currentRun?: AgentRun | null;
  selectedStepIndex: number | null;
  onSelectStep: (index: number) => void;
  onOpenCopilot: () => void;
}) {
  if (!currentRun) {
    return (
      <div className="board-stack">
        <section className="empty-board-state">
          <ShieldCheck size={28} />
          <h2>暂无审计内容</h2>
          <p>先通过 Copilot 发起一次自然语言运维任务，审计、证据和风险图会在这里展示。</p>
          <button className="primary-action" type="button" onClick={onOpenCopilot}>
            打开 Copilot
          </button>
        </section>
      </div>
    );
  }

  return (
    <div className="board-stack">
      <FinalAnswerCard run={currentRun} />
      <AgentRunTimeline run={currentRun} selectedStepIndex={selectedStepIndex} onSelectStep={onSelectStep} />
      <RightInsightPanel
        run={currentRun}
        selectedStepIndex={selectedStepIndex}
        onSelectStep={onSelectStep}
      />
    </div>
  );
}

function ToolsBoard({ capabilities }: { capabilities?: CapabilitiesResponse }) {
  const tools = capabilities?.available_tools || [];
  return (
    <div className="board-stack">
      <section className="board-section">
        <SectionHeading icon={<Wrench size={18} />} title="受控运维工具" />
        <p className="section-copy">工具只由 Agent Loop 的 next_action 请求触发，并继续经过 Tool Policy 与 Exec Proxy。</p>
        <div className="tool-catalog">
          {tools.map((tool) => (
            <div className="tool-catalog-row" key={tool.tool_name}>
              <div>
                <strong>{tool.display_name || tool.tool_name}</strong>
                <span>{tool.description || "受控运维工具"}</span>
              </div>
              <small>{tool.operation_type} / {tool.resource_type} / {tool.boundary_level}</small>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}

function RunsBoard({ currentRun, onOpenCopilot }: { currentRun?: AgentRun | null; onOpenCopilot: () => void }) {
  if (!currentRun) {
    return (
      <div className="board-stack">
        <section className="empty-board-state">
          <FileText size={28} />
          <h2>暂无任务会话</h2>
          <p>当前阶段不做数据库持久化。完成一次 Copilot 任务后，这里会显示最近一次任务。</p>
          <button className="primary-action" type="button" onClick={onOpenCopilot}>
            发起任务
          </button>
        </section>
      </div>
    );
  }
  return (
    <div className="board-stack">
      <section className="board-section">
        <SectionHeading icon={<FileText size={18} />} title="最近任务会话" />
        <div className="detail-grid">
          <Detail label="run_id" value={currentRun.run_id || currentRun.task_id} />
          <Detail label="task" value={currentRun.task} />
          <Detail label="scene_type" value={sceneTypeLabel(currentRun.scene_type)} />
          <Detail label="run_status" value={currentRun.run_status} />
          <Detail label="created_at" value={compactDate(currentRun.created_at)} />
          <Detail label="decision" value={currentRun.decision || currentRun.audit_result?.decision} />
        </div>
      </section>
      <section className="board-section">
        <SectionHeading icon={<ListChecks size={18} />} title="工具证据摘要" />
        <div className="evidence-list">
          {(currentRun.tool_trace || []).map((trace, index) => (
            <div className="evidence-row" key={trace.step_id || `${trace.tool_name}-${index}`}>
              <strong>{trace.tool_name || `tool-${index + 1}`}</strong>
              <span>{traceSummary(trace)}</span>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}

function SettingsBoard({ runtimeStatus, acceptance }: { runtimeStatus?: RuntimeStatus; acceptance?: AcceptanceSummary }) {
  return (
    <div className="board-stack">
      <section className="board-section">
        <SectionHeading icon={<Settings size={18} />} title="运行设置" />
        <div className="detail-grid">
          <Detail label="chat_model" value={runtimeStatus?.runtime.chat_model} />
          <Detail label="provider" value={runtimeStatus?.runtime.provider} />
          <Detail label="remote_llm_used" value={runtimeStatus?.runtime.remote_llm_used} />
          <Detail label="api_key" value={runtimeStatus?.secret_safety.api_key_display || "[REDACTED]"} />
        </div>
      </section>
      <section className="board-section">
        <SectionHeading icon={<ListChecks size={18} />} title="验收状态" />
        <div className="acceptance-list">
          {(acceptance?.stages || []).map((stage) => (
            <div className="acceptance-row" key={stage.name}>
              <strong>{stage.status}</strong>
              <span>{stage.name}: {stage.title}</span>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}

function MetricCard({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <div className="metric-card">
      {icon}
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function SectionHeading({ icon, title }: { icon: React.ReactNode; title: string }) {
  return (
    <div className="section-title">
      {icon}
      <h3>{title}</h3>
    </div>
  );
}

function Detail({ label, value }: { label: string; value: unknown }) {
  return (
    <div className="detail-item">
      <span>{label}</span>
      <strong>{value === undefined || value === null || value === "" ? "无" : String(value)}</strong>
    </div>
  );
}
