<template>
  <div class="exec-accordion">
    <a-collapse :bordered="false">
      <a-collapse-item v-if="traces.length" key="tools" :header="'工具调用链 / Tool Calls (' + traces.length + ')'">
        <div v-for="(t, i) in traces" :key="i" class="tool-card">
          <div class="tool-card-header">
            <span class="tool-index">#{{ i + 1 }}</span>
            <strong class="tool-name">{{ t.tool_name }}</strong>
            <a-tag v-if="t.operation_type" color="arcoblue" size="small">{{ t.operation_type }}</a-tag>
            <a-tag v-if="t.resource_type" color="cyan" size="small">{{ t.resource_type }}</a-tag>
            <a-tag :color="boundaryColor(t.boundary_level)" size="small">{{ t.boundary_level }}</a-tag>
            <a-tag :color="t.status === 'ok' ? 'green' : 'red'" size="small">{{ t.status }}</a-tag>
          </div>
          <div class="tool-card-meta">
            <a-space size="mini" wrap>
              <span class="meta-item">策略: {{ t.allowed_by_policy ? '允许' : '拒绝' }}</span>
              <span v-if="t.execution_context?.profile" class="meta-item">Profile: {{ t.execution_context.profile }}</span>
              <span v-if="t.execution_context?.shell_used !== undefined" class="meta-item">Shell: {{ t.execution_context.shell_used ? '是' : '否' }}</span>
              <span v-if="t.execution_context?.sudo_used !== undefined" class="meta-item">Sudo: {{ t.execution_context.sudo_used ? '是' : '否' }}</span>
              <span v-if="t.policy_reason" class="meta-item">{{ t.policy_reason }}</span>
            </a-space>
          </div>
        </div>
      </a-collapse-item>

      <a-collapse-item v-if="plan?.steps?.length" key="plan" header="计划步骤 / Plan Steps">
        <a-timeline>
          <a-timeline-item v-for="(step, i) in plan.steps" :key="i" :line-type="'solid'">
            <div class="step-line">
              <strong>{{ step.tool_name }}</strong>
              <span v-if="step.reason" class="step-reason">: {{ step.reason }}</span>
              <a-tag v-if="step.risk_level" :color="riskColor(step.risk_level)" size="small">{{ step.risk_level }}</a-tag>
            </div>
          </a-timeline-item>
        </a-timeline>
      </a-collapse-item>

      <a-collapse-item v-if="recommendations?.length" key="rec" header="建议 / Recommendations">
        <a-timeline>
          <a-timeline-item v-for="(r, i) in recommendations" :key="i">
            <div class="rec-line">
              <a-tag :color="priorityColor(r.priority)" size="small">{{ r.priority }}</a-tag>
              <span>{{ r.action }}</span>
            </div>
          </a-timeline-item>
        </a-timeline>
      </a-collapse-item>

      <a-collapse-item v-if="evidenceItems && evidenceItems.length" key="ev" header="安全证据链 / Evidence Chain ({{ evidenceItems.length }})">
        <a-table :data="evidenceItems || []" :pagination="false" size="small">
          <a-column title="#" data-index="evidence_id" :width="50"></a-column>
          <a-column title="工具" data-index="tool_name" :width="100"></a-column>
          <a-column title="资源" data-index="resource_type" :width="100"></a-column>
          <a-column title="边界" data-index="boundary_level" :width="80">
            <template #cell="{ record }">
              <a-tag :color="boundaryColor(record.boundary_level)" size="small">{{ record.boundary_level }}</a-tag>
            </template>
          </a-column>
          <a-column title="摘要" data-index="summary" :ellipsis="true"></a-column>
        </a-table>
      </a-collapse-item>
    </a-collapse>
  </div>
</template>

<script setup lang="ts">
import type { ToolTraceItem, Plan, RecommendationItem, EvidenceItem } from '../../types/agent'

defineProps<{
  traces: ToolTraceItem[]
  plan?: Plan | null
  recommendations?: RecommendationItem[]
  evidenceItems?: EvidenceItem[]
}>()

function boundaryColor(b: string) {
  return b === 'sensitive_system_resource' ? 'red' : b === 'privileged' ? 'orange' : b === 'public' ? 'green' : 'arcoblue'
}
function riskColor(r: string) { return r === 'high' ? 'red' : r === 'medium' ? 'orange' : 'green' }
function priorityColor(p: string) { return p === 'high' ? 'red' : p === 'medium' ? 'orange' : 'green' }
</script>

<style scoped>
.exec-accordion { margin-top: 8px; }
.tool-card { border: 1px solid var(--color-border); border-radius: 6px; padding: 8px; margin-bottom: 6px; }
.tool-card-header { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; margin-bottom: 4px; }
.tool-index { color: var(--color-text-3); font-size: 11px; min-width: 20px; }
.tool-name { font-size: 13px; }
.tool-card-meta { font-size: 11px; color: var(--color-text-3); }
.meta-item { color: var(--color-text-3); }
.step-line { font-size: 13px; }
.step-reason { color: var(--color-text-3); }
.rec-line { font-size: 12px; }
</style>
