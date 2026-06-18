<template>
  <div class="running-card">
    <div class="running-line">
      <a-spin :size="18" />
      <span class="running-text">{{ currentText }}</span>
    </div>
    <a-steps :current="step" size="small" style="margin-top:12px;max-width:480px">
      <a-step description="正在理解任务..." />
      <a-step description="正在执行安全受控检查..." />
      <a-step description="正在整理回答..." />
      <a-step description="正在附加安全审计..." />
    </a-steps>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ step: number }>()

const currentText = computed(() => {
  if (props.step <= 1) return '正在理解任务...'
  if (props.step === 2) return '正在执行安全受控检查...'
  return '正在整理回答...'
})
</script>

<style scoped>
.running-card { background: #f7f8fa; padding: 18px; border-radius: 12px; border: 1px solid #e5e6eb; max-width: 520px; }
.running-line { display: flex; align-items: center; gap: 10px; }
.running-text { font-size: 15px; font-weight: 600; color: #1d2129; }
</style>
