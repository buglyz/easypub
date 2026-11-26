<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { useWorkspaceStore } from '@/stores/useWorkspaceStore'
import { desktopApi } from '@/services/bridge'
import TopBar from '@/components/TopBar.vue'
import ConfigSidebar from '@/components/ConfigSidebar.vue'
import ChapterManager from '@/components/ChapterManager.vue'
import ReaderPreview from '@/components/ReaderPreview.vue'
import BatchPanel from '@/components/BatchPanel.vue'
import StatusBar from '@/components/StatusBar.vue'

const store = useWorkspaceStore()

const IMAGE_EXTS = ['.jpg', '.jpeg', '.png', '.webp', '.gif']

async function handleDroppedPaths(paths: string[]) {
  if (!paths.length || store.busy) return
  const first = paths[0]
  try {
    const isDir = await desktopApi.isDirectory(first)
    if (isDir) {
      store.mode = 'batch'
      store.batchInputDir = first
      if (!store.batchOutputDir) {
        store.batchOutputDir = first
      }
      store.setStatus(`已载入批量目录：${first}`, 'ok')
      return
    }
    const lower = first.toLowerCase()
    if (IMAGE_EXTS.some((ext) => lower.endsWith(ext))) {
      const info = await desktopApi.loadCoverPreview(first)
      if (info) {
        store.cover = info
        store.mode = 'single'
        store.activeTab = 'book'
        store.setStatus(`已设置封面图片：${info.name}`, 'ok')
      }
      return
    }
    await store.openTxtPath(first)
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    store.setStatus(`拖拽处理失败：${msg}`, 'error')
  }
}

onMounted(async () => {
  await store.init()
  desktopApi.onFileDrop((paths) => {
    void handleDroppedPaths(paths)
  })
})

onUnmounted(() => {
  desktopApi.offFileDrop()
})
</script>

<template>
  <div class="app-shell" @dragover.prevent @drop.prevent>
    <TopBar />

    <main
      v-if="store.mode === 'single'"
      class="workspace-single"
      :class="{ 'preview-collapsed': !store.previewOpen }"
    >
      <ConfigSidebar />
      <ChapterManager />
      <ReaderPreview v-if="store.previewOpen" />
    </main>

    <BatchPanel v-else />

    <StatusBar />
  </div>
</template>
