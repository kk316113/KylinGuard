<template>
  <header class="health-bar">
    <div class="brand-block">
      <div class="brand-mark">KylinGuard / 麒盾</div>
      <div class="brand-title">KylinGuard Security Console</div>
      <div class="brand-subtitle">面向麒麟操作系统的安全智能运维 Agent</div>
    </div>

    <div class="status-strip">
      <el-tag :type="healthType" effect="dark" round>
        Go Agent: {{ healthLabel }}
      </el-tag>
      <el-tag :type="mode === 'eino' ? 'warning' : 'success'" effect="plain" round>
        {{ mode === 'eino' ? 'Eino Fallback' : 'Stable Runtime' }}
      </el-tag>
      <el-tag class="trace-tag" effect="dark" round>TraceShield</el-tag>
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { HealthResponse, RuntimeMode } from '../types/agent'

const props = defineProps<{
  health: HealthResponse | null
  mode: RuntimeMode
  healthError?: string
}>()

const healthLabel = computed(() => {
  if (props.health?.status === 'ok') return 'ok'
  if (props.healthError) return 'unreachable'
  return 'checking'
})

const healthType = computed(() => {
  if (props.health?.status === 'ok') return 'success'
  if (props.healthError) return 'danger'
  return 'info'
})
</script>
