<template>
  <div v-if="suggestions.length" class="followup">
    <div class="followup-label">追问建议：</div>
    <div class="followup-chips">
      <a-tag v-for="(s, i) in suggestions" :key="i" checkable color="arcoblue" class="f-chip" @click="$emit('pick', s)">
        {{ s.label }}
      </a-tag>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

export interface FollowUpItem {
  label: string
  action: string    // 'open-tab' | 'retry' | 'fill-task' | 'explain'
  payload?: string
}

const props = defineProps<{
  decision: string
  scenario: string
  task: string
  hasEvidence: boolean
}>()

defineEmits<{ pick: [s: FollowUpItem] }>()

const suggestions = computed<FollowUpItem[]>(() => {
  const items: FollowUpItem[] = []

  if (props.decision === 'deny') {
    items.push({ label: '为什么被拦截？', action: 'fill-task', payload: 'why was this task denied' })
    items.push({ label: '查看 intent_guard 证据', action: 'open-tab', payload: 'evidence' })
    items.push({ label: '改成只读检查', action: 'fill-task', payload: 'check system resource usage' })
  } else if (props.decision === 'review') {
    items.push({ label: '为什么结果是 Review？', action: 'fill-task', payload: 'why is the result review required' })
    if (props.hasEvidence) items.push({ label: '查看详细证据链', action: 'open-tab', payload: 'evidence' })
    items.push({ label: '展开执行过程', action: 'open-tab', payload: 'trace' })
    items.push({ label: '用 Eino Runtime 再跑一次', action: 'retry', payload: 'eino' })
  } else {
    items.push({ label: '查看执行过程', action: 'open-tab', payload: 'trace' })
    items.push({ label: '为什么允许执行？', action: 'fill-task', payload: 'why was this task allowed' })
    items.push({ label: '查看安全报告', action: 'open-tab', payload: 'overview' })
  }
  return items
})
</script>

<style scoped>
.followup { margin-top: 12px; }
.followup-label { font-size: 12px; color: #86909c; margin-bottom: 6px; }
.followup-chips { display: flex; flex-wrap: wrap; gap: 6px; }
.f-chip { cursor: pointer; padding: 4px 12px; font-size: 12px; }
</style>
