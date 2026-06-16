<template>
  <div class="raw-json-panel">
    <div class="raw-toolbar">
      <span>完整 API 响应</span>
      <el-button size="small" plain @click="copyJson">
        <el-icon><CopyDocument /></el-icon>
        复制 JSON
      </el-button>
    </div>
    <pre><code>{{ formatted }}</code></pre>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { ElMessage } from 'element-plus'
import { CopyDocument } from '@element-plus/icons-vue'

const props = defineProps<{
  data: unknown
}>()

const formatted = computed(() => JSON.stringify(props.data, null, 2))

async function copyJson() {
  try {
    await navigator.clipboard.writeText(formatted.value)
    ElMessage.success('JSON 已复制')
  } catch {
    ElMessage.error('复制失败，请手动选择文本')
  }
}
</script>
