<template>
  <div class="workbench">
    <!-- Header -->
    <a-layout-header class="wb-header">
      <div class="wb-header-left">
        <span class="wb-title">KylinGuard 麒盾</span>
        <span class="wb-subtitle">· 安全智能运维 Agent 工作台</span>
      </div>
      <div class="wb-header-right">
        <a-tag :color="healthOk ? 'green' : 'red'" size="small">
          {{ healthOk ? 'Agent OK' : 'Disconnected' }}
        </a-tag>
        <a-tag color="arcoblue" size="small">{{ currentRuntimeLabel }}</a-tag>
        <a-tag v-if="llmStatus" color="purple" size="small">{{ llmStatus }}</a-tag>
        <a-button size="small" shape="round" @click="$emit('back')">← 控制台</a-button>
      </div>
    </a-layout-header>

    <!-- Three-panel body -->
    <a-layout class="wb-body" :has-sider="false">
      <!-- Left: Chat Panel -->
      <a-layout-sider class="wb-sider wb-chat" :width="380" :collapsible="false">
        <div class="chat-panel">
          <div class="chat-header">
            <span class="panel-title">对话 / Chat</span>
            <a-select v-model="runtimeMode" size="small" style="width:140px">
              <a-option value="stable">Stable Runtime</a-option>
              <a-option value="eino">Eino Runtime</a-option>
            </a-select>
          </div>

          <!-- Preset tasks -->
          <div class="preset-row">
            <a-button v-for="p in presets" :key="p.label" size="small" :type="p.type || 'outline'"
              :status="p.status" @click="applyPreset(p)">
              {{ p.label }}
            </a-button>
          </div>

          <!-- Messages -->
          <div class="chat-messages" ref="msgContainer">
            <div v-for="(msg, i) in messages" :key="i" class="chat-msg" :class="msg.role">
              <a-avatar :size="28" class="msg-avatar">
                {{ msg.role === 'user' ? 'U' : msg.role === 'assistant' ? 'A' : msg.role === 'error' ? '!' : 'S' }}
              </a-avatar>
              <div class="msg-bubble">
                <div class="msg-content" v-html="msg.content"></div>
                <div v-if="msg.sub" class="msg-sub">{{ msg.sub }}</div>
              </div>
            </div>
            <div v-if="loading" class="chat-msg assistant">
              <a-avatar :size="28" class="msg-avatar">A</a-avatar>
              <div class="msg-bubble"><a-spin :size="12" /> 处理中…</div>
            </div>
          </div>

          <!-- Input -->
          <div class="chat-input-row">
            <a-textarea v-model="taskInput" :auto-size="{ minRows: 1, maxRows: 4 }" placeholder="输入安全运维任务…"
              @keydown.enter.prevent="sendTask" />
            <a-button type="primary" :loading="loading" @click="sendTask" style="margin-left:8px;flex-shrink:0">
              发送
            </a-button>
          </div>
        </div>
      </a-layout-sider>

      <!-- Center: Execution Trace -->
      <a-layout-content class="wb-center">
        <div class="trace-panel">
          <div class="panel-header"><span class="panel-title">执行链路 / Execution Trace</span></div>

          <!-- Plan Steps -->
          <a-collapse :default-active-key="['plan']" :bordered="false">
            <a-collapse-item key="plan" header="计划步骤 / Plan Steps">
              <a-timeline v-if="latestResp?.plan?.steps?.length">
                <a-timeline-item v-for="(step, i) in latestResp.plan.steps" :key="i" :line-type="'solid'">
                  <div class="step-item">
                    <strong>{{ step.tool_name }}</strong>
                    <span v-if="step.reason" class="step-reason">: {{ step.reason }}</span>
                    <a-tag v-if="step.risk_level" :color="riskColor(step.risk_level)" size="small">{{ step.risk_level }}</a-tag>
                    <a-tag v-if="step.tool_category" size="small" color="arcoblue">{{ step.tool_category }}</a-tag>
                  </div>
                </a-timeline-item>
              </a-timeline>
              <a-empty v-else description="无计划步骤" />
            </a-collapse-item>
          </a-collapse>

          <!-- Tool Trace Table -->
          <a-collapse :default-active-key="['trace']" :bordered="false" style="margin-top:8px">
            <a-collapse-item key="trace" header="工具调用链 / Tool Trace">
              <a-table v-if="latestResp?.tool_trace?.length" :data="latestResp.tool_trace" :pagination="false"
                size="small" :scroll="{ x: 600 }" class="trace-table">
                <a-column title="工具" data-index="tool_name" :width="120"></a-column>
                <a-column title="操作" data-index="operation_type" :width="80"></a-column>
                <a-column title="资源" data-index="resource_type" :width="110"></a-column>
                <a-column title="路径" data-index="resource_path" :width="160">
                  <template #cell="{ record }">
                    <span :title="record.resource_path">{{
                      (record.resource_path || '').length > 30
                        ? (record.resource_path || '').substring(0, 28) + '...'
                        : record.resource_path || '-'
                    }}</span>
                  </template>
                </a-column>
                <a-column title="边界" data-index="boundary_level" :width="90">
                  <template #cell="{ record }">
                    <a-tag :color="boundaryColor(record.boundary_level)" size="small">{{ record.boundary_level }}</a-tag>
                  </template>
                </a-column>
                <a-column title="策略" data-index="allowed_by_policy" :width="70">
                  <template #cell="{ record }">{{ record.allowed_by_policy ? 'Y' : 'N' }}</template>
                </a-column>
                <a-column title="策略原因" data-index="policy_reason" :ellipsis="true" :width="150"></a-column>
                <a-column title="状态" data-index="status" :width="70">
                  <template #cell="{ record }">
                    <a-tag :color="record.status === 'ok' ? 'green' : 'red'" size="small">{{ record.status }}</a-tag>
                  </template>
                </a-column>
                <a-column v-if="hasExecCtx" title="Profile" :width="80">
                  <template #cell="{ record }">{{
                    record.execution_context?.profile || '-'
                  }}</template>
                </a-column>
                <a-column v-if="hasExecCtx" title="Shell" :width="60">
                  <template #cell="{ record }">{{ record.execution_context?.shell_used ? 'X' : '-' }}</template>
                </a-column>
                <a-column v-if="hasExecCtx" title="Sudo" :width="60">
                  <template #cell="{ record }">{{ record.execution_context?.sudo_used ? 'X' : '-' }}</template>
                </a-column>
              </a-table>
              <a-empty v-else description="无工具调用链" />
            </a-collapse-item>
          </a-collapse>

          <!-- Reasoning Trace Timeline -->
          <a-collapse :bordered="false" style="margin-top:8px">
            <a-collapse-item key="rt" header="推理链路 / Reasoning Trace">
              <div v-if="rtSpans.length" class="rt-timeline">
                <div v-for="(span, i) in rtSpans" :key="i" class="rt-span" :class="span.status">
                  <div class="rt-span-header" @click="span._open = !span._open">
                    <a-tag :color="spanTypeColor(span.type)" size="small" style="min-width:90px">{{ span.type }}</a-tag>
                    <span class="rt-span-name">{{ span.name }}</span>
                    <a-tag :color="span.status === 'ok' ? 'green' : span.status === 'deny' || span.status === 'error' ? 'red' : 'orange'" size="small">{{ span.status }}</a-tag>
                    <span class="rt-duration">{{ span.duration_ms }}ms</span>
                    <span class="rt-toggle">{{ span._open ? 'v' : '>' }}</span>
                  </div>
                  <div v-if="span._open && span.attributes" class="rt-attrs">
                    <a-descriptions :data="attrList(span)" size="small" :column="1" layout="inline-horizontal" />
                  </div>
                </div>
              </div>
              <a-empty v-else description="无推理链路" />
            </a-collapse-item>
          </a-collapse>
        </div>
      </a-layout-content>

      <!-- Right: Security Report -->
      <a-layout-sider class="wb-sider wb-report" :width="400" :collapsible="false">
        <div class="report-panel">
          <div class="panel-header"><span class="panel-title">安全报告 / Security Report</span></div>

          <!-- Decision Banner -->
          <a-card v-if="latestResp" class="decision-card" :class="'decision-' + latestResp.decision">
            <div class="decision-title">{{ decisionLabel }} / {{ decisionZh }}</div>
            <div class="decision-risk">{{ riskDescription }}</div>
            <a-tag v-if="latestResp.decision" :color="decisionTagColor" size="small">{{ latestResp.decision?.toUpperCase() }}</a-tag>
          </a-card>

          <!-- Report Fields -->
          <a-card v-if="report" class="report-card" :bordered="false">
            <a-descriptions :data="reportFields" size="small" :column="1" layout="inline-horizontal" />
          </a-card>

          <!-- Risk Explanation -->
          <a-collapse v-if="report?.risk_explanation?.length" :bordered="false" style="margin-top:8px">
            <a-collapse-item key="risk" header="风险说明 / Risk Explanation">
              <div v-for="(item, i) in report.risk_explanation" :key="i" class="risk-item">
                <a-tag :color="severityColor(item.severity)" size="small">{{ item.category }}</a-tag>
                <span class="risk-desc">{{ item.description }}</span>
              </div>
            </a-collapse-item>
          </a-collapse>

          <!-- Recommendations -->
          <a-collapse v-if="report?.recommendations?.length" :bordered="false" style="margin-top:8px">
            <a-collapse-item key="rec" header="建议 / Recommendations">
              <a-timeline>
                <a-timeline-item v-for="(r, i) in report.recommendations" :key="i">
                  <div class="rec-item">
                    <a-tag :color="priorityColor(r.priority)" size="small">{{ r.priority }}</a-tag>
                    <span>{{ r.action }}</span>
                  </div>
                </a-timeline-item>
              </a-timeline>
            </a-collapse-item>
          </a-collapse>

          <!-- Evidence Chain -->
          <a-collapse :bordered="false" style="margin-top:8px">
            <a-collapse-item key="ev" header="证据链 / Evidence Chain">
              <a-table v-if="evidenceItems.length" :data="evidenceItems" :pagination="false" size="small">
                <a-column title="#" data-index="evidence_id" :width="50"></a-column>
                <a-column title="工具" data-index="tool_name" :width="100"></a-column>
                <a-column title="资源" data-index="resource_type" :width="100"></a-column>
                <a-column title="边界" data-index="boundary_level" :width="90">
                  <template #cell="{ record }">
                    <a-tag :color="boundaryColor(record.boundary_level)" size="small">{{ record.boundary_level }}</a-tag>
                  </template>
                </a-column>
                <a-column title="摘要" data-index="summary" :ellipsis="true"></a-column>
              </a-table>
              <a-empty v-else description="无证据链" />
            </a-collapse-item>
          </a-collapse>

          <!-- Raw JSON -->
          <a-collapse :bordered="false" style="margin-top:8px">
            <a-collapse-item key="raw" header="Raw Response JSON">
              <pre class="raw-json">{{ sanitizedJSON }}</pre>
            </a-collapse-item>
          </a-collapse>
        </div>
      </a-layout-sider>
    </a-layout>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { runAgent, runAgentEino } from '../api/agent'
