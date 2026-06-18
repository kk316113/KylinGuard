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
    case 'allow': return '通过'
    case 'review': return '需复核'
    case 'deny': return '安全拦截/高风险'
    default: return 'Unknown'
  }
})

const zhText = computed(() => {
  switch (props.decision) {
    case 'allow': return '安全审计通过'
    case 'review': return '安全审计提示：需要复核'
    case 'deny': return '安全审计提示：高风险或已拦截'
    default: return '未知'
  }
})

const description = computed(() => {
  switch (props.decision) {
    case 'allow': return '审计层未发现需要阻断的风险，当前结果可作为低风险诊断输出查看。'
    case 'review': return '审计层认为该任务涉及敏感系统信息或需要人工确认，结果应作为需复核的安全提示查看。'
    case 'deny': return '审计层给出高风险或拦截判定。这不是前端请求失败，而是安全控制链路的正常结果。'
    default: return ''
  }
})

function riskColor(r: string) {
  return r === 'high' ? 'red' : r === 'medium' ? 'orange' : 'green'
}
</script>

<style scoped>
.decision-card { margin-bottom: 10px; border: 1px solid #e5e6eb; }
.decision-card.decision-deny { border-left: 5px solid rgb(var(--red-6)); }
.decision-card.decision-review { border-left: 5px solid rgb(var(--orange-6)); }
.decision-card.decision-allow { border-left: 5px solid rgb(var(--green-6)); }
.decision-header { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.decision-title { font-size: 16px; font-weight: 700; margin-bottom: 6px; color: #1d2129; }
.decision-desc { font-size: 13px; color: #4e5969; margin-bottom: 8px; line-height: 1.5; }
</style>
