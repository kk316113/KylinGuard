<template>
  <div class="kg-shell">
    <header class="runtime-bar">
      <div class="brand-block">
        <div class="brand-mark">KG</div>
        <div>
          <div class="brand-title">KylinGuard Agent Console</div>
          <div class="brand-subtitle">Secure Kylin OS operations workspace</div>
        </div>
      </div>
      <div class="runtime-metrics">
        <a-tag :color="serviceColor(runtimeStatus?.services.go_agent.status)" size="small">
          Go Agent {{ runtimeStatus?.services.go_agent.status || 'unknown' }}
        </a-tag>
        <a-tag :color="serviceColor(runtimeStatus?.services.audit_core.status)" size="small">
          audit-core {{ runtimeStatus?.services.audit_core.status || 'unknown' }}
        </a-tag>
        <a-tag :color="runtimeModeColor" size="small">{{ runtimeModeLabel }}</a-tag>
        <a-tag v-if="runtimeStatus?.runtime.model" color="blue" size="small">{{ runtimeStatus.runtime.model }}</a-tag>
        <a-tag v-for="layer in securityLayerTags" :key="layer.name" color="green" size="small">
          {{ layer.name }} {{ layer.status }}
        </a-tag>
      </div>
    </header>

    <main class="workspace">
      <aside class="task-sidebar">
        <section class="sidebar-section">
          <button class="new-task-btn" @click="newSession">+ New Task</button>
          <div class="mode-card">
            <span class="section-label">Runtime</span>
            <strong>{{ runtimeModeLabel }}</strong>
            <span>{{ runtimeStatus?.runtime.chat_model || 'waiting for status' }}</span>
          </div>
        </section>

        <section class="sidebar-section">
          <div class="section-title">Suggested Ops Prompts</div>
          <button v-for="prompt in promptSuggestions" :key="prompt" class="prompt-item" @click="applySuggestion(prompt)">
            {{ prompt }}
          </button>
        </section>

        <section class="sidebar-section">
          <div class="section-title">Recent Runs</div>
          <div v-if="recentRuns.length === 0" class="empty-note">No task session yet.</div>
          <button v-for="run in recentRuns" :key="run.id" class="recent-item" @click="showRun(run.response)">
            <span>{{ run.title }}</span>
            <a-tag :color="decisionColor(run.decision)" size="small">{{ run.decision }}</a-tag>
          </button>
        </section>

        <section class="sidebar-section">
          <div class="section-title">Acceptance</div>
          <div class="acceptance-line">
            <strong>{{ passedStages }}</strong>
            <span>verified stages</span>
          </div>
          <div v-if="acceptanceSummary?.stages?.length" class="stage-list">
            <span v-for="stage in acceptanceSummary.stages.slice(0, 5)" :key="stage.name">{{ stage.name }}</span>
          </div>
        </section>
      </aside>

      <section class="agent-workspace">
        <div class="workspace-head">
          <div>
            <div class="workspace-title">Operations Task Session</div>
            <div class="workspace-subtitle">Type any real operations problem. Prompts are text only, not scenario IDs.</div>
          </div>
          <a-segmented v-model="runtimeMode" :options="runtimeOpts" size="small" />
        </div>

        <div ref="scrollRef" class="chat-scroll">
          <div v-if="messages.length === 0" class="empty-state">
            <h2>What should KylinGuard diagnose?</h2>
            <p>The Agent Loop chooses safe next actions, executes controlled tools, and returns a final answer with evidence.</p>
          </div>

          <div v-for="(msg, i) in messages" :key="i" :class="['msg-row', msg.role]">
            <div v-if="msg.role === 'user'" class="msg user-msg">
              <div class="msg-text">{{ msg.content }}</div>
              <div class="msg-meta">{{ msg.sub }}</div>
            </div>

            <div v-else-if="msg.role === 'assistant'" class="msg assistant-msg">
              <div v-if="msg.session" class="session-strip">
                <span>Task {{ msg.session.taskId || 'kg-pending' }}</span>
                <span>{{ sceneLabel(msg.session.sceneType) }}</span>
                <span>{{ msg.session.runStatus || 'completed' }}</span>
                <span>{{ msg.runtimeBadge?.label || 'Agent Runtime' }}</span>
              </div>
              <div v-if="msg.runtimeBadge" class="runtime-line">
                <a-tag :color="msg.runtimeBadge.color" size="small">{{ msg.runtimeBadge.label }}</a-tag>
                <a-tag v-if="msg.runtimeBadge.chatModel" color="gray" size="small">{{ msg.runtimeBadge.chatModel }}</a-tag>
              </div>
              <div class="msg-text">{{ msg.content }}</div>
              <div v-if="msg.resultSummary" class="result-strip">
                <span>Steps {{ msg.resultSummary.agentSteps }}</span>
                <span>Evidence {{ msg.resultSummary.toolTrace }}</span>
                <span>Decision {{ msg.resultSummary.decision }}</span>
                <span>Report {{ msg.resultSummary.auditReady ? 'ready' : 'pending' }}</span>
              </div>
              <DecisionCard v-if="msg.decision" v-bind="msg.decision" />

              <div v-if="msg.agentSteps && msg.agentSteps.length > 0" class="step-timeline">
                <div class="panel-title">Agent execution timeline</div>
                <div v-for="(step, si) in msg.agentSteps" :key="si" class="step-card">
                  <div class="step-header">
                    <span class="step-num">#{{ step.step_index || si + 1 }}</span>
                    <strong>{{ step.tool_name || step.action_type || 'agent_step' }}</strong>
                    <a-tag v-if="step.policy_decision" :color="decisionColor(step.policy_decision)" size="small">
                      {{ step.policy_decision }}
                    </a-tag>
                  </div>
                  <div v-if="step.user_visible_summary || step.reason" class="step-summary">
                    {{ step.user_visible_summary || step.reason }}
                  </div>
                  <div v-if="observationSummary(step)" class="step-observation">
                    Observation: {{ observationSummary(step) }}
                  </div>
                  <div class="step-semantic">
                    <span v-if="step.operation_type">operation={{ step.operation_type }}</span>
                    <span v-if="step.resource_type">resource={{ step.resource_type }}</span>
                    <span v-if="step.boundary_level">boundary={{ step.boundary_level }}</span>
                  </div>
                </div>
              </div>

              <a-button v-if="msg.hasInspector" size="mini" type="outline" @click="openInspector">Open Inspector</a-button>
            </div>

            <div v-else-if="msg.role === 'error'" class="msg error-msg">
              <a-alert type="error" :title="msg.content" :closable="false" />
            </div>
          </div>

          <div v-if="running" class="msg-row assistant">
            <div class="msg assistant-msg">
              <AgentRunningNarrative :step="runStep" />
            </div>
          </div>
        </div>

        <div class="composer">
          <a-textarea
            v-model="taskInput"
            :auto-size="{ minRows: 1, maxRows: 4 }"
            placeholder="Describe a Kylin operations problem..."
            @keydown.enter.prevent="send"
          />
          <a-button type="primary" :loading="running" @click="send">Send</a-button>
        </div>
      </section>

      <aside class="insight-panel">
        <div class="insight-head">
          <strong>Insight Panel</strong>
          <a-button size="mini" type="outline" @click="$emit('back')">Classic Console</a-button>
        </div>
        <a-tabs v-model:active-key="activeInsightTab" size="small">
          <a-tab-pane key="steps" title="Steps">
            <div v-if="lastResponse?.agent_steps?.length" class="insight-list">
              <div v-for="(step, idx) in lastResponse.agent_steps" :key="idx" class="insight-card">
                <strong>#{{ step.step_index || idx + 1 }} {{ step.tool_name || step.action_type }}</strong>
                <span>{{ step.user_visible_summary || step.reason || observationSummary(step) }}</span>
              </div>
            </div>
            <a-empty v-else description="No agent steps yet" />
          </a-tab-pane>

          <a-tab-pane key="evidence" title="Evidence">
            <div v-if="lastResponse?.tool_trace?.length" class="insight-list">
              <div v-for="trace in lastResponse.tool_trace" :key="trace.step_id" class="insight-card">
                <strong>{{ trace.tool_name }}</strong>
                <span>{{ trace.output_summary }}</span>
                <small>{{ trace.operation_type }} / {{ trace.resource_type }} / {{ trace.boundary_level }}</small>
              </div>
            </div>
            <a-empty v-else description="No tool evidence yet" />
          </a-tab-pane>

          <a-tab-pane key="audit" title="Audit">
            <div v-if="lastResponse" class="audit-summary">
              <a-descriptions :data="auditFields" :column="1" size="mini" layout="inline-horizontal" />
            </div>
            <a-empty v-else description="No audit result yet" />
          </a-tab-pane>

          <a-tab-pane key="tools" title="Tools">
            <div v-if="capabilities?.available_tools?.length" class="tool-list">
              <div v-for="tool in capabilities.available_tools" :key="tool.tool_name" class="tool-row">
                <strong>{{ tool.tool_name }}</strong>
                <span>{{ tool.operation_type }} / {{ tool.resource_type }} / {{ tool.boundary_level }}</span>
              </div>
            </div>
            <a-empty v-else description="Capabilities unavailable" />
          </a-tab-pane>

          <a-tab-pane key="report" title="Report">
            <div v-if="lastResponse" class="report-card">
              <a-descriptions :data="reportFields" :column="1" size="mini" layout="inline-horizontal" />
              <div class="report-answer">{{ shortText(lastResponse.final_answer || lastResponse.summary || '') }}</div>
            </div>
            <a-empty v-else description="Run a task to create report summary" />
          </a-tab-pane>
        </a-tabs>
      </aside>
    </main>

    <InspectorDrawer
      :visible="inspectorVisible"
      :resp="inspectorResp"
      :initial-tab="inspectorTab"
      @close="inspectorVisible = false"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import {
  getAcceptanceSummary,
  getAgentCapabilities,
  getRuntimeStatus,
  runAgent,
  runAgentEino
} from '../api/agent'
import type {
  AcceptanceSummaryResponse,
  AgentCapabilitiesResponse,
  AgentRunResponse,
  AgentStep,
  Decision,
  RuntimeStatusResponse
} from '../types/agent'
import DecisionCard from '../components/agent/DecisionCard.vue'
import InspectorDrawer from '../components/agent/InspectorDrawer.vue'
import AgentRunningNarrative from '../components/agent/AgentRunningNarrative.vue'

