<template>
  <section v-if="metadata" class="eino-metadata-panel">
    <el-descriptions :column="2" border size="small">
      <el-descriptions-item label="Runtime">
        <el-tag size="small">{{ metadata.runtime || 'unknown' }}</el-tag>
      </el-descriptions-item>
      <el-descriptions-item label="Route">
        {{ metadata.route || 'unknown' }}
      </el-descriptions-item>
      <el-descriptions-item label="Graph Enabled">
        <el-tag :type="metadata.eino_graph_enabled ? 'success' : 'info'" size="small">
          {{ metadata.eino_graph_enabled ? 'Yes' : 'No' }}
        </el-tag>
      </el-descriptions-item>
      <el-descriptions-item label="LLM Enabled">
        <el-tag :type="metadata.llm_enabled ? 'warning' : 'info'" size="small">
          {{ metadata.llm_enabled ? 'Yes' : 'No' }}
        </el-tag>
      </el-descriptions-item>
      <el-descriptions-item label="Chat Model">
        {{ metadata.chat_model || 'unknown' }}
      </el-descriptions-item>
      <el-descriptions-item label="Chat Model Adapter">
        <el-tag size="small">{{ metadata.chat_model_adapter || 'unknown' }}</el-tag>
      </el-descriptions-item>
      <el-descriptions-item label="Orchestration">
        {{ metadata.orchestration || 'unknown' }}
      </el-descriptions-item>
      <el-descriptions-item label="Tool Protocol">
        {{ metadata.tool_protocol || 'unknown' }}
      </el-descriptions-item>
      <el-descriptions-item label="Eino Runtime Version">
        <el-tag type="primary" size="small">{{ metadata.eino_runtime_version || 'unknown' }}</el-tag>
      </el-descriptions-item>
      <el-descriptions-item label="Tools Used">
        <el-tag
          v-for="tool in (metadata.tools_used || [])"
          :key="tool"
          size="small"
          class="tool-tag"
        >
          {{ tool }}
        </el-tag>
        <span v-if="!(metadata.tools_used as string[] || []).length">None</span>
      </el-descriptions-item>
    </el-descriptions>
  </section>
  <el-empty v-else description="No Eino metadata available" :image-size="60" />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { SecurityReport } from '../types/agent'

const props = defineProps<{
  report: SecurityReport | null
}>()

const metadata = computed(() => props.report?.audit_metadata || null)
</script>

<style scoped>
.eino-metadata-panel {
  padding: 16px;
}
.tool-tag {
  margin-right: 4px;
  margin-bottom: 4px;
}
</style>
