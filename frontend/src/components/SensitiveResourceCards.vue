<template>
  <div v-if="items.length" class="resource-grid">
    <article v-for="item in items" :key="resourceKey(item)" class="resource-card">
      <div class="resource-card-head">
        <el-tag class="sensitive-tag" effect="dark">{{ item.resource_type }}</el-tag>
        <el-tag :type="item.allowed_by_policy ? 'success' : 'danger'" effect="plain">
          {{ item.allowed_by_policy ? '策略允许' : '策略拒绝' }}
        </el-tag>
      </div>
      <div class="resource-path">{{ item.resource_path || 'resource path unavailable' }}</div>
      <div class="resource-meta">{{ item.boundary_level }}</div>
      <p>{{ item.access_reason }}</p>
    </article>
  </div>
  <el-empty v-else description="未发现敏感资源访问" :image-size="80" />
</template>

<script setup lang="ts">
import type { SensitiveResourceItem } from '../types/agent'

defineProps<{
  items: SensitiveResourceItem[]
}>()

function resourceKey(item: SensitiveResourceItem): string {
  return `${item.resource_type}-${item.resource_path}-${item.boundary_level}`
}
</script>
