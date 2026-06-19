"use client";

import { useMemo, useState } from "react";
import type { ReactNode } from "react";
import { FileText, GitBranch, ListChecks, Shield, Siren, Wrench } from "lucide-react";
import { RiskDecisionBadge } from "@/components/audit/RiskDecisionBadge";
import { RiskGraphPanel } from "@/components/risk-graph/RiskGraphPanel";
import { asText, compactDate, observationSummary, traceSummary } from "@/lib/formatters";
import type { AgentRun, AgentStep, ToolTrace } from "@/types/agent";
import type { AcceptanceSummary, CapabilitiesResponse } from "@/types/runtime";

type TabKey = "audit" | "risk" | "hotspots" | "decision" | "tools" | "report";

type Props = {
  run?: AgentRun | null;
  selectedStepIndex: number | null;
  onSelectStep: (index: number) => void;
  capabilities?: CapabilitiesResponse;
  acceptance?: AcceptanceSummary;
};

const tabs: Array<{ key: TabKey; label: string; icon: ReactNode }> = [
  { key: "audit", label: "审计", icon: <Shield size={15} /> },
  { key: "risk", label: "风险图", icon: <GitBranch size={15} /> },
  { key: "hotspots", label: "热点", icon: <Siren size={15} /> },
  { key: "decision", label: "路径", icon: <ListChecks size={15} /> },
  { key: "tools", label: "工具", icon: <Wrench size={15} /> },
  { key: "report", label: "报告", icon: <FileText size={15} /> },
];

export function RightInsightPanel({ run, selectedStepIndex, onSelectStep, capabilities, acceptance }: Props) {
  const [activeTab, setActiveTab] = useState<TabKey>("audit");
  const selectedStep = useMemo(() => {
    if (!run?.agent_steps?.length || selectedStepIndex === null) {
      return undefined;
    }
    return run.agent_steps[selectedStepIndex];
  }, [run, selectedStepIndex]);
  const selectedTrace = useMemo(() => findTraceForStep(run, selectedStepIndex, selectedStep), [run, selectedStepIndex, selectedStep]);

  return (
    <section className="insight-panel">
      <nav className="insight-tabs" aria-label="Agent insight tabs">
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
        {activeTab === "decision" ? <DecisionPathTab run={run} selectedStepIndex={selectedStepIndex} onSelectStep={onSelectStep} /> : null}
        {activeTab === "tools" ? <ToolsTab capabilities={capabilities} acceptance={acceptance} /> : null}
        {activeTab === "report" ? <ReportTab run={run} /> : null}
      </div>
    </section>
  );
}