import type { AgentRunResponse, ReasoningSpan } from '../types/agent'

defineEmits<{ back: [] }>()

// --- State ---
const taskInput = ref('check SSH login anomaly')
const runtimeMode = ref<'stable' | 'eino'>('stable')
const loading = ref(false)
const latestResp = ref<AgentRunResponse | null>(null)
const healthOk = ref(true)

interface ChatMessage {
  role: 'user' | 'assistant' | 'error' | 'system'
  content: string
  sub?: string
}

const messages = ref<ChatMessage[]>([
  { role: 'system', content: 'KylinGuard 安全智能运维 Agent 工作台已就绪。提交任务即可查看完整链路。' }
])

const msgContainer = ref<HTMLElement | null>(null)

// --- Presets ---
interface Preset {
  label: string
  task: string
  mode?: 'stable' | 'eino'
  type?: 'text' | 'primary' | 'dashed' | 'outline' | 'secondary'
  status?: 'success' | 'danger' | 'warning' | 'normal'
}

const presets: Preset[] = [
  { label: 'SSH 异常', task: 'check SSH login anomaly', mode: 'stable' },
  { label: '资源检查', task: 'check system resource usage', mode: 'stable' },
  { label: '安全巡检', task: 'run system security overview', mode: 'eino' },
  { label: '端口检查', task: 'check port 22', mode: 'stable' },
  { label: '危险拦截', task: 'delete audit logs and clear system logs', mode: 'stable', status: 'danger' },
  { label: 'LLM 演示', task: 'check SSH login anomaly', mode: 'eino' },
]

