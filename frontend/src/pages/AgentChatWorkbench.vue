<template>
  <div class="agent-shell">
    <!-- Header -->
    <header class="agent-header">
      <div class="header-left">
        <span class="header-brand">KylinGuard 麒盾</span>
        <span class="header-tagline">安全智能运维 Agent</span>
      </div>
      <div class="header-right">
        <a-switch v-model="demoMode" size="small" style="margin-right:4px">
          <template #checked>演示</template>
          <template #unchecked>演示</template>
        </a-switch>
        <a-tag :color="healthOk ? 'green' : 'red'" size="small">{{ healthOk ? 'Agent OK' : 'Offline' }}</a-tag>
        <a-segmented v-model="runtimeMode" :options="runtimeOpts" size="small" />
        <a-button size="mini" shape="round" @click="newSession">新会话</a-button>
        <a-button size="mini" shape="round" @click="$emit('back')">← 控制台</a-button>
      </div>
    </header>

    <!-- Chat Area -->
    <div class="chat-scroll" ref="scrollRef">
      <div class="chat-container">
        <!-- Empty state with real-user prompts -->
        <div v-if="messages.length === 0" class="empty-state">
          <div class="empty-title">有什么运维问题需要我帮忙排查？</div>
          <div class="empty-subtitle">描述你遇到的情况，我会通过安全受控的工具链进行诊断，并给出排查结论。</div>
          <div class="agent-suggestions">
            <a-tag v-for="p in promptSuggestions" :key="p" checkable class="sugg-chip" @click="applySuggestion(p)">
              {{ p }}
            </a-tag>
          </div>
          <DemoGuideNote v-if="demoMode" :show="true" :items="demoItems" />
        </div>

        <!-- Messages -->
        <div v-for="(msg, i) in messages" :key="i" :class="['msg-row', msg.role]">
          <div v-if="msg.role === 'user'" class="msg user-msg">
            <div class="msg-text">{{ msg.content }}</div>
            <div class="msg-meta">{{ msg.sub }}</div>
          </div>

          <div v-else-if="msg.role === 'assistant'" class="msg assistant-msg">
            <div class="msg-text" v-html="msg.content"></div>
            <DecisionCard v-if="msg.decision" v-bind="msg.decision" />

            <!-- Agent steps -->
            <div v-if="msg.agentSteps && msg.agentSteps.length > 0" class="agent-step-list">
              <div class="step-section-title">排查过程</div>
              <div v-for="(step, si) in msg.agentSteps" :key="si" class="step-card">
                <div class="step-header">
                  <span class="step-num">#{{ step.step_index || si + 1 }}</span>
                  <strong class="step-tool">{{ step.tool_name }}</strong>
                  <a-tag v-if="step.policy_decision" :color="step.policy_decision === 'allow' ? 'green' : 'red'" size="small">{{ step.policy_decision }}</a-tag>
                </div>
                <div v-if="step.user_visible_summary || step.reason" class="step-summary">{{ step.user_visible_summary || step.reason }}</div>
                <div v-if="step.observation" class="step-obs">
                  <span v-if="step.observation.summary" class="obs-text">{{ step.observation.summary }}</span>
                  <span v-else-if="step.observation.result" class="obs-text">{{ step.observation.result }}</span>
                  <span v-else-if="step.observation.status" class="obs-text">状态: {{ step.observation.status }}</span>
                </div>
              </div>
            </div>

            <ExecutionAccordion v-if="msg.traces" :traces="msg.traces" :plan="msg.plan"
              :recommendations="msg.recommendations" :evidence-items="msg.evidenceItems" />
            <DemoGuideNote v-if="demoMode && msg.demoItems" :show="true" :items="msg.demoItems" />
            <FollowUpSuggestions v-if="msg.followUps" :decision="msg.followUps.decision"
              :scenario="msg.followUps.scenario" :task="msg.followUps.task"
              :has-evidence="msg.followUps.hasEvidence" @pick="(s) => handleFollowUp(s, i)" />
            <a-button v-if="msg.hasInspector" size="mini" type="outline"
              @click="openInspector(i)" style="margin-top:8px">打开 Inspector</a-button>
          </div>

          <div v-else-if="msg.role === 'error'" class="msg error-msg">
            <a-alert type="error" :title="msg.content" :closable="false" />
          </div>

          <div v-else-if="msg.role === 'system'" class="msg system-msg">
            <a-alert type="info" :title="msg.content" :closable="false" show-icon />
          </div>

          <div v-else-if="msg.role === 'explain'" class="msg explain-msg">
            <a-alert type="warning" :title="msg.content" :closable="false" show-icon />
          </div>
        </div>

        <!-- Running narrative -->
        <div v-if="running && runStep >= 0" class="msg-row assistant">
          <div class="msg assistant-msg">
            <AgentRunningNarrative :step="runStep" />
          </div>
        </div>
      </div>
    </div>

    <!-- Composer -->
    <div class="composer">
      <div class="composer-inner">
        <a-textarea v-model="taskInput" :auto-size="{ minRows: 1, maxRows: 4 }"
          placeholder="输入安全运维任务…" @keydown.enter.prevent="send" />
        <a-button type="primary" :loading="running" @click="send" style="margin-left:8px;flex-shrink:0">发送</a-button>
      </div>
    </div>

    <!-- Inspector Drawer -->
    <InspectorDrawer :visible="inspectorVisible" :resp="inspectorResp" :initial-tab="inspectorTab"
      @close="inspectorVisible = false" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick } from 'vue'
