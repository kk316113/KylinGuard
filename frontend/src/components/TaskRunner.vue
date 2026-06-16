<template>
  <section class="console-panel task-runner">
    <div class="panel-heading">
      <span>任务输入</span>
      <el-tag size="small" effect="plain">Go Agent API</el-tag>
    </div>

    <el-input
      v-model="localTask"
      type="textarea"
      :rows="5"
      resize="none"
      placeholder="输入安全运维任务，例如：检查当前系统 SSH 登录异常"
    />

    <div class="mode-row">
      <el-segmented v-model="localMode" :options="modeOptions" />
    </div>

    <div class="sample-grid">
      <el-button
        v-for="sample in samples"
        :key="sample"
        size="small"
        plain
        @click="localTask = sample"
      >
        {{ sample }}
      </el-button>
    </div>

    <el-button
      class="run-button"
      type="primary"
      :loading="loading"
      :disabled="!localTask.trim()"
      @click="emitRun"
    >
      <el-icon><VideoPlay /></el-icon>
      运行诊断
    </el-button>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { VideoPlay } from '@element-plus/icons-vue'
import type { RuntimeMode } from '../types/agent'

const props = defineProps<{
  task: string
  mode: RuntimeMode
  loading: boolean
}>()

const emit = defineEmits<{
  'update:task': [value: string]
  'update:mode': [value: RuntimeMode]
  run: []
}>()

const localTask = computed({
  get: () => props.task,
  set: (value: string) => emit('update:task', value)
})

const localMode = computed({
  get: () => props.mode,
  set: (value: string | number | boolean) => emit('update:mode', value as RuntimeMode)
})

const modeOptions = [
  { label: 'Stable Runtime', value: 'stable' },
  { label: 'Eino Fallback', value: 'eino' }
]

const samples = [
  '检查当前系统 SSH 登录异常',
  '检查 sshd 服务状态',
  '检查 22 端口是否开放',
  '检查当前系统资源使用情况',
  '检查当前系统网络连接',
  '检查 sshd 进程状态',
  '查看 sshd 最近日志',
  '执行一次系统安全巡检',
  'delete audit logs and clear system logs'
]

function emitRun() {
  if (!localTask.value.trim()) return
  emit('run')
}
</script>
