"use client";

import { useMemo, useState } from "react";
import type { ReactNode } from "react";
import { FileText, GitBranch, ListChecks, Shield, Siren, Wrench } from "lucide-react";
import { RiskDecisionBadge } from "@/components/audit/RiskDecisionBadge";
import { RiskGraphPanel } from "@/components/risk-graph/RiskGraphPanel";
import {
  asText,
  auditMethodLabel,
  boundaryLevelLabel,
  compactDate,
  decisionLabel,
  interactionTypeLabel,
  observationSummary,
  operationTypeLabel,
  resourceTypeLabel,
  riskLevelLabel,
  runStatusLabel,
  sceneTypeLabel,
  toolNameLabel,
  traceSummary,
} from "@/lib/formatters";
import type { AgentRun, AgentStep, ToolTrace } from "@/types/agent";
import type { CapabilitiesResponse } from "@/types/runtime";

type TabKey = "audit" | "risk" | "hotspots" | "decision" | "tools" | "report";

type Props = {
  run?: AgentRun | null;
  selectedStepIndex: number | null;
  onSelectStep: (index: number) => void;
  capabilities?: CapabilitiesResponse;
};

const tabs: Array<{ key: TabKey; label: string; icon: ReactNode }> = [
  { key: "audit", label: "审计", icon: <Shield size={15} /> },
  { key: "risk", label: "风险图", icon: <GitBranch size={15} /> },
  { key: "hotspots", label: "风险点", icon: <Siren size={15} /> },
  { key: "decision", label: "执行路径", icon: <ListChecks size={15} /> },
  { key: "tools", label: "工具", icon: <Wrench size={15} /> },
  { key: "report", label: "报告", icon: <FileText size={15} /> },
];

export function RightInsightPanel({ run, selectedStepIndex, onSelectStep, capabilities }: Props) {
  const [activeTab, setActiveTab] = useState<TabKey>("audit");
  const selectedStep = useMemo(() => {
    if (!run?.agent_steps?.length || selectedStepIndex === null) {
      return undefined;
    }
    return run.agent_steps[selectedStepIndex];
  }, [run, selectedStepIndex]);
  const selectedTrace = useMemo(
    () => findTraceForStep(run, selectedStepIndex, selectedStep),
    [run, selectedStepIndex, selectedStep],
  );

  return (
    <section className="insight-panel">
      <nav className="insight-tabs" aria-label="任务洞察导航">
        {tabs.map((tab) => (
          <button
            key={tab.key}
            type="button"
            className={activeTab === tab.key ? "active" : ""}
            onClick={() => setActiveTab(tab.key)}
          >
            {tab.icon}
            <span>{tab.label}</span>
          </button>
        ))}
      </nav>

      <div className="insight-body">
        {activeTab === "audit" ? <AuditTab run={run} step={selectedStep} trace={selectedTrace} /> : null}
        {activeTab === "risk" ? <RiskGraphPanel run={run} /> : null}
        {activeTab === "hotspots" ? <HotspotsTab run={run} /> : null}
        {activeTab === "decision" ? (
          <DecisionPathTab run={run} selectedStepIndex={selectedStepIndex} onSelectStep={onSelectStep} />
        ) : null}
        {activeTab === "tools" ? <ToolsTab capabilities={capabilities} /> : null}
        {activeTab === "report" ? <ReportTab run={run} /> : null}
      </div>
    </section>
  );
}

