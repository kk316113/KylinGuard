<template>
  <div class="table-wrap">
    <el-table :data="items" stripe class="dark-table" empty-text="暂无证据链" row-key="evidence_id">
      <el-table-column prop="evidence_id" label="Evidence" width="96" />
      <el-table-column prop="tool_name" label="Tool" width="160">
        <template #default="{ row }">
          <el-tag :class="`tool-tag tool-${row.tool_name.replace(/_/g, '-')}`" effect="dark">
            {{ row.tool_name }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="operation_type" label="Operation" width="120">
        <template #default="{ row }">
          <el-tag :type="row.operation_type === 'analyze' ? 'warning' : 'info'" effect="plain">
            {{ row.operation_type }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="resource_type" label="Resource" width="150" />
      <el-table-column prop="boundary_level" label="Boundary" width="190">
        <template #default="{ row }">
          <el-tag :class="{ 'sensitive-tag': row.boundary_level === 'sensitive_system_resource' }" effect="dark">
            {{ row.boundary_level }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="status" label="Status" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 'error' ? 'danger' : 'success'" effect="plain">
            {{ row.status }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="why_relevant" label="Why Relevant" min-width="260" />
      <el-table-column prop="audit_meaning" label="Audit Meaning" min-width="300" />
    </el-table>
  </div>
</template>

<script setup lang="ts">
import type { EvidenceItem } from '../types/agent'

defineProps<{
  items: EvidenceItem[]
}>()
</script>
