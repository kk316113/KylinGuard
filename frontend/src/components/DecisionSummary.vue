<template>
  <section class="console-panel decision-summary">
    <div class="panel-heading">
      <span>审计摘要</span>
      <el-tag :type="decisionType" effect="dark">{{ decisionLabel }}</el-tag>
    </div>

    <div v-if="response" class="summary-stack">
      <div class="metric-grid">
        <div class="metric-box">
          <span class="metric-label">Decision</span>
          <strong :class="`decision-${response.decision}`">{{ response.decision }}</strong>
        </div>
        <div class="metric-box">
          <span class="metric-label">Diagnosis Risk</span>
          <strong :class="`risk-${riskLevel}`">{{ riskLevel }}</strong>
        </div>
      </div>

      <div class="summary-item">
        <span class="summary-label">Audit Method</span>
        <el-tag :type="methodType" effect="plain">{{ response.audit_result?.method || 'unknown' }}</el-tag>
      </div>

      <div class="summary-item">
        <span class="summary-label">Audit Message</span>
        <p>{{ response.audit_result?.message || 'No audit message.' }}</p>
      </div>

      <div class="summary-item">
        <span class="summary-label">Report</span>
        <h3>{{ response.security_report?.title || 'No security report' }}</h3>
        <p>{{ response.security_report?.summary || response.summary }}</p>
      </div>
    </div>

    <div v-else class="summary-placeholder">
      运行任务后将在这里展示 decision、TraceShield 审计摘要和安全报告。
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { AgentRunResponse } from '../types/agent'

const props = defineProps<{
  response: AgentRunResponse | null
}>()

const decisionType = computed(() => {
  switch (props.response?.decision) {
    case 'allow':
      return 'success'
    case 'deny':
      return 'danger'
    case 'review':
      return 'warning'
    default:
      return 'info'
  }
})

const decisionLabel = computed(() => {
  switch (props.response?.decision) {
    case 'allow':
      return '允许'
    case 'deny':
      return '已拦截'
    case 'review':
      return '需审计复核'
    default:
      return '等待任务'
  }
})

const riskLevel = computed(() => props.response?.diagnosis?.risk_level || props.response?.security_report?.risk_level || 'unknown')

const methodType = computed(() => {
  const method = props.response?.audit_result?.method
  if (method === 'intent_guard') return 'danger'
  if (method === 'traceshield') return 'success'
  return 'info'
})
</script>
