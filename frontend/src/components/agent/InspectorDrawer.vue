<template>
  <a-drawer :visible="visible" :width="580" placement="right" @cancel="$emit('close')" :footer="false" :title="'Inspector — ' + (resp?.decision || '')">
    <a-tabs v-if="resp" default-active-tab="overview" size="small">
      <a-tab-pane key="overview" title="Overview">
        <a-descriptions :data="overviewFields" size="mini" :column="1" layout="inline-horizontal" />
      </a-tab-pane>

      <a-tab-pane key="plan" title="Plan">
        <a-timeline v-if="resp.plan?.steps?.length">
          <a-timeline-item v-for="(step, i) in resp.plan.steps" :key="i">
            <div><strong>{{ step.tool_name }}</strong><span v-if="step.reason">: {{ step.reason }}</span></div>
          </a-timeline-item>
        </a-timeline>
        <a-empty v-else description="No plan" />
      </a-tab-pane>

      <a-tab-pane key="tools" title="Tool Calls">
        <a-table v-if="resp.tool_trace?.length" :data="resp.tool_trace" :pagination="false" size="small" :scroll="{ x: 500 }">
          <a-column title="Tool" data-index="tool_name" :width="100"></a-column>
          <a-column title="Op" data-index="operation_type" :width="60"></a-column>
          <a-column title="Resource" data-index="resource_type" :width="90"></a-column>
          <a-column title="Boundary" data-index="boundary_level" :width="80">
            <template #cell="{ record }"><a-tag :color="bc(record.boundary_level)" size="small">{{ record.boundary_level }}</a-tag></template>
          </a-column>
          <a-column title="Policy" data-index="allowed_by_policy" :width="60">
            <template #cell="{ record }">{{ record.allowed_by_policy ? 'Y' : 'N' }}</template>
          </a-column>
          <a-column title="Status" data-index="status" :width="60">
            <template #cell="{ record }"><a-tag :color="record.status === 'ok' ? 'green' : 'red'" size="small">{{ record.status }}</a-tag></template>
          </a-column>
        </a-table>
        <a-empty v-else description="No tool calls" />
      </a-tab-pane>

      <a-tab-pane key="trace" title="Reasoning Trace">
        <div v-if="rtSpans.length" class="rt-list">
          <div v-for="(s, i) in rtSpans" :key="i" class="rt-item" @click="s._open = !s._open">
            <div class="rt-head">
              <a-tag :color="stc(s.type)" size="small" style="min-width:80px">{{ s.type }}</a-tag>
              <span class="rt-name">{{ s.name }}</span>
              <a-tag :color="s.status === 'ok' ? 'green' : 'red'" size="small">{{ s.status }}</a-tag>
              <span class="rt-dur">{{ s.duration_ms }}ms</span>
            </div>
            <div v-if="s._open && s.attributes" class="rt-attrs">
              <a-descriptions :data="sant(s)" size="mini" :column="1" layout="inline-horizontal" />
            </div>
          </div>
        </div>
        <a-empty v-else description="No reasoning trace" />
      </a-tab-pane>

      <a-tab-pane key="evidence" title="Evidence">
        <a-table v-if="evidenceItems.length" :data="evidenceItems" :pagination="false" size="small">
          <a-column title="#" data-index="evidence_id" :width="50"></a-column>
          <a-column title="Tool" data-index="tool_name" :width="100"></a-column>
          <a-column title="Resource" data-index="resource_type" :width="100"></a-column>
          <a-column title="Boundary" data-index="boundary_level" :width="80">
            <template #cell="{ record }"><a-tag :color="bc(record.boundary_level)" size="small">{{ record.boundary_level }}</a-tag></template>
          </a-column>
          <a-column title="Summary" data-index="summary" :ellipsis="true"></a-column>
        </a-table>
        <a-empty v-else description="No evidence chain" />
      </a-tab-pane>

      <a-tab-pane key="raw" title="Raw JSON">
        <pre class="raw-json">{{ sanitized }}</pre>
      </a-tab-pane>
    </a-tabs>
  </a-drawer>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { AgentRunResponse, ReasoningSpan } from '../../types/agent'

const props = defineProps<{
  visible: boolean
  resp: AgentRunResponse | null
}>()

defineEmits<{ close: [] }>()

