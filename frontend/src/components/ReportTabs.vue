<template>
  <section class="console-panel report-tabs">
    <div class="panel-heading">
      <span>报告详情</span>
      <el-tag size="small" effect="plain">{{ report?.audit_metadata?.report_version || 'no-report' }}</el-tag>
    </div>

    <el-empty v-if="!response" description="运行任务后展示证据链和安全报告" :image-size="90" />

    <el-tabs v-else model-value="evidence" class="dark-tabs">
      <el-tab-pane label="Evidence Chain" name="evidence">
        <EvidenceTable :items="report?.evidence_chain || []" />
      </el-tab-pane>
      <el-tab-pane label="Sensitive Resources" name="resources">
        <SensitiveResourceCards :items="report?.sensitive_resources || []" />
      </el-tab-pane>
      <el-tab-pane label="Risk Explanation" name="risks">
        <RiskExplanationList :items="report?.risk_explanation || []" />
      </el-tab-pane>
      <el-tab-pane label="Recommendations" name="recommendations">
        <RecommendationList :items="report?.recommendations || []" />
      </el-tab-pane>
      <el-tab-pane v-if="isEinoMode" label="Eino Metadata" name="metadata">
        <EinoMetadataPanel :report="report" />
      </el-tab-pane>
      <el-tab-pane label="Reasoning Trace" name="trace">
        <ReasoningTracePanel :trace="response?.reasoning_trace || null" />
      </el-tab-pane>
      <el-tab-pane label="Raw JSON" name="raw">
        <RawJsonPanel :data="response" />
      </el-tab-pane>
    </el-tabs>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { AgentRunResponse } from '../types/agent'
import EvidenceTable from './EvidenceTable.vue'
import SensitiveResourceCards from './SensitiveResourceCards.vue'
import RiskExplanationList from './RiskExplanationList.vue'
import RecommendationList from './RecommendationList.vue'
import EinoMetadataPanel from './EinoMetadataPanel.vue'
import ReasoningTracePanel from './ReasoningTracePanel.vue'
import RawJsonPanel from './RawJsonPanel.vue'

const props = defineProps<{
  response: AgentRunResponse | null
}>()

const report = computed(() => props.response?.security_report || null)

const isEinoMode = computed(() => {
  return props.response?.security_report?.audit_metadata?.runtime === 'eino'
})
</script>