function AuditTab({ run, step, trace }: { run?: AgentRun | null; step?: AgentStep; trace?: ToolTrace }) {
  if (!run) {
    return <EmptyPanel title="等待审计数据" description="完成一次对话后，这里会展示后端返回的安全摘要。" />;
  }

  return (
    <div className="insight-stack">
      <section className="insight-section">
        <div className="section-title">
          <Shield size={17} />
          <h3>全局结论</h3>
        </div>
        <div className="audit-summary">
          <RiskDecisionBadge decision={run.decision || run.audit_result?.decision} />
          <span>method={run.audit_result?.method || "unknown"}</span>
          <span>risk_score={run.audit_result?.risk_score ?? "n/a"}</span>
        </div>
        <p>{run.audit_result?.message || run.security_report?.summary || "未返回额外审计说明。"}</p>
      </section>

      <section className="insight-section">
        <div className="section-title">
          <ListChecks size={17} />
          <h3>当前步骤</h3>
        </div>
        {step || trace ? (
          <div className="detail-grid">
            <Detail label="tool" value={step?.tool_name || trace?.tool_name} />
            <Detail label="policy" value={step?.policy_decision || trace?.policy_reason || trace?.allowed_by_policy} />
            <Detail label="operation" value={step?.operation_type || trace?.operation_type} />
            <Detail label="resource" value={step?.resource_type || trace?.resource_type} />
            <Detail label="boundary" value={step?.boundary_level || trace?.boundary_level} />
            <Detail label="observation" value={observationSummary(step) || (trace ? traceSummary(trace) : "")} wide />
          </div>
        ) : (
          <p>没有可选步骤时，只显示全局审计结论。</p>
        )}
      </section>

      {run.audit_result?.violations?.length ? (
        <section className="insight-section">
          <div className="section-title">
            <Siren size={17} />
            <h3>风险点</h3>
          </div>
          <ul className="compact-list">
            {run.audit_result.violations.map((violation, index) => (
              <li key={`${violation.type || "violation"}-${index}`}>
                <strong>{violation.severity || "risk"}</strong>
                <span>{violation.message || violation.type}</span>
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
    return <EmptyPanel title="暂无风险热点" description="热点来自后端审计结果，前端不自行推断。" />;
  }
  const violations = run.audit_result?.violations || [];
  const sensitiveTraces = (run.tool_trace || []).filter((trace) => trace.risk_level === "high" || trace.boundary_level === "high");

  if (!violations.length && !sensitiveTraces.length) {
    return <EmptyPanel title="未发现高风险热点" description="当前响应没有返回 violation 或高边界工具证据。" />;
  }

  return (
    <div className="insight-stack">
      {violations.map((violation, index) => (
        <section className="hotspot-item" key={`${violation.type || "violation"}-${index}`}>
          <strong>{violation.severity || "risk"}</strong>
          <p>{violation.message || violation.type}</p>
        </section>
      ))}
      {sensitiveTraces.map((trace, index) => (
        <section className="hotspot-item" key={`${trace.step_id || "trace"}-${index}`}>
          <strong>{trace.tool_name || "tool"}</strong>
          <p>{trace.risk_hint || trace.output_summary || "高边界工具调用"}</p>
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
    return <EmptyPanel title="暂无执行路径" description="存在工具步骤时，这里会展示每一步 policy decision 和 observation。" />;
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
          <span>#{step.step_index ?? index + 1}</span>
          <strong>{step.tool_name || step.action_type || "step"}</strong>
          <RiskDecisionBadge decision={step.policy_decision} />
          <small>{observationSummary(step) || step.reason || "等待 observation"}</small>
        </button>
      ))}
    </div>
  );
}

function ToolsTab({ capabilities, acceptance }: { capabilities?: CapabilitiesResponse; acceptance?: AcceptanceSummary }) {
  const tools = capabilities?.available_tools || [];

  return (
    <div className="insight-stack">
      <section className="insight-section">
        <div className="section-title">
          <Wrench size={17} />
          <h3>工具注册表</h3>
        </div>
        <p>{tools.length ? `当前后端暴露 ${tools.length} 个受控工具。` : "尚未加载工具能力。"}</p>
        <div className="tool-list">
          {tools.slice(0, 16).map((tool) => (
            <div className="tool-row" key={tool.tool_name}>
              <strong>{tool.display_name || tool.tool_name}</strong>
              <span>{tool.operation_type || "operation"} / {tool.resource_type || "resource"} / {tool.boundary_level || "boundary"}</span>
            </div>
          ))}
        </div>
      </section>

      {acceptance ? (
        <section className="insight-section">
          <div className="section-title">
            <FileText size={17} />
            <h3>验收基线</h3>
          </div>
          <ul className="compact-list">
            {acceptance.stages.map((stage) => (
              <li key={stage.name}>
                <strong>{stage.status}</strong>
                <span>{stage.name}: {stage.title}</span>
              </li>
            ))}
          </ul>
        </section>
      ) : null}
    </div>
  );
}

function ReportTab({ run }: { run?: AgentRun | null }) {
  if (!run) {
    return <EmptyPanel title="暂无报告摘要" description="完成一次对话后，这里会展示会话摘要和最终回答。" />;
  }

  return (
    <div className="insight-stack">
      <section className="insight-section">
        <div className="section-title">
          <FileText size={17} />
          <h3>会话信息</h3>
        </div>
        <div className="detail-grid">
          <Detail label="run_id" value={run.run_id || run.task_id} />
          <Detail label="scene_type" value={run.scene_type} />
          <Detail label="run_status" value={run.run_status} />
          <Detail label="created_at" value={compactDate(run.created_at)} />
          <Detail label="interaction_type" value={run.interaction_type} />
          <Detail label="router_source" value={run.router_source} />
          <Detail label="router_reason" value={run.router_reason} wide />
        </div>
      </section>

      <section className="insight-section">
        <div className="section-title">
          <Shield size={17} />
          <h3>审计摘要</h3>
        </div>
        <p>{run.security_report?.executive_summary || run.security_report?.summary || run.audit_result?.message || "无额外报告摘要。"}</p>
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