const sensitiveKeys = ['api_key', 'api-key', 'apikey', 'authorization', 'auth', 'bearer', 'token', 'password', 'passwd', 'secret', 'credential', 'private_key', 'private-key', 'access_key', 'access-key']

function sanitizeVal(obj: any): any {
  if (Array.isArray(obj)) return obj.map(sanitizeVal)
  if (obj && typeof obj === 'object') {
    const r: any = {}
    for (const [k, v] of Object.entries(obj)) {
      if (sensitiveKeys.some(sk => k.toLowerCase().includes(sk))) { r[k] = '[REDACTED]' }
      else { r[k] = sanitizeVal(v) }
    }
    return r
  }
  if (typeof obj === 'string') {
    const vl = obj.toLowerCase()
    if (vl.includes('bearer ') || vl.includes('sk-') || vl.includes('-----begin')) return '[REDACTED]'
  }
  return obj
}

const sanitized = computed(() => {
  return props.resp ? JSON.stringify(sanitizeVal(props.resp), null, 2) : '{}'
})

const overviewFields = computed(() => {
  if (!props.resp) return []
  const m = props.resp.security_report?.audit_metadata || {}
  const f: { label: string; value: any }[] = []
  f.push({ label: 'Decision', value: props.resp.decision })
  f.push({ label: 'Risk Level', value: props.resp.security_report?.risk_level || '-' })
  if (props.resp.summary) f.push({ label: 'Summary', value: props.resp.summary })
  if (props.resp.plan?.scenario) f.push({ label: 'Scenario', value: props.resp.plan.scenario })
  if (m.route) f.push({ label: 'Route', value: m.route })
  if (m.runtime) f.push({ label: 'Runtime', value: m.runtime })
  if (m.llm_enabled !== undefined) f.push({ label: 'LLM Enabled', value: m.llm_enabled ? 'true' : 'false' })
  if (m.remote_llm_used !== undefined) f.push({ label: 'Remote LLM', value: m.remote_llm_used ? 'true' : 'false' })
  if (m.fallback_used !== undefined) f.push({ label: 'Fallback', value: m.fallback_used ? 'true' : 'false' })
  if (m.fallback_reason) f.push({ label: 'Fallback Reason', value: m.fallback_reason })
  return f
})

const rtSpans = computed<ReasoningSpan[]>(() => {
  return (props.resp?.reasoning_trace?.spans ?? []).map(s => ({ ...s, _open: false }))
})

const evidenceItems = computed(() => {
  return props.resp?.security_report?.evidence_chain ?? []
})

function bc(b: string) {
  return b === 'sensitive_system_resource' ? 'red' : b === 'privileged' ? 'orange' : b === 'public' ? 'green' : 'arcoblue'
}

function stc(t: string) {
  const m: Record<string, string> = {
    request: 'arcoblue', intent_guard: 'orange', chat_model: 'purple', planner: 'blue',
    tool_policy: 'cyan', exec_proxy: 'cyan', tool_call: 'green', audit: 'arcoblue',
    decision_normalizer: 'orange', diagnosis: 'blue', security_report: 'blue'
  }
  return m[t] || 'gray'
}

function sant(span: ReasoningSpan) {
  if (!span.attributes) return []
  return Object.entries(span.attributes).map(([key, value]) => {
    let display: any = value
    if (sensitiveKeys.some(sk => key.toLowerCase().includes(sk))) display = '[REDACTED]'
    if (typeof display === 'string' && (display.toLowerCase().includes('bearer ') || display.includes('sk-') || display.includes('-----begin'))) display = '[REDACTED]'
    return { label: key, value: String(display) }
  })
}
</script>

<style scoped>
.raw-json { font-size: 12px; max-height: 500px; overflow: auto; white-space: pre-wrap; word-break: break-all; color: #1d2129; }
.rt-list { font-size: 13px; }
.rt-item { border: 1px solid #e5e6eb; border-radius: 4px; margin-bottom: 6px; cursor: pointer; }
.rt-head { display: flex; align-items: center; gap: 8px; padding: 8px 12px; background: #f7f8fa; }
.rt-name { flex: 1; font-weight: 500; color: #1d2129; }
.rt-dur { color: #4e5969; font-size: 12px; min-width: 44px; font-weight: 500; }
.rt-attrs { padding: 8px 12px; background: #fff; }
</style>
