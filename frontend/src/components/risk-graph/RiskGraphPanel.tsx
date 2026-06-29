import { useMemo, useState } from "react";
import {
  Background,
  Controls,
  Handle,
  MiniMap,
  Position,
  ReactFlow,
  type Edge,
  type Node,
  type NodeProps,
} from "@xyflow/react";
import { GitBranch, Route, ShieldAlert } from "lucide-react";
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

type GraphTone = "good" | "warn" | "danger" | "neutral";

type SemanticNodeData = {
  raw: RiskGraphNode;
  label: string;
  summary: string;
  role: string;
  tone: GraphTone;
};

type SemanticFlowNode = Node<SemanticNodeData, "semantic">;

type GraphModel = {
  nodes: SemanticFlowNode[];
  edges: Edge[];
  nodeMap: Map<string, SemanticFlowNode>;
  edgeMap: Map<string, RiskGraphEdge>;
  dangerCount: number;
  semanticRoles: number;
};

const roleOrder = ["policy", "tool_call", "operation", "resource", "boundary", "decision", "unknown"];
const roleLabels: Record<string, string> = {
  policy: "策略",
  tool_call: "工具",
  operation: "操作",
  resource: "资源",
  boundary: "边界",
  decision: "结论",
  unknown: "未知",
};
const roleX: Record<string, number> = {
  policy: 0,
  tool_call: 240,
  operation: 500,
  resource: 760,
  boundary: 1020,
  decision: 1280,
  unknown: 500,
};
const rowGap = 116;