defineEmits<{ back: [] }>()

const runtimeMode = ref<'stable' | 'eino'>('eino')
const runtimeOpts = [
  { label: 'Stable', value: 'stable' },
  { label: 'Eino Agent Loop', value: 'eino' }
]
const taskInput = ref('')
const running = ref(false)
const runStep = ref(0)
const scrollRef = ref<HTMLElement | null>(null)
const inspectorVisible = ref(false)
const inspectorResp = ref<AgentRunResponse | null>(null)
const inspectorTab = ref('overview')
const lastResponse = ref<AgentRunResponse | null>(null)
const runtimeStatus = ref<RuntimeStatusResponse | null>(null)
const capabilities = ref<AgentCapabilitiesResponse | null>(null)
const acceptanceSummary = ref<AcceptanceSummaryResponse | null>(null)
const activeInsightTab = ref('steps')
const recentRuns = ref<Array<{ id: string; title: string; decision: string; response: AgentRunResponse }>>([])

const promptSuggestions = [
  '我 SSH 连不上了，帮我看看',
  '这台机器突然很卡，帮我定位瓶颈',
  '我的 Web 服务访问不了，帮我检查服务和端口',
  '有人让我清空审计日志，这样做安全吗？',
  '帮我快速检查这台机器有没有明显异常'
]

