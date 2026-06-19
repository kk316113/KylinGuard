import { GitBranch, Route, ShieldAlert } from "lucide-react";
import type { AgentRun, RiskGraph } from "@/types/agent";
import { asText } from "@/lib/formatters";

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
        <p>后端未返回 risk_graph。前端不会伪造风险图，只展示后端审计核心生成的数据。</p>
      </div>
    );
  }

  return (
    <div className="risk-graph-view">
      <section>
        <div className="mini-heading">
          <GitBranch size={16} />
          <span>风险节点</span>
        </div>
        <div className="graph-list">
          {(graph.nodes || []).map((node, index) => (
            <div className="graph-node" key={node.id || `node-${index}`}>
              <strong>{node.label || node.id || `Node ${index + 1}`}</strong>
              <span>{node.type || "node"} {node.risk_level ? ` / ${node.risk_level}` : ""}</span>
            </div>
          ))}
        </div>
      </section>

      <section>
        <div className="mini-heading">
          <Route size={16} />
          <span>风险边</span>
        </div>
        <div className="graph-list">
          {(graph.edges || []).map((edge, index) => (
            <div className="graph-edge" key={`${edge.source || "s"}-${edge.target || "t"}-${index}`}>
              <strong>{edge.source || "source"} &gt; {edge.target || "target"}</strong>
              <span>{edge.label || edge.type || "edge"}</span>
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
                <strong>{asText(hotspot.summary || hotspot.node_id || `Hotspot ${index + 1}`)}</strong>
                <span>{asText(hotspot.risk_level)}</span>
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
      <h3>等待 Agent 任务</h3>
      <p>任务完成后，如果后端返回 risk_graph，这里会展示执行链风险图。</p>
    </div>
  );
}

function findRiskGraph(run?: AgentRun | null): RiskGraph {
  return run?.risk_graph || run?.audit_result?.risk_graph || run?.security_report?.risk_graph || null;
}