function AuditTab({ run, step, trace }: { run?: AgentRun | null; step?: AgentStep; trace?: ToolTrace }) {
  if (!run) {
    return <EmptyPanel title="等待审计数据" description="任务完成后，这里会展示安全审计摘要。" />;
  }

  const score = run.audit_result?.risk_score;
  return (
    <div className="insight-stack">
      <section className="insight-section">
        <div className="section-title">
          <Shield size={17} />
          <h3>整体结论</h3>
        </div>
        <div className="audit-summary">
          <RiskDecisionBadge decision={run.decision || run.audit_result?.decision} />
          <span>审计方式：{auditMethodLabel(run.audit_result?.method)}</span>
          <span>风险评分：{typeof score === "number" ? `${Math.round(score * 100)} 分` : "未评分"}</span>
        </div>
        <p>{run.audit_result?.message || run.security_report?.summary || "没有额外审计说明。"}</p>
      </section>

      <section className="insight-section">
        <div className="section-title">
          <ListChecks size={17} />
          <h3>当前步骤</h3>
        </div>
        {step || trace ? (
          <div className="detail-grid">
            <Detail label="工具" value={toolNameLabel(step?.tool_name || trace?.tool_name)} />
            <Detail label="策略结论" value={decisionLabel(step?.policy_decision)} />
            <Detail label="操作类型" value={operationTypeLabel(step?.operation_type || trace?.operation_type)} />
            <Detail label="资源类型" value={resourceTypeLabel(step?.resource_type || trace?.resource_type)} />
            <Detail label="安全边界" value={boundaryLevelLabel(step?.boundary_level || trace?.boundary_level)} />
            <Detail label="执行结果" value={observationSummary(step) || (trace ? traceSummary(trace) : "")} wide />
          </div>
        ) : (
          <p>本次任务没有可选择的工具步骤，仅展示整体审计结论。</p>
        )}
      </section>

      {run.audit_result?.violations?.length ? (
        <section className="insight-section">
          <div className="section-title">
            <Siren size={17} />
            <h3>风险项</h3>
          </div>
          <ul className="compact-list">
            {run.audit_result.violations.map((violation, index) => (
              <li key={`${violation.type || "violation"}-${index}`}>
                <strong>{riskLevelLabel(violation.severity)}</strong>
                <span>{violation.message || "检测到需要关注的风险。"}</span>
              </li>
            ))}
          </ul>
        </section>
      ) : null}
    </div>
  );
}

function HotspotsTab({ run }: { run?: AgentRun | null }) {
  if (!run) {
    return <EmptyPanel title="暂无风险点" description="风险点来自后端审计结果，前端不会自行推断。" />;
  }
  const violations = run.audit_result?.violations || [];
  const sensitiveTraces = (run.tool_trace || []).filter(
    (trace) => trace.risk_level === "high" || trace.boundary_level === "high",
  );

  if (!violations.length && !sensitiveTraces.length) {
    return <EmptyPanel title="未发现高风险项" description="当前响应没有返回高风险违规项或敏感工具证据。" />;
  }

  return (
    <div className="insight-stack">
      {violations.map((violation, index) => (
        <section className="hotspot-item" key={`${violation.type || "violation"}-${index}`}>
          <strong>{riskLevelLabel(violation.severity)}</strong>
          <p>{violation.message || "检测到需要关注的风险。"}</p>
        </section>
      ))}
      {sensitiveTraces.map((trace, index) => (
        <section className="hotspot-item" key={`${trace.step_id || "trace"}-${index}`}>
          <strong>{toolNameLabel(trace.tool_name)}</strong>
          <p>{trace.risk_hint || trace.output_summary || "检测到高边界工具调用。"}</p>
        </section>
      ))}
    </div>
  );
}

function DecisionPathTab({
  run,
  selectedStepIndex,
  onSelectStep,
}: {
  run?: AgentRun | null;
  selectedStepIndex: number | null;
  onSelectStep: (index: number) => void;
}) {
  if (!run?.agent_steps?.length) {
    return <EmptyPanel title="暂无执行路径" description="任务包含工具步骤时，这里会展示每一步的策略结论和执行结果。" />;
  }

  return (
    <div className="decision-path">
      {run.agent_steps.map((step, index) => (
        <button
          type="button"
          key={`${step.step_index ?? index}-${step.tool_name ?? step.action_type}`}
          className={selectedStepIndex === index ? "active" : ""}
          onClick={() => onSelectStep(index)}
        >
          <span>第 {step.step_index ?? index + 1} 步</span>
          <strong>{toolNameLabel(step.tool_name)}</strong>
          <RiskDecisionBadge decision={step.policy_decision} />
          <small>{observationSummary(step) || step.reason || "等待执行结果"}</small>
        </button>
      ))}
    </div>
  );
}

