import { GitBranch, Route, ShieldAlert } from "lucide-react";
import { asText, riskLevelLabel } from "@/lib/formatters";
import type { AgentRun, RiskGraph } from "@/types/agent";

export function RiskGraphPanel({ run }: { run?: AgentRun | null }) {
  const graph = findRiskGraph(run);

  if (!run) {
    return <EmptyRiskGraph />;
  }

  if (!graph || (!graph.nodes?.length && !graph.edges?.length)) {
    return (
      <div className="insight-empty">
        <GitBranch size={22} />
        <h3>暂无风险图</h3>
        <p>本次审计没有生成风险图数据。</p>
      </div>
    );
  }

  return (
    <div className="risk-graph-view">
      <section className="risk-graph-summary">
        <div className="graph-node">
          <strong>{graph.nodes?.length || 0}</strong>
          <span>风险节点</span>
        </div>
        <div className="graph-edge">
          <strong>{graph.edges?.length || 0}</strong>
          <span>关联路径</span>
        </div>
        <div className="graph-node">
          <strong>{graph.risk_hotspots?.length || 0}</strong>
          <span>风险热点</span>
        </div>
      </section>

      <section>
        <div className="mini-heading">
          <GitBranch size={16} />
          <span>风险节点</span>
        </div>
        <div className="graph-list">
          {(graph.nodes || []).map((node, index) => (
            <div className="graph-node" key={node.id || `node-${index}`}>
              <strong>{node.label || `节点 ${index + 1}`}</strong>
              <span>{node.risk_level ? riskLevelLabel(node.risk_level) : "普通节点"}</span>
            </div>
          ))}
        </div>
      </section>

      <section>
        <div className="mini-heading">
          <Route size={16} />
          <span>关联路径</span>
        </div>
        <div className="graph-list">
          {(graph.edges || []).map((edge, index) => (
            <div className="graph-edge" key={`${edge.source || edge.from || "s"}-${edge.target || edge.to || "t"}-${index}`}>
              <strong>节点关联 {index + 1}</strong>
              <span>{edge.label || `${edge.source || edge.from || "起点"} → ${edge.target || edge.to || "终点"}`}</span>
            </div>
          ))}
        </div>
      </section>

      {graph.risk_hotspots?.length ? (
        <section>
          <div className="mini-heading">
            <ShieldAlert size={16} />
            <span>风险热点</span>
          </div>
          <div className="graph-list">
            {graph.risk_hotspots.map((hotspot, index) => (
              <div className="graph-node danger" key={`hotspot-${index}`}>
                <strong>{asText(hotspot.summary || `热点 ${index + 1}`)}</strong>
                <span>{riskLevelLabel(typeof hotspot.risk_level === "string" ? hotspot.risk_level : undefined)}</span>
              </div>
            ))}
          </div>
        </section>
      ) : null}
    </div>
  );
}

function EmptyRiskGraph() {
  return (
    <div className="insight-empty">
      <GitBranch size={22} />
      <h3>暂无会话数据</h3>
      <p>完成一次对话后，这里会展示对应的审计图数据。</p>
    </div>
  );
}

function findRiskGraph(run?: AgentRun | null): RiskGraph {
  return run?.risk_graph || run?.audit_result?.risk_graph || run?.security_report?.risk_graph || null;
}
