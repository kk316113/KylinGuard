<template>
  <div v-if="graph && graph.nodes && graph.nodes.length" class="risk-graph-view">
    <a-collapse :default-active-key="['nodes']" :bordered="false">
      <a-collapse-item key="nodes" header="全局风险图">
        <template #extra>
          <a-tag color="arcoblue" size="small">{{ nodeCount }} 节点 / {{ edgeCount }} 边</a-tag>
        </template>

        <!-- Nodes: one per tool_call audit_report -->
        <div class="rg-nodes">
          <div v-for="(node, i) in graph.nodes" :key="nodeKey(node, i)" class="rg-node">
            <div class="rg-node-head">
              <span class="rg-node-idx">#{{ node.step_index ?? i + 1 }}</span>
              <strong class="rg-node-tool">{{ node.tool_name || 'step' }}</strong>
              <a-tag v-if="node.decision" :color="decisionColor(node.decision)" size="small">{{ node.decision }}</a-tag>
              <span v-if="typeof node.risk_score === 'number'" class="rg-node-risk">风险 {{ node.risk_score.toFixed(2) }}</span>
            </div>
            <div class="rg-node-meta">
              <span v-if="node.method" class="rg-chip">{{ node.method }}</span>
              <span v-if="typeof node.violations_count === 'number' && node.violations_count > 0" class="rg-chip rg-chip-warn">{{ node.violations_count }} 违规</span>
              <span v-if="node.operation_type" class="rg-chip">op={{ node.operation_type }}</span>
              <span v-if="node.boundary_level" class="rg-chip">boundary={{ node.boundary_level }}</span>
            </div>
            <!-- Sequence edge to next node -->
            <div v-if="i < edgeCount" class="rg-edge">
              <span class="rg-edge-arrow">↓</span>
              <span class="rg-edge-label">sequence</span>
            </div>
          </div>
        </div>

        <!-- Edges list (detail) -->
        <div v-if="graph.edges && graph.edges.length" class="rg-edges">
          <div class="rg-edges-title">边 ({{ edgeCount }})</div>
          <div v-for="(edge, i) in graph.edges" :key="edgeKey(edge, i)" class="rg-edge-row">
            <span class="rg-edge-from">{{ edge.from || '?' }}</span>
            <span class="rg-edge-arrow-h">→</span>
            <span class="rg-edge-to">{{ edge.to || '?' }}</span>
            <a-tag size="small" color="gray">{{ edge.type }}</a-tag>
          </div>
        </div>
      </a-collapse-item>
    </a-collapse>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { RiskGraph } from '../../types/agent'

const props = defineProps<{
  graph?: RiskGraph | null
}>()

const nodeCount = computed(() => props.graph?.nodes?.length ?? 0)
const edgeCount = computed(() => props.graph?.edges?.length ?? 0)

function decisionColor(decision: unknown): string {
  const d = String(decision || '').toLowerCase()
  if (d === 'allow' || d === 'allowed') return 'green'
  if (d === 'review') return 'orange'
  if (d === 'deny' || d === 'denied') return 'red'
  return 'gray'
}

function nodeKey(node: Record<string, unknown>, i: number): string {
  return String(node.step_id || node.step_index || i)
}
function edgeKey(edge: Record<string, unknown>, i: number): string {
  return `${edge.from}-${edge.to}-${i}`
}
</script>

<style scoped>
.risk-graph-view { margin-top: 10px; }
.rg-nodes { display: flex; flex-direction: column; }
.rg-node { border: 1px solid #e5e6eb; border-radius: 6px; padding: 8px 12px; background: #fafafa; }
.rg-node-head { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.rg-node-idx { color: #86909c; font-size: 12px; font-weight: 600; min-width: 22px; }
.rg-node-tool { font-size: 14px; font-weight: 600; color: #1d2129; }
.rg-node-risk { font-size: 12px; color: #4e5969; background: #f0f0f0; padding: 1px 8px; border-radius: 4px; }
.rg-node-meta { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 6px; }
.rg-chip { font-size: 12px; color: #86909c; }
.rg-chip-warn { color: #f53f3f; }
.rg-edge { display: flex; flex-direction: column; align-items: center; margin: 4px 0; color: #c9cdd4; }
.rg-edge-arrow { font-size: 16px; line-height: 1; }
.rg-edge-label { font-size: 11px; color: #86909c; }
.rg-edges { margin-top: 12px; border-top: 1px dashed #e5e6eb; padding-top: 8px; }
.rg-edges-title { font-size: 12px; font-weight: 600; color: #4e5969; margin-bottom: 6px; }
.rg-edge-row { display: flex; align-items: center; gap: 8px; font-size: 12px; color: #4e5969; margin-bottom: 4px; font-family: monospace; }
.rg-edge-arrow-h { color: #c9cdd4; }
</style>