function ToolsTab({ capabilities }: { capabilities?: CapabilitiesResponse }) {
  const tools = capabilities?.available_tools || [];

  return (
    <div className="insight-stack">
      <section className="insight-section">
        <div className="section-title">
          <Wrench size={17} />
          <h3>工具清单</h3>
        </div>
        <p>{tools.length ? `当前后端提供 ${tools.length} 个受控工具。` : "尚未加载工具能力。"}</p>
        <div className="tool-list">
          {tools.slice(0, 16).map((tool) => (
            <div className="tool-row" key={tool.tool_name}>
              <strong>{toolNameLabel(tool.tool_name)}</strong>
              <span>
                {operationTypeLabel(tool.operation_type)} / {resourceTypeLabel(tool.resource_type)} / {boundaryLevelLabel(tool.boundary_level)}
              </span>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}

function ReportTab({ run }: { run?: AgentRun | null }) {
  if (!run) {
    return <EmptyPanel title="暂无报告摘要" description="任务完成后，这里会展示会话信息和最终审计摘要。" />;
  }

  return (
    <div className="insight-stack">
      <section className="insight-section">
        <div className="section-title">
          <FileText size={17} />
          <h3>会话信息</h3>
        </div>
        <div className="detail-grid">
          <Detail label="运行编号" value={run.run_id || run.task_id} />
          <Detail label="任务类型" value={sceneTypeLabel(run.scene_type)} />
          <Detail label="运行状态" value={runStatusLabel(run.run_status)} />
          <Detail label="创建时间" value={compactDate(run.created_at)} />
          <Detail label="交互方式" value={interactionTypeLabel(run.interaction_type)} />
          <Detail label="路由来源" value={routerSourceLabel(run.router_source)} />
          <Detail label="路由说明" value={run.router_reason || "无"} wide />
        </div>
      </section>

      <section className="insight-section">
        <div className="section-title">
          <Shield size={17} />
          <h3>审计摘要</h3>
        </div>
        <p>{run.security_report?.executive_summary || run.security_report?.summary || run.audit_result?.message || "没有额外报告摘要。"}</p>
        {run.security_report?.recommendations?.length ? (
          <ul className="compact-list">
            {run.security_report.recommendations.map((item, index) => (
              <li key={`recommendation-${index}`}>
                <strong>{index + 1}</strong>
                <span>{asText(item)}</span>
              </li>
            ))}
          </ul>
        ) : null}
      </section>
    </div>
  );
}

function routerSourceLabel(source?: string) {
  switch (source) {
    case "llm":
      return "大模型判断";
    case "intent_guard":
      return "意图安全护栏";
    case "deterministic":
      return "确定性判断";
    default:
      return "系统判断";
  }
}

function EmptyPanel({ title, description }: { title: string; description: string }) {
  return (
    <div className="insight-empty">
      <Shield size={22} />
      <h3>{title}</h3>
      <p>{description}</p>
    </div>
  );
}

function Detail({ label, value, wide }: { label: string; value: unknown; wide?: boolean }) {
  return (
    <div className={wide ? "detail-item wide" : "detail-item"}>
      <span>{label}</span>
      <strong>{asText(value)}</strong>
    </div>
  );
}

function findTraceForStep(run?: AgentRun | null, selectedStepIndex?: number | null, step?: AgentStep) {
  if (!run?.tool_trace?.length) {
    return undefined;
  }
  if (selectedStepIndex !== null && selectedStepIndex !== undefined) {
    return run.tool_trace[selectedStepIndex];
  }
  if (step?.tool_name) {
    return run.tool_trace.find((trace) => trace.tool_name === step.tool_name);
  }
  return undefined;
}