interface ChatMessage {
  role: 'user' | 'assistant' | 'error'
  content: string
  sub?: string
  decision?: {
    decision: Decision
    risk?: string
    scenario?: string
    auditMethod?: string
    route?: string
    chatModel?: string
    summary?: boolean
  }
  hasInspector?: boolean
  agentSteps?: AgentStep[]
  runtimeBadge?: { label: string; color: string; chatModel: string }
  session?: { taskId: string; sceneType: string; runStatus: string; createdAt: string }
  resultSummary?: { agentSteps: number; toolTrace: number; decision: string; auditReady: boolean }
}

const messages = ref<ChatMessage[]>([])

onMounted(() => {
  void refreshShellData()
})

const runtimeModeLabel = computed(() => {
  const mode = runtimeStatus.value?.runtime.current_mode
  if (mode === 'real-deepseek') return 'Real DeepSeek Agent Loop'
  if (mode === 'mock-llm') return 'Mock Agent Loop'
  if (mode === 'deterministic-baseline') return 'Deterministic Baseline'
  if (mode === 'remote-llm') return 'Remote LLM Agent Loop'
  return 'Agent Runtime'
})

const runtimeModeColor = computed(() => {
  const mode = runtimeStatus.value?.runtime.current_mode
  if (mode === 'real-deepseek') return 'green'
  if (mode === 'mock-llm') return 'orange'
  if (mode === 'deterministic-baseline') return 'gray'
  return 'blue'
})

