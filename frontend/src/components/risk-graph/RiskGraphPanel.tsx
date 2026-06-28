import { useMemo, useState } from "react";
import { GitBranch, LocateFixed, Route, ShieldAlert, ZoomIn, ZoomOut } from "lucide-react";
import {
  asText,
  boundaryLevelLabel,
  decisionLabel,
  operationTypeLabel,
  resourceTypeLabel,
  riskLevelLabel,
  toolNameLabel,
} from "@/lib/formatters";
import type { AgentRun, RiskGraph, RiskGraphEdge, RiskGraphNode } from "@/types/agent";

type DrawableNode = {
  id: string;
  node: RiskGraphNode;
  index: number;
  x: number;
  y: number;
  tone: "good" | "warn" | "danger" | "neutral";
};

type DrawableEdge = {
  id: string;
  edge: RiskGraphEdge;
  index: number;
  from: DrawableNode;
  to: DrawableNode;
  tone: "good" | "warn" | "danger" | "neutral";
};

const nodeWidth = 188;
const nodeHeight = 78;
const horizontalGap = 86;
const verticalGap = 68;
const graphPadding = 42;

export function RiskGraphPanel({ run }: { run?: AgentRun | null }) {
  const graph = findRiskGraph(run);
  const [selectedId, setSelectedId] = useState<string>("");
  const [zoom, setZoom] = useState(1);
  const model = useMemo(() => buildGraphModel(graph), [graph]);

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

  const selected =
    model.nodes.find((item) => item.id === selectedId) ||
    model.edges.find((item) => item.id === selectedId) ||
    model.nodes.find((item) => item.tone === "danger") ||
    model.nodes[0] ||
    null;
  const selectedNode = isDrawableNode(selected) ? selected : null;
  const selectedEdge = isDrawableEdge(selected) ? selected : null;
  const selectedNodeIds = selectedEdge ? new Set([selectedEdge.from.id, selectedEdge.to.id]) : new Set([selectedNode?.id || ""]);

  return (
    <div className="risk-graph-view">
      <section className="risk-graph-summary">
        <div className="graph-stat">
          <strong>{model.nodes.length}</strong>
          <span>风险节点</span>
        </div>
        <div className="graph-stat">
          <strong>{model.edges.length}</strong>
          <span>关联路径</span>
        </div>
        <div className="graph-stat danger">
          <strong>{model.dangerCount}</strong>
          <span>高风险节点</span>
        </div>
      </section>

      <section className="risk-map-card">
        <div className="risk-map-toolbar">
          <div>
            <div className="mini-heading">
              <GitBranch size={16} />
              <span>审计风险图</span>
            </div>
            <p>从真实工具调用链生成，节点表示审计事件，连线表示执行与证据传递顺序。</p>
          </div>
          <div className="risk-map-controls" aria-label="风险图缩放">
            <button type="button" onClick={() => setZoom((value) => Math.max(0.75, value - 0.15))}>
              <ZoomOut size={15} />
              <span>缩小</span>
            </button>
            <button
              type="button"
              onClick={() => {
                setZoom(1);
                setSelectedId("");
              }}
            >
              <LocateFixed size={15} />
              <span>重置</span>
            </button>
            <button type="button" onClick={() => setZoom((value) => Math.min(1.6, value + 0.15))}>
              <ZoomIn size={15} />
              <span>放大</span>
            </button>
          </div>
        </div>

        <div className="risk-map-legend" aria-label="风险图图例">
          <span className="legend-item good">低风险</span>
          <span className="legend-item warn">需复核</span>
          <span className="legend-item danger">高风险</span>
        </div>

        <div className="risk-map-scroll">
          <svg
            className="risk-map-svg"
            width={model.width * zoom}
            height={model.height * zoom}
            viewBox={`0 0 ${model.width} ${model.height}`}
            role="img"
            aria-label="KylinGuard 安全审计风险图"
          >
            <defs>
              <linearGradient id="riskEdgeGradient" x1="0%" x2="100%" y1="0%" y2="0%">
                <stop offset="0%" stopColor="var(--brand)" stopOpacity="0.25" />
                <stop offset="100%" stopColor="var(--brand)" stopOpacity="0.8" />
              </linearGradient>
              <filter id="riskNodeShadow" x="-20%" y="-20%" width="140%" height="140%">
                <feDropShadow dx="0" dy="10" stdDeviation="10" floodColor="rgba(15, 23, 42, 0.16)" />
              </filter>
              <marker id="riskArrow" markerHeight="8" markerWidth="8" orient="auto" refX="7" refY="4">
                <path d="M 0 0 L 8 4 L 0 8 z" fill="currentColor" />
              </marker>
            </defs>

            <g className="risk-map-grid">
              {Array.from({ length: Math.ceil(model.width / 80) + 1 }).map((_, index) => (
                <line key={`v-${index}`} x1={index * 80} x2={index * 80} y1={0} y2={model.height} />
              ))}
              {Array.from({ length: Math.ceil(model.height / 80) + 1 }).map((_, index) => (
                <line key={`h-${index}`} x1={0} x2={model.width} y1={index * 80} y2={index * 80} />
              ))}
            </g>

            <g className="risk-map-edges">
              {model.edges.map((edge) => {
                const active = selectedId === edge.id;
                const related = selectedNode ? edge.from.id === selectedNode.id || edge.to.id === selectedNode.id : active;
                return (
                  <g key={edge.id}>
                    <path
                      className={["risk-map-edge-hit", active ? "active" : "", related ? "related" : ""]
                        .filter(Boolean)
                        .join(" ")}
                      d={edgePath(edge)}
                      onClick={() => setSelectedId(edge.id)}
                    />
                    <path
                      className={["risk-map-edge", edge.tone, active ? "active" : "", related ? "related" : ""]
                        .filter(Boolean)
                        .join(" ")}
                      d={edgePath(edge)}
                      markerEnd="url(#riskArrow)"
                    />
                  </g>
                );
              })}
            </g>

            <g className="risk-map-nodes">
              {model.nodes.map((item) => {
                const active = selectedId === item.id || selectedNodeIds.has(item.id);
                return (
                  <g
                    className={["risk-map-node", item.tone, active ? "active" : ""].filter(Boolean).join(" ")}
                    key={item.id}
                    role="button"
                    tabIndex={0}
                    transform={`translate(${item.x}, ${item.y})`}
                    onClick={() => setSelectedId(item.id)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter" || event.key === " ") {
                        event.preventDefault();
                        setSelectedId(item.id);
                      }
                    }}
                  >
                    <rect width={nodeWidth} height={nodeHeight} rx="16" filter="url(#riskNodeShadow)" />
                    <circle cx="22" cy="24" r="7" />
                    <text className="risk-map-node-index" x="38" y="28">
                      #{item.index + 1}
                    </text>
                    <text className="risk-map-node-title" x="16" y="52">
                      {truncateText(nodeLabel(item.node, item.index), 15)}
                    </text>
                    <text className="risk-map-node-meta" x="16" y="68">
                      {truncateText(nodeSummary(item.node), 22)}
                    </text>
                  </g>
                );
              })}
            </g>
          </svg>
        </div>
      </section>

      <section className="risk-graph-detail">
        {selectedNode ? <NodeDetail item={selectedNode} /> : null}
        {selectedEdge ? <EdgeDetail item={selectedEdge} /> : null}
        {!selectedNode && !selectedEdge ? <p>点击图中的节点或连线查看审计细节。</p> : null}
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

