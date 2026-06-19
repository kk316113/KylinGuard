import {
  Activity,
  FileText,
  GitBranch,
  LayoutDashboard,
  ListChecks,
  RotateCcw,
  Settings,
  ShieldCheck,
  Wrench,
} from "lucide-react";
import { AgentRunTimeline } from "@/components/agent/AgentRunTimeline";
import { FinalAnswerCard } from "@/components/agent/FinalAnswerCard";
import { RiskDecisionBadge } from "@/components/audit/RiskDecisionBadge";
import { RightInsightPanel } from "@/components/layout/RightInsightPanel";
import { RiskGraphPanel } from "@/components/risk-graph/RiskGraphPanel";
import type { ConsolePreferences } from "@/hooks/useConsolePreferences";
import { compactDate, finalAnswerOf, runtimeModeLabel, sceneTypeLabel, traceSummary } from "@/lib/formatters";
import type { AgentRun } from "@/types/agent";
import type { AcceptanceSummary, CapabilitiesResponse, RuntimeStatus } from "@/types/runtime";

export type DashboardView = "overview" | "audit" | "tools" | "runs" | "settings";

type Props = {
  activeView: DashboardView;
  runtimeStatus?: RuntimeStatus;
  capabilities?: CapabilitiesResponse;
  acceptance?: AcceptanceSummary;
  currentRun?: AgentRun | null;
  selectedStepIndex: number | null;
  onSelectStep: (index: number) => void;
  preferences: ConsolePreferences;
  onUpdatePreferences: (patch: Partial<ConsolePreferences>) => void;
  onResetPreferences: () => void;
};

export function OpsDashboard({
  activeView,
  runtimeStatus,
  capabilities,
  acceptance,
  currentRun,
  selectedStepIndex,
  onSelectStep,
  preferences,
  onUpdatePreferences,
  onResetPreferences,
}: Props) {
  return (
    <main className="dashboard-board">
      {activeView === "overview" ? (
        <OverviewBoard
          runtimeStatus={runtimeStatus}
          capabilities={capabilities}
          acceptance={acceptance}
          currentRun={currentRun}
        />
      ) : null}
      {activeView === "audit" ? (
        <AuditBoard currentRun={currentRun} selectedStepIndex={selectedStepIndex} onSelectStep={onSelectStep} />
      ) : null}
      {activeView === "tools" ? <ToolsBoard capabilities={capabilities} /> : null}
      {activeView === "runs" ? <RunsBoard currentRun={currentRun} /> : null}
      {activeView === "settings" ? (
        <SettingsBoard
          runtimeStatus={runtimeStatus}
          acceptance={acceptance}
          preferences={preferences}
          onUpdatePreferences={onUpdatePreferences}
          onResetPreferences={onResetPreferences}
        />
      ) : null}
    </main>
  );
}

function OverviewBoard({
  runtimeStatus,
  capabilities,
  acceptance,
  currentRun,
}: {
  runtimeStatus?: RuntimeStatus;
  capabilities?: CapabilitiesResponse;
  acceptance?: AcceptanceSummary;
  currentRun?: AgentRun | null;
}) {
  const stagesPassed = acceptance?.stages.filter((stage) => stage.status === "PASS").length || 0;
  const tools = capabilities?.available_tools || [];

  return (
    <div className="board-stack">
      <section className="board-hero">
        <div>
          <p className="eyebrow">KylinGuard Agent Console</p>
          <h1>系统状态与安全看板</h1>
          <p>运行状态、执行记录、工具证据和安全审计在这里集中呈现。</p>
        </div>
      </section>

      <section className="metric-grid">
        <MetricCard icon={<Activity size={18} />} label="运行模式" value={runtimeModeLabel(runtimeStatus?.runtime.chat_model)} />
        <MetricCard
          icon={<ShieldCheck size={18} />}
          label="安全层"
          value={`${Object.keys(runtimeStatus?.security_layers || {}).length || 0} layers`}
        />
        <MetricCard icon={<Wrench size={18} />} label="工具" value={`${tools.length || 0}`} />
        <MetricCard icon={<ListChecks size={18} />} label="验收" value={`${stagesPassed}/${acceptance?.stages.length || 0} PASS`} />
      </section>

      {currentRun ? (
        <section className="board-section">
          <SectionHeading icon={<FileText size={18} />} title="最近结果" />
          <div className="run-overview-grid">
            <div className="run-overview-main">
              <RiskDecisionBadge decision={currentRun.decision || currentRun.audit_result?.decision} />
              <h2>{currentRun.scene_summary || currentRun.task || "最近会话"}</h2>
              <p>{finalAnswerOf(currentRun)}</p>
            </div>
            <div className="run-overview-meta">
              <Detail label="run_id" value={currentRun.run_id || currentRun.task_id} />
              <Detail label="分类" value={sceneTypeLabel(currentRun.scene_type)} />
              <Detail label="步骤" value={currentRun.agent_steps?.length || 0} />
              <Detail label="证据" value={currentRun.tool_trace?.length || 0} />
            </div>
          </div>
        </section>
      ) : (
        <section className="empty-board-state">
          <GitBranch size={28} />
          <h2>暂无会话数据</h2>
          <p>聊天完成后，回答、执行步骤、工具证据和审计结果会自动同步到看板。</p>
        </section>
      )}

      <section className="board-section">
        <SectionHeading icon={<ShieldCheck size={18} />} title="风险图" />
        <RiskGraphPanel run={currentRun} />
      </section>
    </div>
  );
}