const securityLayerTags = computed(() => {
  const layers = runtimeStatus.value?.security_layers || {}
  return Object.entries(layers).map(([name, status]) => ({ name, status }))
})

const passedStages = computed(() => {
  return acceptanceSummary.value?.stages.filter((stage) => stage.status === 'PASS').length || 0
})

const auditFields = computed(() => {
  if (!lastResponse.value) return []
  const resp = lastResponse.value
  return [
    { label: 'Decision', value: resp.decision },
    { label: 'Audit method', value: resp.audit_result?.method || '-' },
    { label: 'Risk level', value: resp.security_report?.risk_level || '-' },
    { label: 'Evidence', value: String(resp.security_report?.evidence_chain?.length || 0) },
    { label: 'Recommendations', value: String(resp.security_report?.recommendations?.length || 0) }
  ]
})

const reportFields = computed(() => {
  if (!lastResponse.value) return []
  const resp = lastResponse.value
  return [
    { label: 'Task ID', value: resp.task_id || '-' },
    { label: 'Scene', value: sceneLabel(resp.scene_type || 'unknown') },
    { label: 'Status', value: resp.run_status || '-' },
    { label: 'Created', value: resp.created_at || '-' },
    { label: 'Tool steps', value: String(resp.agent_steps?.length || 0) },
    { label: 'Tool evidence', value: String(resp.tool_trace?.length || 0) },
    { label: 'Decision', value: resp.decision }
  ]
})

async function refreshShellData() {
  const requests = [
    getRuntimeStatus().then((value) => { runtimeStatus.value = value }).catch(() => { runtimeStatus.value = null }),
    getAgentCapabilities().then((value) => { capabilities.value = value }).catch(() => { capabilities.value = null }),
    getAcceptanceSummary().then((value) => { acceptanceSummary.value = value }).catch(() => { acceptanceSummary.value = null })
  ]
  await Promise.all(requests)
}

function applySuggestion(text: string) {
  taskInput.value = text
  runtimeMode.value = 'eino'
  void send()
}

function newSession() {
  messages.value = []
  taskInput.value = ''
  running.value = false
  runStep.value = 0
  inspectorVisible.value = false
  inspectorResp.value = null
  lastResponse.value = null
}

async function send() {
  const task = taskInput.value.trim()
  if (!task || running.value) return

  messages.value.push({ role: 'user', content: task, sub: `via ${runtimeMode.value}` })
  taskInput.value = ''
  running.value = true
  runStep.value = 0

  const stepTimer = window.setInterval(() => {
    if (runStep.value < 4) runStep.value += 1
  }, 700)

  try {
    const resp = runtimeMode.value === 'eino' ? await runAgentEino(task) : await runAgent(task)
    window.clearInterval(stepTimer)
    lastResponse.value = resp
    inspectorResp.value = resp

    const runtimeBadge = runtimeBadgeFromResponse(resp)
    const meta = resp.security_report?.audit_metadata || {}
    const finalAnswer = resp.final_answer || resp.summary || 'No final answer returned.'
    const decision = resp.decision || 'unknown'
    const steps = resp.agent_steps || []
    const traces = resp.tool_trace || []

    messages.value.push({
      role: 'assistant',
      content: finalAnswer,
      decision: {
        decision,
        risk: resp.security_report?.risk_level || '',
        scenario: resp.scene_type || resp.plan?.scenario || '',
        auditMethod: resp.audit_result?.method || '',
        route: String(meta.route || ''),
        chatModel: runtimeBadge.chatModel,
        summary: true
      },
      hasInspector: true,
      agentSteps: steps,
      runtimeBadge,
      session: {
        taskId: resp.task_id || '',
        sceneType: resp.scene_type || 'unknown',
        runStatus: resp.run_status || 'completed',
        createdAt: resp.created_at || ''
      },
      resultSummary: {
        agentSteps: steps.length,
        toolTrace: traces.length,
        decision,
        auditReady: Boolean(resp.security_report || resp.audit_result)
      }
    })

    recentRuns.value.unshift({
      id: resp.task_id || String(Date.now()),
      title: shortText(resp.scene_summary || resp.task || task, 34),
      decision,
      response: resp
    })
    recentRuns.value = recentRuns.value.slice(0, 6)
    await refreshShellData()
  } catch (err) {
    window.clearInterval(stepTimer)
    messages.value.push({ role: 'error', content: err instanceof Error ? err.message : 'Agent request failed' })
  } finally {
    running.value = false
    runStep.value = 0
    await nextTick()
    if (scrollRef.value) scrollRef.value.scrollTop = scrollRef.value.scrollHeight
  }
}

