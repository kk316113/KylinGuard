<template>
  <!-- Nav bar -->
  <div class="nav-bar">
    <div class="nav-left">
      <span class="nav-brand">KylinGuard 麒盾</span>
      <span class="nav-version">v0.1</span>
    </div>
    <div class="nav-tabs">
      <button :class="['nav-tab', { active: view === 'console' }]" @click="view = 'console'">🛡️ 安全控制台</button>
      <button :class="['nav-tab', { active: view === 'workbench' }]" @click="view = 'workbench'">🤖 Agent 工作台</button>
    </div>
    <div class="nav-right"></div>
  </div>

  <!-- Console view -->
  <div v-if="view === 'console'" class="app-shell">
    <HealthBar :health="health" :health-error="healthError" :mode="mode" />

    <main class="dashboard">
      <section class="main-grid">
        <TaskRunner
          v-model:task="task"
          v-model:mode="mode"
          :loading="loading"
          @run="runTask"
        />

        <PlanTimeline :plan="latestResponse?.plan" />

        <DecisionSummary :response="latestResponse" />
      </section>

      <el-alert
        v-if="error"
        class="error-alert"
        type="error"
        :title="error"
        show-icon
        :closable="true"
        @close="error = ''"
      />

      <section v-if="!latestResponse && !error" class="welcome-panel">
        <div>
          <h2>麒麟安全智能运维 Agent 控制台</h2>
          <p>
            选择运行模式并提交安全运维任务。页面只展示 Go Agent 返回的 plan、diagnosis、
            tool_trace、TraceShield audit_result 和 security_report，不直接执行任何系统命令。
          </p>
        </div>
        <div class="welcome-checks">
          <el-tag effect="plain">Intent Guard</el-tag>
          <el-tag effect="plain">Rule-based Planner</el-tag>
          <el-tag effect="plain">SSH Diagnosis</el-tag>
          <el-tag effect="plain">TraceShield Audit</el-tag>
          <el-tag effect="plain">Evidence Chain</el-tag>
        </div>
      </section>

      <ReportTabs :response="latestResponse" />
    </main>
  </div>

  <!-- Workbench view -->
  <AgentChatWorkbench v-else @back="view = 'console'" />
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import HealthBar from './components/HealthBar.vue'
import TaskRunner from './components/TaskRunner.vue'
import PlanTimeline from './components/PlanTimeline.vue'
import DecisionSummary from './components/DecisionSummary.vue'
import ReportTabs from './components/ReportTabs.vue'
import AgentChatWorkbench from './pages/AgentChatWorkbench.vue'
import { getHealth, runAgent, runAgentEino } from './api/agent'
import type { AgentRunResponse, HealthResponse, RuntimeMode } from './types/agent'

const view = ref<'console' | 'workbench'>('console')
const task = ref('检查当前系统 SSH 登录异常')
const mode = ref<RuntimeMode>('stable')
const loading = ref(false)
const error = ref('')
const healthError = ref('')
const health = ref<HealthResponse | null>(null)
const latestResponse = ref<AgentRunResponse | null>(null)

onMounted(() => {
  void refreshHealth()
})

async function refreshHealth() {
  try {
    healthError.value = ''
    health.value = await getHealth()
  } catch (err) {
    health.value = null
    healthError.value = err instanceof Error ? err.message : 'health check failed'
  }
}

async function runTask() {
  const nextTask = task.value.trim()
  if (!nextTask) return

  loading.value = true
  error.value = ''
  try {
    latestResponse.value = mode.value === 'eino' ? await runAgentEino(nextTask) : await runAgent(nextTask)
    await refreshHealth()
  } catch (err) {
    latestResponse.value = null
    error.value = err instanceof Error ? err.message : 'Agent request failed'
  } finally {
    loading.value = false
  }
}
</script>

<style>
/* Minimal nav bar overrides */
.nav-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 40px;
  padding: 0 20px;
  background: #1d2129;
  color: #fff;
  font-size: 13px;
}
.nav-left { display: flex; align-items: center; gap: 8px; }
.nav-brand { font-weight: 600; }
.nav-version { color: #888; font-size: 11px; }
.nav-tabs { display: flex; gap: 0; }
.nav-tab {
  background: transparent; border: none; color: #aaa; cursor: pointer;
  padding: 8px 16px; font-size: 13px; transition: all 0.2s;
}
.nav-tab:hover { color: #fff; background: rgba(255,255,255,0.08); }
.nav-tab.active { color: #fff; border-bottom: 2px solid #165dff; background: rgba(22,93,255,0.12); }
</style>