function AuditBoard({
  currentRun,
  selectedStepIndex,
  onSelectStep,
}: {
  currentRun?: AgentRun | null;
  selectedStepIndex: number | null;
  onSelectStep: (index: number) => void;
}) {
  if (!currentRun) {
    return (
      <div className="board-stack">
        <section className="empty-board-state">
          <ShieldCheck size={28} />
          <h2>暂无审计内容</h2>
          <p>任务完成后，安全结论、证据链和策略判定会显示在这里。</p>
        </section>
      </div>
    );
  }

  return (
    <div className="board-stack">
      <FinalAnswerCard run={currentRun} />
      <AgentRunTimeline run={currentRun} selectedStepIndex={selectedStepIndex} onSelectStep={onSelectStep} />
      <RightInsightPanel run={currentRun} selectedStepIndex={selectedStepIndex} onSelectStep={onSelectStep} />
    </div>
  );
}

function ToolsBoard({ capabilities }: { capabilities?: CapabilitiesResponse }) {
  const tools = capabilities?.available_tools || [];
  return (
    <div className="board-stack">
      <section className="board-section">
        <SectionHeading icon={<Wrench size={18} />} title="工具能力" />
        <p className="section-copy">工具选择来自 Agent Loop，实际执行仍由后端安全策略控制。</p>
        <div className="tool-catalog">
          {tools.length ? (
            tools.map((tool) => (
              <div className="tool-catalog-row" key={tool.tool_name}>
                <div>
                  <strong>{tool.display_name || tool.tool_name}</strong>
                  <span>{tool.description || "受控工具"}</span>
                </div>
                <small>{tool.operation_type} / {tool.resource_type} / {tool.boundary_level}</small>
              </div>
            ))
          ) : (
            <EmptyInline>暂无工具数据</EmptyInline>
          )}
        </div>
      </section>
    </div>
  );
}

function RunsBoard({ currentRun }: { currentRun?: AgentRun | null }) {
  if (!currentRun) {
    return (
      <div className="board-stack">
        <section className="empty-board-state">
          <FileText size={28} />
          <h2>暂无最近会话</h2>
          <p>当前阶段只展示最近一次响应，不做本地历史持久化。</p>
        </section>
      </div>
    );
  }

  return (
    <div className="board-stack">
      <section className="board-section">
        <SectionHeading icon={<FileText size={18} />} title="最近会话" />
        <div className="detail-grid">
          <Detail label="run_id" value={currentRun.run_id || currentRun.task_id} />
          <Detail label="input" value={currentRun.task} />
          <Detail label="scene_type" value={sceneTypeLabel(currentRun.scene_type)} />
          <Detail label="run_status" value={currentRun.run_status} />
          <Detail label="created_at" value={compactDate(currentRun.created_at)} />
          <Detail label="decision" value={currentRun.decision || currentRun.audit_result?.decision} />
        </div>
      </section>
      <section className="board-section">
        <SectionHeading icon={<ListChecks size={18} />} title="工具证据摘要" />
        <div className="evidence-list">
          {(currentRun.tool_trace || []).length ? (
            (currentRun.tool_trace || []).map((trace, index) => (
              <div className="evidence-row" key={trace.step_id || `${trace.tool_name}-${index}`}>
                <strong>{trace.tool_name || `tool-${index + 1}`}</strong>
                <span>{traceSummary(trace)}</span>
              </div>
            ))
          ) : (
            <EmptyInline>本次响应没有工具证据。</EmptyInline>
          )}
        </div>
      </section>
    </div>
  );
}