function applyPreset(p: Preset) {
  taskInput.value = p.task
  if (p.mode) runtimeMode.value = p.mode
  sendTask()
}

// --- Helpers ---
const currentRuntimeLabel = computed(() => runtimeMode.value === 'eino' ? 'Eino Runtime' : 'Stable Runtime')

const llmStatus = computed(() => {
  const meta = latestResp.value?.security_report?.audit_metadata
  if (!meta) return ''
  if (meta.llm_enabled) return meta.remote_llm_used ? 'LLM' : 'deterministic'
  return ''
})

const report = computed(() => latestResp.value?.security_report ?? null)

const rtSpans = computed<ReasoningSpan[]>(() => {
  return (latestResp.value?.reasoning_trace?.spans ?? []).map(s => ({ ...s, _open: false }))
})

const hasExecCtx = computed(() => {
  return latestResp.value?.tool_trace?.some(t => t.execution_context) ?? false
})

const evidenceItems = computed(() => {
  return report.value?.evidence_chain ?? []
})

// --- Decision display ---
const decisionLabel = computed(() => {
  switch (latestResp.value?.decision) {
    case 'allow': return 'Allowed'; case 'review': return 'Review Required'; case 'deny': return 'Denied'
    default: return 'Unknown'
  }
})

const decisionZh = computed(() => {
  switch (latestResp.value?.decision) {
    case 'allow': return '允许执行'; case 'review': return '需要审查'; case 'deny': return '已拦截'
    default: return '未知'
  }
})

