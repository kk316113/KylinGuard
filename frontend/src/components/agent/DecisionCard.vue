<template>
  <a-card :class="['decision-card', 'decision-' + decision]" :bordered="true">
    <div class="decision-header">
      <a-tag :color="tagColor" size="medium">{{ label }}</a-tag>
      <a-tag v-if="risk" :color="riskColor(risk)" size="small">{{ risk }}</a-tag>
    </div>
    <div class="decision-title">{{ zhText }}</div>
    <div class="decision-desc">{{ description }}</div>
    <a-space v-if="summary" size="mini" wrap>
      <a-tag v-if="scenario" color="arcoblue" size="small">{{ scenario }}</a-tag>
      <a-tag v-if="auditMethod" color="purple" size="small">{{ auditMethod }}</a-tag>
      <a-tag v-if="route" color="cyan" size="small">{{ route }}</a-tag>
      <a-tag v-if="chatModel" color="blue" size="small">{{ chatModel }}</a-tag>
    </a-space>
  </a-card>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  decision: string
  risk?: string
  scenario?: string
  auditMethod?: string
  route?: string
  chatModel?: string
  summary?: boolean
}>()

const tagColor = computed(() => {
  switch (props.decision) {
    case 'allow': return 'green'
    case 'review': return 'orange'
    case 'deny': return 'red'
    default: return 'gray'
  }
})

const label = computed(() => {
  switch (props.decision) {
    case 'allow': return 'Allowed'
    case 'review': return 'Review Required'
    case 'deny': return 'Denied'
    default: return 'Unknown'
  }
})

const zhText = computed(() => {
  switch (props.decision) {
    case 'allow': return '允许执行'
    case 'review': return '需要审查'
    case 'deny': return '已拦截'
    default: return '未知'
  }
})

const description = computed(() => {
  switch (props.decision) {
    case 'allow': return '任务通过安全检查，未触发风险规则。仅涉及低风险只读信息采集。'
    case 'review': return '涉及敏感系统资源访问（如系统日志、认证日志），需要人工关注。'
    case 'deny': return '任务包含危险意图，已在 intent_guard 阶段阻断，未执行任何系统工具。'
    default: return ''
  }
})

function riskColor(r: string) {
  return r === 'high' ? 'red' : r === 'medium' ? 'orange' : 'green'
}
</script>

<style scoped>
.decision-card { margin-bottom: 8px; }
.decision-card.decision-deny { border-left: 4px solid rgb(var(--red-6)); }
.decision-card.decision-review { border-left: 4px solid rgb(var(--orange-6)); }
.decision-card.decision-allow { border-left: 4px solid rgb(var(--green-6)); }
.decision-header { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; }
.decision-title { font-size: 14px; font-weight: 600; margin-bottom: 4px; }
.decision-desc { font-size: 12px; color: var(--color-text-3); margin-bottom: 6px; }
</style>
