<template>
  <section class="console-panel plan-timeline">
    <div class="panel-heading">
      <span>执行链路</span>
      <el-tag size="small" effect="plain">{{ plan?.scenario || 'no-plan' }}</el-tag>
    </div>

    <div v-if="steps.length" class="timeline-list">
      <article v-for="step in steps" :key="step.step_id" class="timeline-step">
        <div class="step-index">{{ step.step_id }}</div>
        <div class="step-body">
          <div class="step-title-row">
            <el-tag :class="toolClass(step.tool_name)" effect="dark">{{ step.tool_name }}</el-tag>
            <span class="step-reason">{{ step.reason }}</span>
          </div>
          <code class="input-summary">{{ summarizeInput(step.input) }}</code>
        </div>
      </article>
    </div>

    <el-empty v-else description="未生成工具计划" :image-size="80" />
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Plan, PlanStep } from '../types/agent'

const props = defineProps<{
  plan?: Plan | null
}>()

const steps = computed<PlanStep[]>(() => props.plan?.steps ?? [])

function summarizeInput(input: Record<string, unknown>): string {
  const text = JSON.stringify(input ?? {})
  return text.length > 130 ? `${text.slice(0, 130)}...` : text
}

function toolClass(toolName: string): string {
  return `tool-tag tool-${toolName.replace(/_/g, '-')}`
}
</script>