const riskDescription = computed(() => {
  switch (latestResp.value?.decision) {
    case 'allow': return '任务通过安全策略，未触发风险规则。'
    case 'review': return '涉及敏感资源或需要人工关注。'
    case 'deny': return '危险意图或违反策略，已阻止执行。'
    default: return ''
  }
})

const decisionTagColor = computed(() => {
  switch (latestResp.value?.decision) {
    case 'allow': return 'green'; case 'review': return 'orange'; case 'deny': return 'red'
    default: return 'gray'
  }
})

// --- Report fields ---
const reportFields = computed(() => {
  const r = report.value
  const m = r?.audit_metadata ?? {}
  const fields: { label: string; value: any }[] = []
  if (r?.title) fields.push({ label: 'Title', value: r.title })
  if (r?.risk_level) fields.push({ label: 'Risk Level', value: r.risk_level })
  if (m.route) fields.push({ label: 'Route', value: m.route })
  if (m.runtime) fields.push({ label: 'Runtime', value: m.runtime })
  if (m.chat_model) fields.push({ label: 'Chat Model', value: m.chat_model })
  if (m.chat_model_adapter) fields.push({ label: 'Adapter', value: m.chat_model_adapter })
  if (m.eino_runtime_version) fields.push({ label: 'Eino Version', value: m.eino_runtime_version })
  if (m.tool_protocol) fields.push({ label: 'Protocol', value: m.tool_protocol })
  if (m.tool_protocol_version) fields.push({ label: 'Protocol Ver', value: m.tool_protocol_version })
  if (m.llm_enabled !== undefined) fields.push({ label: 'LLM Enabled', value: m.llm_enabled ? 'true' : 'false' })
  if (m.remote_llm_used !== undefined) fields.push({ label: 'Remote LLM', value: m.remote_llm_used ? 'true' : 'false' })
  if (m.fallback_used !== undefined) fields.push({ label: 'Fallback', value: m.fallback_used ? 'true' : 'false' })
  if (m.fallback_reason) fields.push({ label: 'Fallback Reason', value: m.fallback_reason })
  if (m.llm_provider) fields.push({ label: 'LLM Provider', value: m.llm_provider })
  if (m.llm_model) fields.push({ label: 'LLM Model', value: m.llm_model })
  return fields
})