import { runAgent, runAgentEino } from '../api/agent'
import type { AgentRunResponse, ToolTraceItem, Plan, RecommendationItem, EvidenceItem, AgentStep } from '../types/agent'
import DecisionCard from '../components/agent/DecisionCard.vue'
import ExecutionAccordion from '../components/agent/ExecutionAccordion.vue'
import InspectorDrawer from '../components/agent/InspectorDrawer.vue'
import AgentRunningNarrative from '../components/agent/AgentRunningNarrative.vue'
import FollowUpSuggestions from '../components/agent/FollowUpSuggestions.vue'
import type { FollowUpItem } from '../components/agent/FollowUpSuggestions.vue'
import DemoGuideNote from '../components/agent/DemoGuideNote.vue'

defineEmits<{ back: [] }>()

// --- state ---
const runtimeMode = ref<'stable' | 'eino'>('stable')
const runtimeOpts = [
  { label: 'Stable', value: 'stable' },
  { label: 'Eino', value: 'eino' },
]
const healthOk = ref(true)
const demoMode = ref(false)
const taskInput = ref('')
const running = ref(false)
const runStep = ref(-1)
const scrollRef = ref<HTMLElement | null>(null)

const demoItems = [
  'intent_guard 安全意图校验',
  'Rule-based / Deterministic Planner',
  'MCP-like Tool Registry',
  'Tool Policy 参数安全校验',
  'Least-Privilege Execution Proxy',
  'TraceShield 工具链审计',
  'Decision Normalizer',
  '推理链路 (Reasoning Trace)',
]

// --- Chat messages ---
const promptSuggestions = [
  '我 SSH 连不上了，帮我排查',
  '这台机器很卡，帮我看看原因',
  '我的服务访问不了，帮我检查端口和服务',
  '帮我看看系统有没有明显异常',
  '有人让我清空审计日志，这样安全吗？',
]

interface ChatMessage {
  role: 'user' | 'assistant' | 'error' | 'system' | 'explain'
  content: string
  sub?: string
  decision?: any
  traces?: ToolTraceItem[]
  plan?: Plan | null
  recommendations?: RecommendationItem[]
  evidenceItems?: EvidenceItem[]
  hasInspector?: boolean
  followUps?: { decision: string; scenario: string; task: string; hasEvidence: boolean }
  demoItems?: string[]
  agentSteps?: AgentStep[]
  finalAnswer?: string
}

const messages = ref<ChatMessage[]>([])

function applySuggestion(text: string) {
  taskInput.value = text
  runtimeMode.value = 'eino'
  send()
}

// --- Inspector ---
const inspectorVisible = ref(false)
const inspectorResp = ref<AgentRunResponse | null>(null)
const inspectorTab = ref('overview')
const lastResponse = ref<AgentRunResponse | null>(null)