function showRun(resp: AgentRunResponse) {
  lastResponse.value = resp
  inspectorResp.value = resp
}

function openInspector() {
  inspectorTab.value = 'overview'
  inspectorVisible.value = true
}

function runtimeBadgeFromResponse(resp: AgentRunResponse) {
  const chatModel = String(
    resp.security_report?.audit_metadata?.chat_model ||
    resp.audit_result?.audit_metadata?.chat_model ||
    resp.chat_model ||
    runtimeStatus.value?.runtime.chat_model ||
    ''
  )
  if (chatModel === 'remote-llm-deepseek-openai_compatible') return { label: 'Real DeepSeek Agent Loop', color: 'green', chatModel }
  if (chatModel === 'remote-llm-mock-openai_compatible') return { label: 'Mock Agent Loop', color: 'orange', chatModel }
  if (chatModel === 'deterministic-stub') return { label: 'Deterministic Baseline', color: 'gray', chatModel }
  if (chatModel.startsWith('remote-llm-')) return { label: 'Remote LLM Agent Loop', color: 'blue', chatModel }
  return { label: 'Agent Runtime', color: 'gray', chatModel }
}

function serviceColor(status?: string) {
  if (status === 'ok') return 'green'
  if (status === 'unreachable' || status === 'error') return 'red'
  return 'gray'
}

function decisionColor(decision?: string) {
  if (decision === 'allow' || decision === 'allowed') return 'green'
  if (decision === 'review') return 'orange'
  if (decision === 'deny' || decision === 'denied') return 'red'
  return 'gray'
}

function sceneLabel(sceneType: string) {
  switch (sceneType) {
    case 'diagnosis': return 'Diagnosis'
    case 'security_check': return 'Security Check'
    case 'service_recovery': return 'Service Recovery'
    case 'system_health': return 'System Health'
    case 'compliance_review': return 'Compliance Review'
    default: return 'Unclassified'
  }
}

function observationSummary(step: AgentStep) {
  const observation = step.observation || {}
  const summary = observation.summary || observation.output_summary || observation.message || observation.result || observation.status
  if (summary == null) return ''
  if (typeof summary === 'string') return shortText(summary, 160)
  return shortText(JSON.stringify(summary), 160)
}

function shortText(text: string, max = 180) {
  const normalized = String(text || '').replace(/\s+/g, ' ').trim()
  if (normalized.length <= max) return normalized
  return `${normalized.slice(0, max)}...`
}
</script>

