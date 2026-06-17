<template>
  <div class="running-card">
    <div class="running-line">
      <a-spin :size="18" />
      <span class="running-text">Agent 正在执行…</span>
    </div>
    <a-steps :current="step" size="small" style="margin-top:12px;max-width:480px">
      <a-step description="理解运维任务" />
      <a-step description="选择最小权限工具" />
      <a-step description="执行受控系统检查" />
      <a-step description="TraceShield 审计" />
      <a-step description="生成安全结论" />
    </a-steps>
  </div>
</template>

<script setup lang="ts">
defineProps<{ step: number }>()
</script>

<style scoped>
.running-card { background: #f7f8fa; padding: 18px; border-radius: 12px; border: 1px solid #e5e6eb; max-width: 520px; }
.running-line { display: flex; align-items: center; gap: 10px; }
.running-text { font-size: 15px; font-weight: 600; color: #1d2129; }
</style>