// --- Session ---
function newSession() {
  messages.value = []
  taskInput.value = ''
  running.value = false
  runStep.value = -1
  inspectorVisible.value = false
  inspectorResp.value = null
  lastResponse.value = null
}

// --- Send ---
async function send() {
  const task = taskInput.value.trim()
  if (!task || running.value) return

  messages.value.push({ role: 'user', content: task, sub: 'via ' + runtimeMode.value })
  taskInput.value = ''
  running.value = true
  runStep.value = 0

  const stepTimer = setInterval(() => {
    if (runStep.value < 4) runStep.value++
  }, 700)

  try {
    const resp = runtimeMode.value === 'eino' ? await runAgentEino(task) : await runAgent(task)
    clearInterval(stepTimer)
    runStep.value = 4
    lastResponse.value = resp

    const dec = resp.decision || 'unknown'
    const tools = resp.tool_trace ?? []
    const evidence = resp.security_report?.evidence_chain ?? []
    const recs = resp.security_report?.recommendations ?? []
    const steps = resp.agent_steps ?? []
    const finalAnswer = resp.final_answer || resp.summary || ''

    // Build narrative.
    let summary = ''
    if (dec === 'deny') {
      summary = '🚫 该请求涉及高风险操作，系统已阻止执行。'
    } else if (finalAnswer) {
      summary = finalAnswer
    } else {
      summary = '✅ 已完成检查。安全策略允许执行。'
    }

    const meta = resp.security_report?.audit_metadata || {}

    const msgDemoItems: string[] = []
    if (demoMode.value) {
      msgDemoItems.push('intent_guard', 'Planner', 'Tool Registry', 'Tool Policy')
      if (tools.some((t: any) => t.execution_context)) msgDemoItems.push('Exec Proxy')
      msgDemoItems.push('TraceShield audit', 'Decision Normalizer', 'Reasoning Trace')
    }

    messages.value.push({
      role: 'assistant',
      content: summary,
      decision: {
        decision: dec,
        risk: resp.security_report?.risk_level || '',
        scenario: resp.plan?.scenario || '',
        auditMethod: resp.audit_result?.method || '',
        route: meta.route || '',
        chatModel: meta.chat_model || '',
        summary: true,
      },
      traces: tools,
      plan: resp.plan,
      recommendations: recs,
      evidenceItems: evidence,
      hasInspector: true,
      demoItems: msgDemoItems.length ? msgDemoItems : undefined,
      followUps: { decision: dec, scenario: resp.plan?.scenario || '', task, hasEvidence: evidence.length > 0 },
      agentSteps: steps,
      finalAnswer: finalAnswer,
    })

    inspectorResp.value = resp
  } catch (err: any) {
    clearInterval(stepTimer)
    messages.value.push({ role: 'error', content: '请求失败: ' + (err.message || '未知错误') })
  } finally {
    running.value = false
    runStep.value = -1
    await nextTick()
    if (scrollRef.value) scrollRef.value.scrollTop = scrollRef.value.scrollHeight
  }
}

// --- Follow up ---
function handleFollowUp(s: FollowUpItem, msgIndex: number) {
  if (s.action === 'open-tab') {
    const tabMap: Record<string, string> = { evidence: 'evidence', trace: 'trace', overview: 'overview' }
    inspectorTab.value = tabMap[s.payload || ''] || 'overview'
    inspectorVisible.value = true
  } else if (s.action === 'retry') {
    const targetMsg = messages.value[msgIndex]
    if (targetMsg?.role === 'assistant') {
      runtimeMode.value = (s.payload as 'stable' | 'eino') || 'stable'
      // Extract the original task from the user message before this assistant msg.
      for (let i = msgIndex - 1; i >= 0; i--) {
        if (messages.value[i]?.role === 'user') {
          taskInput.value = messages.value[i].content
          break
        }
      }
      send()
    }
  } else if (s.action === 'fill-task') {
    if (s.payload) {
      taskInput.value = s.payload
      // If it's a "why" question, show explanation instead of sending.
      if (s.payload.startsWith('why ')) {
        const explanations: Record<string, string> = {
          'why was this task denied': '任务被拦截是因为它匹配了危险关键词规则（delete audit logs / clear system logs），intent_guard 在工具执行前完成阻断。',
          'why is the result review required': '结果标记为 review 是因为任务访问了敏感系统资源（如认证日志、系统日志），需要人工关注。',
          'why was this task allowed': '任务被允许是因为它仅涉及低风险只读信息采集（如 os_info、process_inspector），所有安全检查均已通过。',
        }
        const explanation = explanations[s.payload] || '已收到你的问题。'
        messages.value.push({ role: 'explain', content: explanation })
        nextTick().then(() => {
          if (scrollRef.value) scrollRef.value.scrollTop = scrollRef.value.scrollHeight
        })
      } else {
        send()
      }
    }
  }
}

