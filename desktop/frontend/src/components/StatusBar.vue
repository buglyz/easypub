<script setup lang="ts">
import { useWorkspaceStore } from '@/stores/useWorkspaceStore'
import { desktopApi } from '@/services/bridge'

const store = useWorkspaceStore()

async function openResultFile() {
  const target = store.lastResult?.epubPath || store.batchResult?.outputDir
  if (!target) return
  try {
    await desktopApi.openInExplorer(target)
  } catch (e) {
    store.setStatus(`打开输出目录失败：${e instanceof Error ? e.message : String(e)}`, 'error')
  }
}
</script>

<template>
  <footer class="statusbar">
    <div class="status-left">
      <span class="status-dot" :class="store.statusTone"></span>
      <span class="status-msg" :title="store.statusText">{{ store.statusText }}</span>
    </div>

    <div class="status-right">
      <span v-if="store.mode === 'single' && store.book" class="status-stat">
        {{ store.book.encoding }} · {{ store.activeChapterCount }} 章 ·
        {{ (store.book.totalChars / 10000).toFixed(2) }} 万字
      </span>

      <button
        v-if="(store.mode === 'single' && store.lastResult) || (store.mode === 'batch' && store.batchResult)"
        class="btn"
        type="button"
        @click="openResultFile"
      >
        打开输出位置
      </button>

      <button
        v-if="store.mode === 'single'"
        class="btn btn-primary"
        style="height: 34px; padding: 0 18px"
        type="button"
        :disabled="!store.book || store.busy"
        @click="store.convertSingle"
      >
        {{ store.busy ? '正在生成...' : '生成电子书' }}
      </button>
    </div>
  </footer>
</template>
