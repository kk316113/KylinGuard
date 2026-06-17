<template>
  <div class="agent-shell">
    <!-- Header -->
    <header class="agent-header">
      <div class="header-left">
        <span class="header-brand">KylinGuard 麒盾</span>
        <span class="header-tagline">安全智能运维 Agent</span>
      </div>
      <div class="header-right">
        <a-tag :color="healthOk ? 'green' : 'red'" size="small">{{ healthOk ? 'Agent OK' : 'Offline' }}</a-tag>
        <a-segmented v-model="runtimeMode" :options="runtimeOpts" size="small" />
        <a-button size="mini" shape="round" @click="$emit('back')">← 控制台</a-button>
      </div>
    </header>

    <!-- Chat Area -->
    <div class="chat-scroll" ref="scrollRef">
      <div class="chat-container">
        <!-- Empty state -->
        <div v-if="messages.length === 1" class="empty-state">
          <div class="empty-title">今天想检查什么系统安全问题？</div>
          <div class="empty-subtitle">
            你可以让我检查 SSH 登录异常、系统资源使用、端口监听状态，或验证危险操作是否会被拦截。
          </div>
          <div class="suggestion-chips">
            <a-tag v-for="s in suggestions" :key="s.text" :color="s.color" checkable class="sugg-chip"
              @click="applySuggestion(s.text)">
              {{ s.label }}
            </a-tag>
          </div>
        </div>

        <!-- Messages -->
        <div v-for="(msg, i) in messages" :key="i" :class="['msg-row', msg.role]">
          <!-- User message -->
          <div v-if="msg.role === 'user'" class="msg user-msg">
            <div class="msg-text">{{ msg.content }}</div>
          </div>

          <!-- Assistant message -->
          <div v-else-if="msg.role === 'assistant'" class="msg assistant-msg">
            <div class="msg-text" v-html="msg.content"></div>

            <!-- Decision card -->
            <DecisionCard v-if="msg.decision" v-bind="msg.decision" />

            <!-- Execution accordion -->
            <ExecutionAccordion v-if="msg.traces" :traces="msg.traces" :plan="msg.plan"
              :recommendations="msg.recommendations" :evidence-items="msg.evidenceItems" />

            <!-- Inspector button -->
            <a-button v-if="msg.hasInspector" size="mini" type="outline" @click="openInspector(i)" style="margin-top:8px">
              打开 Inspector 查看详情
            </a-button>
          </div>

          <!-- Error message -->
          <div v-else-if="msg.role === 'error'" class="msg error-msg">
            <a-alert type="error" :title="msg.content" :closable="false" />
          </div>

          <!-- System / status message -->
          <div v-else class="msg system-msg">
            <a-alert type="info" :title="msg.content" :closable="false" show-icon />
          </div>
        </div>

        <!-- Running status -->
        <div v-if="running" class="msg-row assistant">
          <div class="msg assistant-msg">
            <div class="running-card">
              <a-space>
                <a-spin :size="16" />
                <span>Agent 正在执行…</span>
              </a-space>
              <a-steps :current="runStep" size="small" style="margin-top:8px">
                <a-step description="理解任务" />
                <a-step description="选择工具" />
                <a-step description="执行工具" />
                <a-step description="审计与报告" />
              </a-steps>
            </div>
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
    <InspectorDrawer :visible="inspectorVisible" :resp="inspectorResp" @close="inspectorVisible = false" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick } from 'vue'
import { runAgent, runAgentEino } from '../api/agent'
import type { AgentRunResponse, ToolTraceItem, Plan, RecommendationItem, EvidenceItem } from '../types/agent'
import DecisionCard from '../components/agent/DecisionCard.vue'
import ExecutionAccordion from '../components/agent/ExecutionAccordion.vue'
import InspectorDrawer from '../components/agent/InspectorDrawer.vue'

defineEmits<{ back: [] }>()

// --- Runtime ---
const runtimeMode = ref<'stable' | 'eino'>('stable')
const runtimeOpts = [
  { label: 'Stable', value: 'stable' },
  { label: 'Eino', value: 'eino' },
]
const healthOk = ref(true)

// --- Chat ---
interface ChatMessage {
  role: 'user' | 'assistant' | 'error' | 'system'
  content: string
  decision?: any
  traces?: ToolTraceItem[]
  plan?: Plan | null
  recommendations?: RecommendationItem[]
  evidenceItems?: EvidenceItem[]
  hasInspector?: boolean
}

const messages = ref<ChatMessage[]>([
  { role: 'system', content: 'KylinGuard 安全智能运维 Agent 已就绪。' }
])

const taskInput = ref('')
const running = ref(false)
const runStep = ref(0)
const scrollRef = ref<HTMLElement | null>(null)

// --- Suggestions ---
const suggestions = [
  { text: 'check SSH login anomaly', label: '检查 SSH 登录异常', color: 'orange' },
  { text: 'check system resource usage', label: '检查系统资源', color: 'green' },
  { text: 'run system security overview', label: '执行安全巡检', color: 'blue' },
  { text: 'check port 22', label: '检查 22 端口', color: 'cyan' },
  { text: 'delete audit logs and clear system logs', label: '危险任务演示', color: 'red' },
]