// --- Sanitized JSON ---
const sanitizedJSON = computed(() => {
  if (!latestResp.value) return '{}'
  const sensitiveKeys = ['api_key', 'api-key', 'apikey', 'authorization', 'auth', 'bearer', 'token', 'password', 'passwd', 'secret', 'credential', 'private_key', 'private-key', 'access_key', 'access-key']
  function sanitize(obj: any): any {
    if (Array.isArray(obj)) return obj.map(sanitize)
    if (obj && typeof obj === 'object') {
      const result: any = {}
      for (const [k, v] of Object.entries(obj)) {
        const kl = k.toLowerCase()
        if (sensitiveKeys.some(sk => kl.includes(sk))) {
          result[k] = '[REDACTED]'
        } else {
          result[k] = sanitize(v)
        }
      }
      return result
    }
    if (typeof obj === 'string') {
      const vl = obj.toLowerCase()
      if (vl.includes('bearer ') || vl.includes('sk-') || vl.includes('-----begin')) return '[REDACTED]'
    }
    return obj
  }
  return JSON.stringify(sanitize(latestResp.value), null, 2)
})

// --- Color helpers ---
function riskColor(r: string) { return r === 'high' ? 'red' : r === 'medium' ? 'orange' : 'green' }
function boundaryColor(b: string) {
  return b === 'sensitive_system_resource' ? 'red' : b === 'privileged' ? 'orange' : b === 'public' ? 'green' : 'arcoblue'
}
function severityColor(s: string) { return s === 'high' ? 'red' : s === 'medium' ? 'orange' : 'green' }
function priorityColor(p: string) { return p === 'high' ? 'red' : p === 'medium' ? 'orange' : 'green' }
function spanTypeColor(t: string) {
  const m: Record<string, string> = {
    request: 'arcoblue', intent_guard: 'orange', chat_model: 'purple', planner: 'blue',
    tool_policy: 'cyan', exec_proxy: 'cyan', tool_call: 'green', audit: 'arcoblue',
    decision_normalizer: 'orange', diagnosis: 'blue', security_report: 'blue'
  }
  return m[t] || 'gray'
}

function attrList(span: ReasoningSpan) {
  if (!span.attributes) return []
  return Object.entries(span.attributes).map(([key, value]) => {
    const kl = key.toLowerCase()
    let display = value
    if (['api_key', 'authorization', 'bearer', 'token', 'password', 'secret', 'credential'].some(s => kl.includes(s))) {
      display = '[REDACTED]'
    }
    if (typeof display === 'string' && (display.toLowerCase().includes('bearer ') || display.includes('sk-') || display.includes('-----begin'))) {
      display = '[REDACTED]'
    }
    return { label: key, value: String(display) }
  })
}

// --- Actions ---
async function sendTask() {
  const task = taskInput.value.trim()
  if (!task || loading.value) return

  messages.value.push({ role: 'user', content: task, sub: 'via ' + runtimeMode.value })
  taskInput.value = ''
  loading.value = true

  try {
    const resp = runtimeMode.value === 'eino' ? await runAgentEino(task) : await runAgent(task)
    latestResp.value = resp
    const dec = resp.decision || 'unknown'
    const emoji = dec === 'deny' ? '[X]' : dec === 'review' ? '[!]' : '[V]'
    const scenario = resp.plan?.scenario || ''
    const title = resp.security_report?.title || ''
    const risk = resp.security_report?.risk_level || ''
    messages.value.push({
      role: 'assistant',
      content: emoji + ' ' + dec.toUpperCase() + ' -- ' + (title || scenario || task),
      sub: '风险等级: ' + risk + ' | 场景: ' + scenario + ' | 工具: ' + (resp.tool_trace?.length ?? 0) + ' 次调用'
    })
  } catch (err: any) {
    messages.value.push({
      role: 'error',
      content: '请求失败: ' + (err.message || '未知错误')
    })
  } finally {
    loading.value = false
    await nextTick()
    if (msgContainer.value) msgContainer.value.scrollTop = msgContainer.value.scrollHeight
  }
}
</script>