export function RiskGraphPanel({ run }: { run?: AgentRun | null }) {
  const graph = findRiskGraph(run);
  const [selectedId, setSelectedId] = useState<string>("");
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

  const selectedNode = selectedId ? model.nodeMap.get(selectedId) : undefined;
  const selectedEdge = selectedId ? model.edgeMap.get(selectedId) : undefined;
  const nodeTypes = { semantic: SemanticRiskNode };

  return (
    <div className="risk-graph-view">
      <section className="risk-graph-summary">
        <div className="graph-stat">
          <strong>{model.nodes.length}</strong>
          <span>语义节点</span>
        </div>
        <div className="graph-stat">
          <strong>{model.edges.length}</strong>
          <span>语义关系</span>
        </div>
        <div className="graph-stat danger">
          <strong>{model.dangerCount}</strong>
          <span>风险热点</span>
        </div>
      </section>

      <section className="risk-map-card">
        <div className="risk-map-toolbar">
          <div>
            <div className="mini-heading">
              <GitBranch size={16} />
              <span>联合语义风险图</span>
            </div>
            <p>基于工具调用、操作类型、资源对象、边界等级、策略结论和审计结论联合生成。</p>
          </div>
          <div className="risk-map-legend" aria-label="风险图图例">
            <span className="legend-item good">低风险</span>
            <span className="legend-item warn">需复核</span>
            <span className="legend-item danger">高风险</span>
          </div>
        </div>

        <div className="semantic-flow-shell">
          <ReactFlow
            nodes={model.nodes}
            edges={model.edges}
            nodeTypes={nodeTypes}
            fitView
            fitViewOptions={{ padding: 0.2, minZoom: 0.45, maxZoom: 1.2 }}
            minZoom={0.25}
            maxZoom={1.8}
            nodesDraggable
            nodesConnectable={false}
            elementsSelectable
            onNodeClick={(_, node) => setSelectedId(node.id)}
            onEdgeClick={(_, edge) => setSelectedId(edge.id)}
          >
            <Background gap={24} size={1} />
            <Controls showInteractive={false} />
            <MiniMap
              pannable
              zoomable
              nodeColor={(node) => toneColor((node.data as SemanticNodeData | undefined)?.tone || "neutral")}
            />
          </ReactFlow>
        </div>
      </section>

      <section className="risk-graph-detail">
        {selectedNode ? <NodeDetail node={selectedNode} /> : null}
        {selectedEdge ? <EdgeDetail edge={selectedEdge} model={model} /> : null}
        {!selectedNode && !selectedEdge ? <GraphOverview graph={graph} model={model} /> : null}
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

function SemanticRiskNode({ data, selected }: NodeProps<SemanticFlowNode>) {
  return (
    <div className={["semantic-risk-node", data.tone, selected ? "selected" : ""].filter(Boolean).join(" ")}>
      <Handle type="target" position={Position.Left} />
      <div className="semantic-node-role">{roleLabel(data.role)}</div>
      <strong>{data.label}</strong>
      <span>{data.summary}</span>
      <Handle type="source" position={Position.Right} />
    </div>
  );
}

function NodeDetail({ node }: { node: SemanticFlowNode }) {
  const raw = node.data.raw;
  const rows = [
    ["类型", roleLabel(node.data.role)],
    ["名称", node.data.label],
    ["风险", riskLevelLabel(asText(raw.risk_level))],
    ["结论", raw.decision ? decisionLabel(asText(raw.decision)) : "未记录"],
    ["工具", raw.tool_name ? toolNameLabel(asText(raw.tool_name)) : "未记录"],
    ["操作", raw.operation_type ? operationTypeLabel(asText(raw.operation_type)) : "未记录"],
    ["资源", raw.resource_type ? resourceTypeLabel(asText(raw.resource_type)) : "未记录"],
    ["边界", raw.boundary_level ? boundaryLevelLabel(asText(raw.boundary_level)) : "未记录"],
    ["路径", asText(raw.resource_path || "未记录")],
    ["策略", raw.allowed_by_policy === undefined ? "未记录" : raw.allowed_by_policy ? "允许" : "拦截"],
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

function EdgeDetail({ edge, model }: { edge: RiskGraphEdge; model: GraphModel }) {
  const fromID = edgeFrom(edge);
  const toID = edgeTo(edge);
  const from = model.nodeMap.get(fromID);
  const to = model.nodeMap.get(toID);

  return (
    <>
      <div className="mini-heading">
        <Route size={16} />
        <span>关系详情</span>
      </div>
      <div className="risk-detail-grid">
        <div>
          <span>起点</span>
          <strong>{from?.data.label || fromID}</strong>
        </div>
        <div>
          <span>终点</span>
          <strong>{to?.data.label || toID}</strong>
        </div>
        <div>
          <span>关系</span>
          <strong>{edgeLabel(edge)}</strong>
        </div>
      </div>
    </>
  );
}

function GraphOverview({ graph, model }: { graph: RiskGraph; model: GraphModel }) {
  return (
    <>
      <div className="mini-heading">
        <GitBranch size={16} />
        <span>图谱说明</span>
      </div>
      <div className="risk-detail-grid">
        <div>
          <span>语义层数</span>
          <strong>{model.semanticRoles}</strong>
        </div>
        <div>
          <span>边界穿越</span>
          <strong>{graph?.boundary_crossings?.length || 0}</strong>
        </div>
        <div>
          <span>决策路径</span>
          <strong>{graph?.decision_path?.length || 0}</strong>
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

function buildGraphModel(graph: RiskGraph): GraphModel {
  const rawNodes = graph?.nodes || [];
  const grouped = groupNodes(rawNodes);
  const nodes = rawNodes.map((node, index): SemanticFlowNode => {
    const id = nodeID(node, index);
    const role = nodeRole(node);
    const position = semanticPosition(role, grouped.get(role)?.indexOf(node) ?? index);
    return {
      id,
      type: "semantic",
      position,
      data: {
        raw: node,
        label: nodeLabel(node, index),
        summary: nodeSummary(node),
        role,
        tone: nodeTone(node),
      },
    };
  });
  const nodeMap = new Map(nodes.map((node) => [node.id, node]));
  const edgeMap = new Map<string, RiskGraphEdge>();
  const edges = (graph?.edges || [])
    .map((edge, index): Edge | null => {
      const source = edgeFrom(edge);
      const target = edgeTo(edge);
      if (!nodeMap.has(source) || !nodeMap.has(target)) {
        return null;
      }
      const id = asText(edge.id || `${source}-${target}-${edge.type || "edge"}-${index}`);
      edgeMap.set(id, edge);
      const tone = edgeTone(edge, nodeMap);
      return {
        id,
        source,
        target,
        type: "smoothstep",
        label: edgeLabel(edge),
        animated: edge.type === "next_action" || edge.type === "crosses_boundary",
        style: { stroke: toneColor(tone), strokeWidth: tone === "danger" ? 2.6 : 2 },
        markerEnd: { type: "arrowclosed", color: toneColor(tone) },
        labelStyle: { fill: "var(--muted)", fontSize: 11, fontWeight: 600 },
        labelBgStyle: { fill: "var(--surface)", fillOpacity: 0.88 },
        data: { tone },
      };
    })
    .filter((edge): edge is Edge => Boolean(edge));

  return {
    nodes,
    edges,
    nodeMap,
    edgeMap,
    dangerCount: nodes.filter((node) => node.data.tone === "danger").length,
    semanticRoles: new Set(nodes.map((node) => node.data.role)).size,
  };
}

function groupNodes(nodes: RiskGraphNode[]) {
  const grouped = new Map<string, RiskGraphNode[]>();
  for (const node of nodes) {
    const role = nodeRole(node);
    const list = grouped.get(role) || [];
    list.push(node);
    grouped.set(role, list);
  }
  return grouped;
}

function semanticPosition(role: string, row: number) {
  const normalizedRole = roleOrder.includes(role) ? role : "unknown";
  return {
    x: roleX[normalizedRole] ?? roleX.unknown,
    y: 40 + row * rowGap,
  };
}

function nodeID(node: RiskGraphNode, index: number) {
  return asText(node.id || node.step_id || node.tool_name || `node-${index}`);
}

function nodeRole(node: RiskGraphNode) {
  const role = asText(node.semantic_role || node.type || "unknown");
  switch (role) {
    case "tool_event":
      return "tool_call";
    case "policy_guard":
      return "policy";
    case "audit_decision":
      return "decision";
    default:
      return role;
  }
}

function roleLabel(role: string) {
  return roleLabels[role] || roleLabels.unknown;
}

function nodeLabel(node: RiskGraphNode, index: number) {
  if (typeof node.label === "string" && node.label.trim()) {
    return node.type === "tool_call" && node.tool_name ? toolNameLabel(node.tool_name) : node.label;
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
  const role = nodeRole(node);
  if (role === "operation") {
    return operationTypeLabel(asText(node.operation_type || node.label));
  }
  if (role === "resource") {
    return [resourceTypeLabel(asText(node.resource_type)), asText(node.resource_path)].filter(Boolean).join(" / ");
  }
  if (role === "boundary") {
    return boundaryLevelLabel(asText(node.boundary_level || node.label));
  }
  if (role === "decision") {
    return decisionLabel(asText(node.decision || node.label));
  }
  const parts = [
    node.risk_level ? riskLevelLabel(asText(node.risk_level)) : "",
    node.decision ? decisionLabel(asText(node.decision)) : "",
    node.resource_type ? resourceTypeLabel(asText(node.resource_type)) : "",
    node.boundary_level ? boundaryLevelLabel(asText(node.boundary_level)) : "",
  ].filter(Boolean);
  return parts.length ? parts.join(" / ") : "普通节点";
}

function nodeTone(node: RiskGraphNode): GraphTone {
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

function edgeTone(edge: RiskGraphEdge, nodeMap: Map<string, SemanticFlowNode>): GraphTone {
  const risk = asText(edge.risk_level || edge.type).toLowerCase();
  if (risk.includes("high") || risk.includes("danger") || risk.includes("deny")) {
    return "danger";
  }
  if (risk.includes("medium") || risk.includes("review") || risk.includes("boundary")) {
    return "warn";
  }
  const from = nodeMap.get(edgeFrom(edge));
  const to = nodeMap.get(edgeTo(edge));
  if (from?.data.tone === "danger" || to?.data.tone === "danger") return "danger";
  if (from?.data.tone === "warn" || to?.data.tone === "warn") return "warn";
  return "good";
}

function edgeLabel(edge: RiskGraphEdge) {
  const label = asText(edge.label || edge.type);
  const labels: Record<string, string> = {
    governs: "策略约束",
    performs: "执行操作",
    targets: "访问资源",
    crosses_boundary: "穿越边界",
    audited_as: "审计结论",
    next_action: "下一步",
  };
  return labels[label] || label || "关联";
}

function edgeFrom(edge: RiskGraphEdge) {
  return asText(edge.source || edge.from || "");
}

function edgeTo(edge: RiskGraphEdge) {
  return asText(edge.target || edge.to || "");
}

function toneColor(tone: GraphTone) {
  switch (tone) {
    case "danger":
      return "#ef4444";
    case "warn":
      return "#f59e0b";
    case "good":
      return "#22c55e";
    default:
      return "#64748b";
  }
}
