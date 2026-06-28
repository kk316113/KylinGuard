"use client";

import { useCallback, useEffect, useState } from "react";
import {
  Activity,
  FileText,
  GitBranch,
  GitMerge,
  LayoutDashboard,
  ListChecks,
  ShieldCheck,
  Siren,
  Wrench,
} from "lucide-react";
import { AgentRunTimeline } from "@/components/agent/AgentRunTimeline";
import { FinalAnswerCard } from "@/components/agent/FinalAnswerCard";
import { RiskDecisionBadge } from "@/components/audit/RiskDecisionBadge";
import { RiskGraphPanel } from "@/components/risk-graph/RiskGraphPanel";
import type { ConsolePreferences } from "@/hooks/useConsolePreferences";
import {
  auditMethodLabel,
  boundaryLevelLabel,
  compactDate,
  decisionLabel,
  finalAnswerOf,
  operationTypeLabel,
  resourceTypeLabel,
  runStatusLabel,
  runtimeModeLabel,
  sceneTypeLabel,
  toolDescriptionLabel,
  toolNameLabel,
  traceSummary,
} from "@/lib/formatters";
import { agentReportMarkdownURL, agentRiskGraphArtifactURL, getAgentRun, getAgentRuns, type AgentRunSummary } from "@/lib/api";
import type { AgentRun } from "@/types/agent";
import type { AcceptanceSummary, CapabilitiesResponse, RuntimeStatus } from "@/types/runtime";

export type DashboardView = "overview" | "audit" | "tools" | "runs";

type Props = {
  activeView: DashboardView;
  runtimeStatus?: RuntimeStatus;
  capabilities?: CapabilitiesResponse;
  acceptance?: AcceptanceSummary;
  currentRun?: AgentRun | null;
  onSelectRun: (run: AgentRun) => void;
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
  onSelectRun,
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
        <AuditBoard
          currentRun={currentRun}
          selectedStepIndex={selectedStepIndex}
          onSelectStep={onSelectStep}
          capabilities={capabilities}
        />
      ) : null}
      {activeView === "tools" ? <ToolsBoard capabilities={capabilities} /> : null}
      {activeView === "runs" ? <RunsBoard currentRun={currentRun} onSelectRun={onSelectRun} /> : null}
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
          <p className="eyebrow">麒盾智能体控制台</p>
          <h1>系统状态与安全看板</h1>
          <p>运行状态、执行记录、工具证据和安全审计在这里集中呈现。</p>
        </div>
      </section>

      <section className="metric-grid">
        <MetricCard icon={<Activity size={18} />} label="运行模式" value={runtimeModeLabel(runtimeStatus?.runtime.chat_model)} />
        <MetricCard
          icon={<ShieldCheck size={18} />}
          label="安全层"
          value={`${Object.keys(runtimeStatus?.security_layers || {}).length || 0} 层`}
        />
        <MetricCard icon={<Wrench size={18} />} label="工具" value={`${tools.length || 0}`} />
        <MetricCard icon={<ListChecks size={18} />} label="验证结果" value={`${stagesPassed}/${acceptance?.stages.length || 0} 项通过`} />
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
              <Detail label="运行编号" value={currentRun.run_id || currentRun.task_id} />
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
        <SectionHeading icon={<GitMerge size={18} />} title="风险图" />
        <RiskGraphPanel run={currentRun} />
      </section>
    </div>
  );
}