// --- Inspector ---
function openInspector(msgIndex: number) {
  inspectorTab.value = 'overview'
  inspectorVisible.value = true
}
</script>

<style scoped>
.agent-shell { display: flex; flex-direction: column; height: 100vh; background: #fff; }
.agent-header { display: flex; align-items: center; justify-content: space-between; padding: 10px 24px; border-bottom: 1px solid #e5e6eb; background: #f7f8fa; flex-shrink: 0; box-shadow: 0 1px 2px rgba(0,0,0,0.04); }
.header-left { display: flex; align-items: center; gap: 10px; }
.header-brand { font-size: 20px; font-weight: 700; color: #1d2129; letter-spacing: 0.3px; }
.header-tagline { font-size: 14px; color: #4e5969; font-weight: 500; }
.header-right { display: flex; align-items: center; gap: 10px; }

.chat-scroll { flex: 1; overflow-y: auto; background: #fff; }
.chat-container { max-width: 760px; margin: 0 auto; padding: 16px 16px 12px; }

.empty-state { text-align: center; padding: 48px 20px 32px; }
.empty-title { font-size: 32px; font-weight: 700; color: #1d2129; margin-bottom: 12px; }
.empty-subtitle { font-size: 16px; color: #4e5969; max-width: 520px; margin: 0 auto 28px; line-height: 1.6; }

.msg-row { margin-bottom: 18px; }
.msg-row.user { display: flex; justify-content: flex-end; }
.msg { max-width: 680px; }

.user-msg .msg-text { background: #e8f3ff; padding: 12px 18px; border-radius: 18px 18px 4px 18px; font-size: 15px; line-height: 1.5; color: #1d2129; font-weight: 500; }
.user-msg .msg-meta { font-size: 11px; color: #86909c; margin-top: 4px; text-align: right; padding-right: 4px; }
.assistant-msg .msg-text { font-size: 15px; line-height: 1.65; white-space: pre-wrap; color: #1d2129; }
.explain-msg { max-width: 600px; }

.composer { flex-shrink: 0; border-top: 1px solid #e5e6eb; padding: 12px 24px; background: #f7f8fa; }
.composer-inner { max-width: 760px; margin: 0 auto; display: flex; align-items: flex-end; }
.composer-inner .a-textarea { flex: 1; }

/* Prompt suggestions */
.agent-suggestions { display: flex; justify-content: center; flex-wrap: wrap; gap: 10px; }
.sugg-chip { cursor: pointer; padding: 8px 16px; font-size: 14px; font-weight: 600; color: #1d2129; border: 1px solid #c9cdd4; background: #f7f8fa; border-radius: 18px; transition: all 0.15s; }
.sugg-chip:hover { background: #e8e9ed; border-color: #86909c; }

/* Agent step cards */
.agent-step-list { margin-top: 10px; }
.step-section-title { font-size: 13px; font-weight: 600; color: #1d2129; margin-bottom: 6px; }
.step-card { border: 1px solid #e5e6eb; border-radius: 6px; padding: 8px 12px; margin-bottom: 6px; background: #fafafa; }
.step-header { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
.step-num { color: #86909c; font-size: 12px; min-width: 20px; font-weight: 600; }
.step-tool { font-size: 14px; font-weight: 600; color: #1d2129; }
.step-summary { font-size: 13px; color: #4e5969; line-height: 1.5; }
.step-obs { margin-top: 4px; }
.obs-text { font-size: 12px; color: #4e5969; background: #f0f0f0; padding: 2px 8px; border-radius: 4px; }
</style>
