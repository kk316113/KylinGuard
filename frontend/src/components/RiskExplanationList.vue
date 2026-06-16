<template>
  <div v-if="items.length" class="explanation-list">
    <article v-for="item in items" :key="item.reason_id" class="explanation-item">
      <div class="item-head">
        <el-tag :type="severityType(item.severity)" effect="dark">{{ item.severity }}</el-tag>
        <strong>{{ item.category }}</strong>
        <span class="muted">{{ item.reason_id }}</span>
      </div>
      <p>{{ item.description }}</p>
      <div v-if="item.evidence_ids?.length" class="evidence-ids">
        <el-tag v-for="id in item.evidence_ids" :key="id" size="small" effect="plain">{{ id }}</el-tag>
      </div>
    </article>
  </div>
  <el-empty v-else description="暂无风险解释" :image-size="80" />
</template>

<script setup lang="ts">
import type { RiskExplanationItem } from '../types/agent'

defineProps<{
  items: RiskExplanationItem[]
}>()

function severityType(severity: string): 'success' | 'warning' | 'danger' | 'info' {
  if (severity === 'high') return 'danger'
  if (severity === 'medium') return 'warning'
  if (severity === 'low') return 'success'
  return 'info'
}
</script>