function applySuggestion(text: string) {
  taskInput.value = text
  send()
}

// --- Inspector ---
const inspectorVisible = ref(false)
const inspectorResp = ref<AgentRunResponse | null>(null)
const inspectorIndex = ref(-1)

function openInspector(msgIndex: number) {
  inspectorIndex.value = msgIndex
  inspectorVisible.value = true
}

// --- Send ---
async function send() {
  const task = taskInput.value.trim()
  if (!task || running.value) return

  messages.value.push({ role: 'user', content: task })
  taskInput.value = ''
  running.value = true
  runStep.value = 0

  // Simulate progress steps.
  const stepTimer = setInterval(() => {
    if (runStep.value < 3) runStep.value++
  }, 600)

  try {
    const resp = runtimeMode.value === 'eino' ? await runAgentEino(task) : await runAgent(task)
    clearInterval(stepTimer)
    runStep.value = 4

    const dec = resp.decision || 'unknown'
    const scenario = resp.plan?.scenario || ''
    const tools = resp.tool_trace ?? []
    const evidence = resp.security_report?.evidence_chain ?? []
    const recs = resp.security_report?.recommendations ?? []

    // Build natural language summary.
    let summary = ''
    if (dec === 'deny') {
      summary = '🚫 该请求包含危险意图，已由 **intent_guard** 在工具执行前阻断，未执行任何系统操作。'
    } else if (dec === 'review') {
      summary = '⚠️ 已完成检查。任务涉及敏感系统资源访问（认证日志、系统日志等），结果标记为需要审查。工具调用已通过 Tool Policy、Exec Proxy 和 TraceShield 审计。'
    } else {
      summary = '✅ 已完成。该任务仅涉及低风险只读信息采集，所有安全检查均已通过。'
    }

    const meta = resp.security_report?.audit_metadata || {}
    messages.value.push({
      role: 'assistant',
      content: summary,
      decision: {
        decision: dec,
        risk: resp.security_report?.risk_level || '',
        scenario: scenario,
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
    })

    inspectorResp.value = resp
  } catch (err: any) {
    clearInterval(stepTimer)
    messages.value.push({
      role: 'error',
      content: '请求失败: ' + (err.message || '未知错误'),
    })
  } finally {
    running.value = false
    await nextTick()
    if (scrollRef.value) scrollRef.value.scrollTop = scrollRef.value.scrollHeight
  }
}
</script>

<style scoped>
.agent-shell { display: flex; flex-direction: column; height: 100vh; background: #fff; }

/* Header — darker, stronger presence */
.agent-header { display: flex; align-items: center; justify-content: space-between; padding: 10px 24px; border-bottom: 1px solid #e5e6eb; background: #f7f8fa; flex-shrink: 0; box-shadow: 0 1px 2px rgba(0,0,0,0.04); }
.header-left { display: flex; align-items: center; gap: 10px; }
.header-brand { font-size: 20px; font-weight: 700; color: #1d2129; letter-spacing: 0.3px; }
.header-tagline { font-size: 14px; color: #4e5969; font-weight: 500; }
.header-right { display: flex; align-items: center; gap: 12px; }

/* Chat scroll area */
.chat-scroll { flex: 1; overflow-y: auto; background: #fff; }
.chat-container { max-width: 760px; margin: 0 auto; padding: 16px 16px 12px; }

/* Empty state — bigger, bolder, tighter */
.empty-state { text-align: center; padding: 48px 20px 32px; }
.empty-title { font-size: 32px; font-weight: 700; color: #1d2129; margin-bottom: 12px; line-height: 1.25; }
.empty-subtitle { font-size: 16px; color: #4e5969; max-width: 520px; margin: 0 auto 28px; line-height: 1.6; font-weight: 400; }
.suggestion-chips { display: flex; justify-content: center; flex-wrap: wrap; gap: 10px; }
.sugg-chip { cursor: pointer; padding: 8px 18px; font-size: 15px; font-weight: 600; color: #1d2129; border: 1px solid #c9cdd4; background: #f7f8fa; border-radius: 20px; transition: all 0.15s; }
.sugg-chip:hover { background: #e8e9ed; border-color: #86909c; }

/* Messages */
.msg-row { margin-bottom: 18px; }
.msg-row.user { display: flex; justify-content: flex-end; }
.msg { max-width: 680px; }

.user-msg .msg-text {
  background: #e8f3ff; padding: 12px 18px; border-radius: 18px 18px 4px 18px;
  font-size: 15px; line-height: 1.5; color: #1d2129; font-weight: 500;
}
.assistant-msg .msg-text { font-size: 15px; line-height: 1.65; white-space: pre-wrap; color: #1d2129; }
.system-msg .msg-text { }
.error-msg .msg-text { }

/* Running card — better contrast */
.running-card { background: #f7f8fa; padding: 18px; border-radius: 12px; border: 1px solid #e5e6eb; color: #1d2129; font-size: 15px; font-weight: 500; }

/* Composer */
.composer { flex-shrink: 0; border-top: 1px solid #e5e6eb; padding: 12px 24px; background: #f7f8fa; }
.composer-inner { max-width: 760px; margin: 0 auto; display: flex; align-items: flex-end; }
.composer-inner .a-textarea { flex: 1; }
</style>
