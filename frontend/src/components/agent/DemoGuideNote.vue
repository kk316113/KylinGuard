<template>
  <div v-if="show" class="demo-guide">
    <a-alert type="warning" :closable="false" show-icon>
      <template #title>
        <div class="demo-title">演示说明</div>
      </template>
      <div class="demo-body">
        <p>本场景展示了以下能力：</p>
        <ul>
          <li v-for="(item, i) in items" :key="i">{{ item }}</li>
        </ul>
      </div>
    </a-alert>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  show: boolean
  items: string[]
}>()
</script>

<style scoped>
.demo-guide { margin-top: 12px; }
.demo-title { font-weight: 600; font-size: 14px; }
.demo-body { font-size: 13px; line-height: 1.6; }
.demo-body ul { margin: 4px 0 0 16px; padding: 0; }
.demo-body li { margin-bottom: 2px; color: #4e5969; }
</style>