function SettingsBoard({
  runtimeStatus,
  acceptance,
  preferences,
  onUpdatePreferences,
  onResetPreferences,
}: {
  runtimeStatus?: RuntimeStatus;
  acceptance?: AcceptanceSummary;
  preferences: ConsolePreferences;
  onUpdatePreferences: (patch: Partial<ConsolePreferences>) => void;
  onResetPreferences: () => void;
}) {
  return (
    <div className="board-stack">
      <section className="board-section settings-section">
        <div className="settings-heading">
          <div>
            <SectionHeading icon={<Settings size={18} />} title="界面设置" />
            <p className="section-copy">更改会立即生效，并保存在当前浏览器。</p>
          </div>
          <button className="icon-button" type="button" onClick={onResetPreferences} title="恢复默认设置" aria-label="恢复默认设置">
            <RotateCcw size={16} />
          </button>
        </div>

        <div className="settings-list">
          <SettingRow label="外观" description="跟随系统或指定浅色、深色主题。">
            <div className="segmented-control" role="group" aria-label="外观主题">
              {(["system", "light", "dark"] as const).map((theme) => (
                <button
                  key={theme}
                  type="button"
                  aria-pressed={preferences.theme === theme}
                  className={preferences.theme === theme ? "active" : ""}
                  onClick={() => onUpdatePreferences({ theme })}
                >
                  {theme === "system" ? "跟随系统" : theme === "light" ? "浅色" : "深色"}
                </button>
              ))}
            </div>
          </SettingRow>

          <SettingRow label="聊天位置" description="使用 CopilotKit 官方侧边栏，选择从左侧或右侧展开。">
            <div className="segmented-control" role="group" aria-label="聊天侧边栏位置">
              {(["left", "right"] as const).map((chatPosition) => (
                <button
                  key={chatPosition}
                  type="button"
                  aria-pressed={preferences.chatPosition === chatPosition}
                  className={preferences.chatPosition === chatPosition ? "active" : ""}
                  onClick={() => onUpdatePreferences({ chatPosition })}
                >
                  {chatPosition === "left" ? "左侧" : "右侧"}
                </button>
              ))}
            </div>
          </SettingRow>

          <SettingRow label="聊天宽度" description={`${preferences.chatWidth}px`}>
            <div className="range-control">
              <span>360</span>
              <input
                type="range"
                min={360}
                max={640}
                step={20}
                value={preferences.chatWidth}
                onChange={(event) => onUpdatePreferences({ chatWidth: Number(event.target.value) })}
                aria-label="聊天侧边栏宽度"
              />
              <span>640</span>
            </div>
          </SettingRow>

          <SettingRow label="默认展开聊天" description="页面加载后直接打开 CopilotKit 聊天侧边栏。">
            <label className="switch-control">
              <input
                type="checkbox"
                checked={preferences.chatDefaultOpen}
                onChange={(event) => onUpdatePreferences({ chatDefaultOpen: event.target.checked })}
              />
              <span aria-hidden="true" />
              <strong>{preferences.chatDefaultOpen ? "已开启" : "已关闭"}</strong>
            </label>
          </SettingRow>
        </div>
      </section>

      <section className="board-section">
        <SectionHeading icon={<Activity size={18} />} title="运行状态" />
        <p className="section-copy">模型与凭据由服务端配置，此处仅显示脱敏后的只读状态。</p>
        <div className="detail-grid">
          <Detail label="chat_model" value={runtimeStatus?.runtime.chat_model} />
          <Detail label="provider" value={runtimeStatus?.runtime.provider} />
          <Detail label="remote_llm_used" value={runtimeStatus?.runtime.remote_llm_used} />
          <Detail label="api_key" value={runtimeStatus?.secret_safety.api_key_display || "[REDACTED]"} />
        </div>
      </section>

      <section className="board-section">
        <SectionHeading icon={<ListChecks size={18} />} title="验收基线" />
        <div className="acceptance-list">
          {(acceptance?.stages || []).length ? (
            (acceptance?.stages || []).map((stage) => (
              <div className="acceptance-row" key={stage.name}>
                <strong>{stage.status}</strong>
                <span>{stage.name}: {stage.title}</span>
              </div>
            ))
          ) : (
            <EmptyInline>暂无验收数据</EmptyInline>
          )}
        </div>
      </section>
    </div>
  );
}

function SettingRow({ label, description, children }: { label: string; description: string; children: React.ReactNode }) {
  return (
    <div className="setting-row">
      <div>
        <strong>{label}</strong>
        <span>{description}</span>
      </div>
      {children}
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

function EmptyInline({ children }: { children: React.ReactNode }) {
  return <div className="empty-inline">{children}</div>;
}
