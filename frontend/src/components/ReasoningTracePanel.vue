<template>
  <section class="reasoning-trace">
    <el-empty v-if="!trace" description="No reasoning trace available" :image-size="80" />

    <template v-else>
      <div class="trace-header">
        <el-descriptions :column="4" size="small" border>
          <el-descriptions-item label="Trace ID">{{ trace.trace_id }}</el-descriptions-item>
          <el-descriptions-item label="Runtime">{{ trace.runtime }}</el-descriptions-item>
          <el-descriptions-item label="Duration">{{ trace.duration_ms }}ms</el-descriptions-item>
          <el-descriptions-item label="Spans">{{ trace.spans.length }}</el-descriptions-item>
          <el-descriptions-item label="Task" :span="4">{{ trace.task_summary }}</el-descriptions-item>
        </el-descriptions>
      </div>

      <el-table :data="flatSpans" stripe size="small" max-height="400" style="width: 100%; margin-top: 12px">
        <el-table-column label="Type" prop="type" width="140">
          <template #default="{ row }">
            <el-tag :type="tagType(row.type)" size="small">{{ row.type }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Name" prop="name" min-width="200" />
        <el-table-column label="Status" prop="status" width="80">
          <template #default="{ row }">
            <el-tag :type="row.status === 'ok' ? 'success' : row.status === 'deny' || row.status === 'error' ? 'danger' : 'warning'" size="small">
              {{ row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Duration" prop="duration_ms" width="100">
          <template #default="{ row }">{{ row.duration_ms }}ms</template>
        </el-table-column>
        <el-table-column label="Key Attributes" min-width="300">
          <template #default="{ row }">
            <div v-if="row.attrsList && row.attrsList.length > 0" class="attr-list">
              <span v-for="attr in row.attrsList.slice(0, 4)" :key="attr.k" class="attr-chip">
                <code>{{ attr.k }}: {{ attr.v }}</code>
              </span>
              <el-tag v-if="row.attrsList.length > 4" size="small" effect="plain">+{{ row.attrsList.length - 4 }} more</el-tag>
            </div>
            <span v-else class="muted">-</span>
          </template>
        </el-table-column>
      </el-table>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { ReasoningSpan, ReasoningTrace } from '../types/agent'

const props = defineProps<{
  trace: ReasoningTrace | null | undefined
}>()

function tagType(type: string): string {
  const map: Record<string, string> = {
    request: 'info',
    intent_guard: 'warning',
    planner: '',
    chat_model: 'primary',
    tool_policy: '',
    exec_proxy: '',
    tool_call: 'success',
    audit: 'info',
    decision_normalizer: 'warning',
    diagnosis: '',
    security_report: '',
  }
  return map[type] || ''
}

interface FlatSpan extends ReasoningSpan {
  attrsList: { k: string; v: unknown }[]
}

const flatSpans = computed<FlatSpan[]>(() => {
  if (!props.trace?.spans) return []
  return props.trace.spans.map((s) => {
    const attrsList: { k: string; v: unknown }[] = []
    if (s.attributes) {
      for (const [k, v] of Object.entries(s.attributes)) {
        attrsList.push({ k, v })
      }
    }
    return { ...s, attrsList }
  })
})
</script>

<style scoped>
.trace-header {
  margin-bottom: 8px;
}
.attr-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
.attr-chip code {
  background: var(--el-fill-color-light);
  border-radius: 3px;
  font-size: 11px;
  padding: 1px 5px;
  color: var(--el-text-color-secondary);
}
.muted {
  color: var(--el-text-color-placeholder);
}
</style>