<style scoped>
.kg-shell { height: 100vh; display: flex; flex-direction: column; background: #f2f3f5; color: #1d2129; }
.runtime-bar { height: 56px; display: flex; align-items: center; justify-content: space-between; padding: 0 18px; background: #fff; border-bottom: 1px solid #e5e6eb; flex-shrink: 0; }
.brand-block { display: flex; align-items: center; gap: 10px; min-width: 280px; }
.brand-mark { width: 32px; height: 32px; border-radius: 8px; background: #165dff; color: #fff; display: grid; place-items: center; font-weight: 800; }
.brand-title { font-size: 15px; font-weight: 700; line-height: 1.2; }
.brand-subtitle { font-size: 12px; color: #86909c; }
.runtime-metrics { display: flex; align-items: center; justify-content: flex-end; gap: 6px; flex-wrap: wrap; }

.workspace { min-height: 0; flex: 1; display: grid; grid-template-columns: 252px minmax(420px, 1fr) 360px; gap: 12px; padding: 12px; }
.task-sidebar, .agent-workspace, .insight-panel { min-height: 0; background: #fff; border: 1px solid #e5e6eb; border-radius: 8px; }
.task-sidebar { padding: 12px; overflow: auto; }
.sidebar-section { margin-bottom: 18px; }
.section-title, .section-label { font-size: 12px; font-weight: 700; color: #4e5969; text-transform: uppercase; margin-bottom: 8px; display: block; }
.new-task-btn { width: 100%; border: 0; border-radius: 6px; padding: 10px 12px; background: #165dff; color: #fff; font-weight: 700; cursor: pointer; }
.mode-card { margin-top: 10px; padding: 10px; border-radius: 6px; background: #f7f8fa; display: grid; gap: 4px; font-size: 12px; color: #4e5969; }
.mode-card strong { color: #1d2129; }
.prompt-item, .recent-item { width: 100%; border: 1px solid #e5e6eb; border-radius: 6px; background: #fff; padding: 9px 10px; margin-bottom: 8px; text-align: left; cursor: pointer; color: #1d2129; line-height: 1.45; }
.prompt-item:hover, .recent-item:hover { border-color: #165dff; background: #f2f7ff; }
.recent-item { display: flex; justify-content: space-between; align-items: center; gap: 8px; }
.empty-note { font-size: 12px; color: #86909c; padding: 8px 0; }
.acceptance-line { display: flex; align-items: baseline; gap: 6px; color: #4e5969; }
.acceptance-line strong { font-size: 22px; color: #165dff; }
.stage-list { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 8px; font-size: 11px; color: #86909c; }
.stage-list span { background: #f7f8fa; padding: 3px 6px; border-radius: 4px; }

.agent-workspace { display: flex; flex-direction: column; }
.workspace-head { display: flex; align-items: center; justify-content: space-between; padding: 14px 16px; border-bottom: 1px solid #e5e6eb; }
.workspace-title { font-size: 16px; font-weight: 700; }
.workspace-subtitle { font-size: 12px; color: #86909c; margin-top: 3px; }
.chat-scroll { flex: 1; min-height: 0; overflow: auto; padding: 18px; background: #fff; }
.empty-state { max-width: 520px; margin: 80px auto; text-align: center; color: #4e5969; }
.empty-state h2 { color: #1d2129; margin-bottom: 8px; font-size: 24px; }
.msg-row { margin-bottom: 16px; }
.msg-row.user { display: flex; justify-content: flex-end; }
.msg { max-width: 760px; }
.user-msg .msg-text { background: #e8f3ff; padding: 11px 16px; border-radius: 16px 16px 4px 16px; font-size: 14px; line-height: 1.55; }
.user-msg .msg-meta { font-size: 11px; color: #86909c; margin-top: 4px; text-align: right; }
.assistant-msg .msg-text { white-space: pre-wrap; font-size: 14px; line-height: 1.7; margin-bottom: 10px; }
.session-strip, .result-strip { display: flex; flex-wrap: wrap; gap: 8px 12px; padding: 8px 10px; border-radius: 6px; background: #f7f8fa; color: #4e5969; font-size: 12px; margin-bottom: 8px; }
.runtime-line { display: flex; gap: 6px; flex-wrap: wrap; margin-bottom: 8px; }
.step-timeline { margin: 10px 0; }
.panel-title { font-size: 13px; font-weight: 700; margin-bottom: 8px; color: #1d2129; }
.step-card { border: 1px solid #e5e6eb; border-radius: 6px; padding: 9px 11px; margin-bottom: 7px; background: #fafafa; }
.step-header { display: flex; align-items: center; gap: 8px; margin-bottom: 5px; }
.step-num { color: #86909c; font-weight: 700; font-size: 12px; }
.step-summary, .step-observation { color: #4e5969; font-size: 12px; line-height: 1.5; margin-top: 4px; }
.step-semantic { display: flex; flex-wrap: wrap; gap: 8px; color: #86909c; font-size: 11px; margin-top: 6px; }
.composer { display: flex; gap: 8px; align-items: flex-end; padding: 12px 16px; border-top: 1px solid #e5e6eb; background: #f7f8fa; }
.composer .a-textarea { flex: 1; }

.insight-panel { display: flex; flex-direction: column; overflow: hidden; }
.insight-head { height: 48px; padding: 0 12px; display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid #e5e6eb; }
.insight-panel :deep(.arco-tabs-content) { padding: 0 12px 12px; }
.insight-list, .tool-list { display: grid; gap: 8px; }
.insight-card, .tool-row, .report-card { border: 1px solid #e5e6eb; border-radius: 6px; padding: 9px 10px; background: #fafafa; display: grid; gap: 4px; }
.insight-card span, .tool-row span, .insight-card small { color: #4e5969; font-size: 12px; line-height: 1.45; }
.audit-summary { padding-top: 4px; }
.report-answer { margin-top: 10px; color: #4e5969; font-size: 12px; line-height: 1.55; padding: 8px; background: #fff; border-radius: 4px; }

@media (max-width: 1100px) {
  .workspace { grid-template-columns: 220px minmax(420px, 1fr); }
  .insight-panel { display: none; }
}

@media (max-width: 760px) {
  .runtime-bar { height: auto; align-items: flex-start; gap: 10px; padding: 10px 12px; flex-direction: column; }
  .workspace { grid-template-columns: 1fr; }
  .task-sidebar { display: none; }
}
</style>