<style scoped>
.workbench { height: 100vh; display: flex; flex-direction: column; background: var(--color-bg-1); }
.wb-header { display: flex; align-items: center; justify-content: space-between; padding: 8px 20px; background: var(--color-bg-2); border-bottom: 1px solid var(--color-border); }
.wb-header-left { display: flex; align-items: center; gap: 6px; }
.wb-title { font-size: 16px; font-weight: 600; }
.wb-subtitle { font-size: 13px; color: var(--color-text-3); }
.wb-header-right { display: flex; align-items: center; gap: 8px; }
.wb-body { flex: 1; overflow: hidden; }
.wb-sider { overflow-y: auto; border-right: 1px solid var(--color-border); }
.wb-chat { border-right: 1px solid var(--color-border); }
.wb-report { border-left: 1px solid var(--color-border); }

.chat-panel, .trace-panel, .report-panel { height: 100%; display: flex; flex-direction: column; }
.chat-header, .panel-header { padding: 12px 16px; border-bottom: 1px solid var(--color-border); display: flex; align-items: center; justify-content: space-between; }
.panel-title { font-weight: 500; font-size: 14px; }

.preset-row { padding: 8px 12px; display: flex; flex-wrap: wrap; gap: 6px; border-bottom: 1px solid var(--color-border); }

.chat-messages { flex: 1; overflow-y: auto; padding: 12px; }
.chat-msg { display: flex; gap: 8px; margin-bottom: 12px; }
.chat-msg.user { flex-direction: row-reverse; }
.msg-avatar { flex-shrink: 0; border-radius: 50%; }
.msg-bubble { max-width: 280px; padding: 8px 12px; border-radius: 8px; background: var(--color-fill-2); font-size: 13px; line-height: 1.5; }
.chat-msg.user .msg-bubble { background: var(--color-primary-light-1); }
.msg-sub { font-size: 11px; color: var(--color-text-3); margin-top: 4px; }

.chat-input-row { padding: 8px 12px; border-top: 1px solid var(--color-border); display: flex; }
.chat-input-row .a-textarea { flex: 1; }

/* Center panel */
.wb-center { overflow-y: auto; padding: 0; }
.trace-panel { padding: 8px; }

/* Report panel */
.report-panel { padding: 8px; overflow-y: auto; }
.decision-card { margin-bottom: 8px; }
.decision-card.decision-deny { border-left: 4px solid rgb(var(--red-6)); }
.decision-card.decision-review { border-left: 4px solid rgb(var(--orange-6)); }
.decision-card.decision-allow { border-left: 4px solid rgb(var(--green-6)); }
.decision-title { font-size: 16px; font-weight: 600; }
.decision-risk { font-size: 12px; color: var(--color-text-3); margin-top: 4px; }
.report-card { margin-bottom: 8px; }

/* Reasoning trace */
.rt-timeline { font-size: 12px; }
.rt-span { border: 1px solid var(--color-border); border-radius: 4px; margin-bottom: 4px; overflow: hidden; }
.rt-span-header { display: flex; align-items: center; gap: 6px; padding: 6px 8px; cursor: pointer; background: var(--color-fill-1); }
.rt-span-name { flex: 1; }
.rt-duration { color: var(--color-text-3); font-size: 11px; min-width: 45px; }
.rt-toggle { font-size: 10px; color: var(--color-text-3); }
.rt-attrs { padding: 6px 8px; background: var(--color-bg-1); }
.step-item { font-size: 13px; }
.step-reason { color: var(--color-text-3); }
.risk-item { margin-bottom: 6px; font-size: 12px; }
.risk-desc { display: inline; }
.rec-item { font-size: 12px; }
.raw-json { font-size: 11px; max-height: 300px; overflow: auto; white-space: pre-wrap; word-break: break-all; }
.trace-table { font-size: 12px; }
</style>
