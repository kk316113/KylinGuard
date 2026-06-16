<template>
  <div v-if="items.length" class="recommendation-list">
    <article v-for="item in items" :key="item.recommendation_id" class="recommendation-item">
      <div class="item-head">
        <el-tag :type="priorityType(item.priority)" effect="dark">{{ item.priority }}</el-tag>
        <strong>{{ item.action }}</strong>
      </div>
      <p>{{ item.rationale }}</p>
      <el-tag :type="item.is_destructive ? 'danger' : 'info'" effect="plain">
        {{ item.is_destructive ? '需要人工审批的高风险动作' : '人工建议' }}
      </el-tag>
    </article>
  </div>
  <el-empty v-else description="暂无处置建议" :image-size="80" />
</template>

<script setup lang="ts">
import type { RecommendationItem } from '../types/agent'

defineProps<{
  items: RecommendationItem[]
}>()

function priorityType(priority: string): 'success' | 'warning' | 'danger' | 'info' {
  if (priority === 'high') return 'danger'
  if (priority === 'medium') return 'warning'
  if (priority === 'low') return 'success'
  return 'info'
}
</script>