function AuditBoard({
  currentRun,
  selectedStepIndex,
  onSelectStep,
  capabilities,
}: {
  currentRun?: AgentRun | null;
  selectedStepIndex: number | null;
  onSelectStep: (index: number) => void;
  capabilities?: CapabilitiesResponse;
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

  const violations = currentRun.audit_result?.violations || [];
  const sensitiveTraces = (currentRun.tool_trace || []).filter(
    (trace) => trace.risk_level === "high" || trace.boundary_level === "high",
  );
  const hasRisks = violations.length > 0 || sensitiveTraces.length > 0;
  const hasRiskGraph = !!(currentRun.risk_graph || currentRun.audit_result?.risk_graph);
  const runId = currentRun.run_id || currentRun.task_id;

  return (
    <div className="board-stack">
      <FinalAnswerCard run={currentRun} />

      {hasRisks ? (
        <section className="board-section">
          <SectionHeading icon={<Siren size={18} />} title="风险点" />
          <p className="section-copy">
            共检测到 <strong>{violations.length + sensitiveTraces.length}</strong> 个风险项
          </p>
          <div className="risk-hotspot-list">
            {violations.map((violation, index) => (
              <RiskHotspotCard
                key={`v-${index}`}
                severity={violation.severity || "medium"}
                label={violation.type || "安全风险"}
                message={violation.message || "检测到需要关注的风险。"}
              />
            ))}
            {sensitiveTraces.map((trace, index) => (
              <RiskHotspotCard
                key={`t-${index}`}
                severity="high"
                label={toolNameLabel(trace.tool_name)}
                message={trace.risk_hint || trace.output_summary || "检测到高边界工具调用。"}
              />
            ))}
          </div>
        </section>
      ) : (
        <section className="board-section">
          <SectionHeading icon={<ShieldCheck size={18} />} title="安全结论" />
          <p className="section-copy">
            本次任务未触发任何风险规则，所有工具调用均在安全策略允许范围内。
          </p>
        </section>
      )}

      {hasRiskGraph ? (
        <section className="board-section">
          <SectionHeading icon={<GitMerge size={18} />} title="风险图" />
          <RiskGraphPanel run={currentRun} />
        </section>
      ) : null}

      <AgentRunTimeline run={currentRun} selectedStepIndex={selectedStepIndex} onSelectStep={onSelectStep} />

      <section className="board-section">
        <SectionHeading icon={<FileText size={18} />} title="审计摘要" />
        <p className="section-copy">
          {currentRun.security_report?.executive_summary ||
            currentRun.security_report?.summary ||
            currentRun.audit_result?.message ||
            "没有额外审计说明。"}
        </p>
        <div className="detail-grid">
          <Detail
            label="审计方法"
            value={auditMethodLabel(currentRun.audit_result?.method)}
          />
          <Detail
            label="风险评分"
            value={typeof currentRun.audit_result?.risk_score === "number" ? `${Math.round(currentRun.audit_result.risk_score * 100)} 分` : "未评分"}
          />
          <Detail
            label="违规项"
            value={
              currentRun.audit_result?.violations?.length !== undefined
                ? `${currentRun.audit_result.violations.length} 项`
                : "无"
            }
          />
          <Detail
            label="证据链"
            value={
              currentRun.audit_result?.evidence_chain?.length !== undefined
                ? `${currentRun.audit_result.evidence_chain.length} 条`
                : "无"
            }
          />
        </div>
        {runId ? (
          <div className="export-actions">
            <a className="secondary-action" href={agentReportMarkdownURL(runId)} download>
              导出 Markdown 报告
            </a>
            <a className="secondary-action" href={agentRiskGraphArtifactURL(runId)} download>
              下载 Risk Graph JSON
            </a>
          </div>
        ) : null}
      </section>
    </div>
  );
}

function RiskHotspotCard({
  severity,
  label,
  message,
}: {
  severity: string;
  label: string;
  message: string;
}) {
  const tone =
    severity === "high" || severity === "critical"
      ? "danger"
      : severity === "medium"
        ? "warn"
        : "good";
  const levelLabel =
    severity === "high" || severity === "critical"
      ? "高危"
      : severity === "medium"
        ? "中危"
        : "低风险";

  return (
    <div className={`risk-hotspot-card ${tone}`}>
      <div className="risk-hotspot-badge">{levelLabel}</div>
      <div className="risk-hotspot-body">
        <strong>{label}</strong>
        <p>{message}</p>
      </div>
    </div>
  );
}

function ToolsBoard({ capabilities }: { capabilities?: CapabilitiesResponse }) {
  const tools = capabilities?.available_tools || [];
  return (
    <div className="board-stack">
      <section className="board-section">
        <SectionHeading icon={<Wrench size={18} />} title="工具能力" />
        <p className="section-copy">工具选择来自智能体规划，实际执行仍由后端安全策略控制。</p>
        <div className="tool-catalog">
          {tools.length ? (
            tools.map((tool) => (
              <div className="tool-catalog-row" key={tool.tool_name}>
                <div>
                  <strong>{toolNameLabel(tool.tool_name)}</strong>
                  <span>{toolDescriptionLabel(tool.tool_name)}</span>
                </div>
                <small>{operationTypeLabel(tool.operation_type)} / {resourceTypeLabel(tool.resource_type)} / {boundaryLevelLabel(tool.boundary_level)}</small>
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

function RunsBoard({ currentRun, onSelectRun }: { currentRun?: AgentRun | null; onSelectRun: (run: AgentRun) => void }) {
  const [runs, setRuns] = useState<AgentRunSummary[]>([]);
  const [nextCursor, setNextCursor] = useState<string | undefined>();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadRuns = useCallback(async (cursor?: string) => {
    setLoading(true);
    setError(null);
    try {
      const response = await getAgentRuns(50, cursor);
      setRuns((previous) => (cursor ? [...previous, ...response.runs] : response.runs));
      setNextCursor(response.next_cursor);
    } catch (err) {
      setError(err instanceof Error ? friendlyAPIError(err.message) : "历史会话加载失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadRuns();
  }, [loadRuns]);

  const openRun = useCallback(
    async (runId: string) => {
      setLoading(true);
      setError(null);
      try {
        onSelectRun(await getAgentRun(runId));
      } catch (err) {
        setError(err instanceof Error ? friendlyAPIError(err.message) : "会话详情加载失败");
      } finally {
        setLoading(false);
      }
    },
    [onSelectRun],
  );

  return (
    <div className="board-stack">
      <section className="board-section">
        <div className="section-heading-row">
          <SectionHeading icon={<FileText size={18} />} title="历史会话" />
          <button className="secondary-action" type="button" onClick={() => void loadRuns()} disabled={loading}>
            {loading ? "刷新中..." : "刷新"}
          </button>
        </div>
        <p className="section-copy">历史会话来自后端持久化存储，服务重启后仍可继续查看执行证据。</p>
        {error ? <div className="inline-error">{error}</div> : null}
        <div className="run-history-list">
          {runs.length ? (
            runs.map((run) => {
              const selected = currentRun?.run_id === run.run_id;
              return (
                <button
                  className={selected ? "run-history-row selected" : "run-history-row"}
                  key={run.run_id}
                  type="button"
                  onClick={() => void openRun(run.run_id)}
                >
                  <span>{compactDate(run.created_at)}</span>
                  <strong>{run.task || "未命名任务"}</strong>
                  <small>
                    {decisionLabel(run.decision)} · {runStatusLabel(run.run_status)} · {run.agent_step_count} 步 / {run.tool_trace_count} 条证据
                  </small>
                  <em>{run.chat_model || run.agent_mode || "未知模型"}</em>
                </button>
              );
            })
          ) : (
            <EmptyInline>{loading ? "正在读取历史会话..." : "暂无持久化历史。完成一次对话后会自动写入。"} </EmptyInline>
          )}
        </div>
        {nextCursor ? (
          <button className="secondary-action" type="button" onClick={() => void loadRuns(nextCursor)} disabled={loading}>
            加载更多
          </button>
        ) : null}
      </section>
      {currentRun ? (
        <>
          <section className="board-section">
            <SectionHeading icon={<FileText size={18} />} title="当前选中会话" />
            <div className="detail-grid">
              <Detail label="运行编号" value={currentRun.run_id || currentRun.task_id} />
              <Detail label="用户输入" value={currentRun.task} />
              <Detail label="任务类型" value={sceneTypeLabel(currentRun.scene_type)} />
              <Detail label="运行状态" value={runStatusLabel(currentRun.run_status)} />
              <Detail label="创建时间" value={compactDate(currentRun.created_at)} />
              <Detail label="安全结论" value={decisionLabel(currentRun.decision || currentRun.audit_result?.decision)} />
            </div>
          </section>
          <section className="board-section">
            <SectionHeading icon={<ListChecks size={18} />} title="工具证据摘要" />
            <div className="evidence-list">
              {(currentRun.tool_trace || []).length ? (
                (currentRun.tool_trace || []).map((trace, index) => (
                  <div className="evidence-row" key={trace.step_id || `${trace.tool_name}-${index}`}>
                    <strong>{trace.tool_name ? toolNameLabel(trace.tool_name) : `工具 ${index + 1}`}</strong>
                    <span>{traceSummary(trace)}</span>
                  </div>
                ))
              ) : (
                <EmptyInline>本次响应没有工具证据。</EmptyInline>
              )}
            </div>
          </section>
        </>
      ) : null}
    </div>
  );
}

function friendlyAPIError(message: string) {
  if (message.includes("AGENT_TIMEOUT") || message.includes("504")) {
    return "智能体请求超时：DeepSeek 或后端工具调用耗时过长，请稍后重试。";
  }
  if (message.includes("AGENT_UNAVAILABLE") || message.includes("fetch failed")) {
    return "Agent 服务不可达：请检查 kylin-guard-agent 是否正在运行。";
  }
  if (message.includes("ECONNRESET") || message.includes("socket hang up")) {
    return "前端代理连接被重置：请刷新状态或重启 Web 服务后重试。";
  }
  return message;
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