function NodeDetail({ item }: { item: DrawableNode }) {
  const node = item.node;
  const rows = [
    ["工具", nodeLabel(node, item.index)],
    ["风险", riskLevelLabel(asText(node.risk_level))],
    ["结论", node.decision ? decisionLabel(asText(node.decision)) : "未记录"],
    ["操作", operationTypeLabel(asText(node.operation_type))],
    ["资源", resourceTypeLabel(asText(node.resource_type))],
    ["边界", boundaryLevelLabel(asText(node.boundary_level))],
    ["路径", asText(node.resource_path)],
    ["策略", node.allowed_by_policy === undefined ? "未记录" : node.allowed_by_policy ? "允许" : "拦截"],
  ];

  return (
    <>
      <div className="mini-heading">
        <ShieldAlert size={16} />
        <span>节点详情</span>
      </div>
      <div className="risk-detail-grid">
        {rows.map(([label, value]) => (
          <div key={label}>
            <span>{label}</span>
            <strong>{value}</strong>
          </div>
        ))}
      </div>
    </>
  );
}

function EdgeDetail({ item }: { item: DrawableEdge }) {
  return (
    <>
      <div className="mini-heading">
        <Route size={16} />
        <span>路径详情</span>
      </div>
      <div className="risk-detail-grid">
        <div>
          <span>起点</span>
          <strong>{nodeLabel(item.from.node, item.from.index)}</strong>
        </div>
        <div>
          <span>终点</span>
          <strong>{nodeLabel(item.to.node, item.to.index)}</strong>
        </div>
        <div>
          <span>关系</span>
          <strong>{item.edge.label || item.edge.type || "执行顺序"}</strong>
        </div>
      </div>
    </>
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

function buildGraphModel(graph: RiskGraph) {
  const rawNodes = graph?.nodes || [];
  const columns = Math.min(3, Math.max(1, Math.ceil(Math.sqrt(rawNodes.length || 1))));
  const rows = Math.max(1, Math.ceil((rawNodes.length || 1) / columns));
  const width = graphPadding * 2 + columns * nodeWidth + Math.max(0, columns - 1) * horizontalGap;
  const height = graphPadding * 2 + rows * nodeHeight + Math.max(0, rows - 1) * verticalGap;
  const nodes = rawNodes.map((node, index): DrawableNode => {
    const col = index % columns;
    const row = Math.floor(index / columns);
    return {
      id: nodeID(node, index),
      node,
      index,
      x: graphPadding + col * (nodeWidth + horizontalGap),
      y: graphPadding + row * (nodeHeight + verticalGap),
      tone: nodeTone(node),
    };
  });
  const nodeMap = new Map(nodes.map((node) => [node.id, node]));
  const edges = (graph?.edges || [])
    .map((edge, index): DrawableEdge | null => {
      const source = edgeFrom(edge);
      const target = edgeTo(edge);
      const from = nodeMap.get(source) || nodes[index];
      const to = nodeMap.get(target) || nodes[index + 1];
      if (!from || !to) {
        return null;
      }
      return {
        id: `${from.id}-${to.id}-${index}`,
        edge,
        index,
        from,
        to,
        tone: edgeTone(from, to),
      };
    })
    .filter((edge): edge is DrawableEdge => Boolean(edge));

  return {
    nodes,
    edges,
    width,
    height,
    dangerCount: nodes.filter((node) => node.tone === "danger").length,
  };
}

function edgePath(edge: DrawableEdge) {
  const startX = edge.from.x + nodeWidth;
  const startY = edge.from.y + nodeHeight / 2;
  const endX = edge.to.x;
  const endY = edge.to.y + nodeHeight / 2;
  const sameColumn = Math.abs(startX - endX) < nodeWidth / 2;
  if (sameColumn) {
    const x = edge.from.x + nodeWidth / 2;
    const y1 = edge.from.y + nodeHeight;
    const y2 = edge.to.y;
    const c1 = y1 + Math.max(28, (y2 - y1) / 2);
    const c2 = y2 - Math.max(28, (y2 - y1) / 2);
    return `M ${x} ${y1} C ${x} ${c1}, ${x} ${c2}, ${x} ${y2}`;
  }
  const mid = Math.max(36, Math.abs(endX - startX) / 2);
  return `M ${startX} ${startY} C ${startX + mid} ${startY}, ${endX - mid} ${endY}, ${endX} ${endY}`;
}

function nodeID(node: RiskGraphNode, index: number) {
  return asText(node.id || node.step_id || node.tool_name || `node-${index}`);
}

function nodeLabel(node: RiskGraphNode, index: number) {
  if (typeof node.label === "string" && node.label.trim()) {
    return node.label;
  }
  if (typeof node.tool_name === "string" && node.tool_name.trim()) {
    const prefix = typeof node.step_index === "number" ? `#${node.step_index} ` : "";
    return `${prefix}${toolNameLabel(node.tool_name)}`;
  }
  if (typeof node.step_id === "string" && node.step_id.trim()) {
    return node.step_id;
  }
  return `节点 ${index + 1}`;
}

function nodeSummary(node: RiskGraphNode) {
  const parts = [
    node.risk_level ? riskLevelLabel(asText(node.risk_level)) : "",
    node.decision ? decisionLabel(asText(node.decision)) : "",
    node.resource_type ? resourceTypeLabel(asText(node.resource_type)) : "",
    node.boundary_level ? boundaryLevelLabel(asText(node.boundary_level)) : "",
  ].filter(Boolean);
  return parts.length ? parts.join(" / ") : "普通节点";
}

function nodeTone(node: RiskGraphNode): DrawableNode["tone"] {
  const risk = asText(node.risk_level || node.decision || node.boundary_level).toLowerCase();
  if (risk.includes("high") || risk.includes("critical") || risk.includes("deny") || risk.includes("danger")) {
    return "danger";
  }
  if (risk.includes("medium") || risk.includes("review") || risk.includes("sensitive")) {
    return "warn";
  }
  if (risk.includes("low") || risk.includes("allow")) {
    return "good";
  }
  return "neutral";
}

function edgeTone(from: DrawableNode, to: DrawableNode): DrawableEdge["tone"] {
  if (from.tone === "danger" || to.tone === "danger") {
    return "danger";
  }
  if (from.tone === "warn" || to.tone === "warn") {
    return "warn";
  }
  if (from.tone === "good" && to.tone === "good") {
    return "good";
  }
  return "neutral";
}

function edgeFrom(edge: RiskGraphEdge) {
  return asText(edge.source || edge.from || "");
}

function edgeTo(edge: RiskGraphEdge) {
  return asText(edge.target || edge.to || "");
}

function truncateText(value: string, maxLength: number) {
  return value.length > maxLength ? `${value.slice(0, maxLength - 1)}…` : value;
}

function isDrawableNode(value: DrawableNode | DrawableEdge | null): value is DrawableNode {
  return Boolean(value && "node" in value);
}

function isDrawableEdge(value: DrawableNode | DrawableEdge | null): value is DrawableEdge {
  return Boolean(value && "edge" in value);
}
