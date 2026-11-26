<script setup lang="ts">
import { useWorkspaceStore } from '@/stores/useWorkspaceStore'
import { desktopApi } from '@/services/bridge'

const store = useWorkspaceStore()

async function pickTxt() {
  try {
    const path = await desktopApi.pickTxtFile()
    if (path) {
      await store.openTxtPath(path)
    }
  } catch (e) {
    store.setStatus(`选择文件失败：${e instanceof Error ? e.message : String(e)}`, 'error')
  }
}

async function loadExternalConfig() {
  try {
    const path = await desktopApi.pickConfigFile()
    if (!path) return
    const loaded = await desktopApi.loadDesktopConfigFrom(path)
    store.config = loaded
    store.setStatus(`已加载配置：${path}`, 'ok')
    if (store.book) {
      await store.detectChapters()
    }
  } catch (e) {
    store.setStatus(`读取配置失败：${e instanceof Error ? e.message : String(e)}`, 'error')
  }
}
</script>

<template>
  <header class="topbar">
    <div class="topbar-left">
      <div class="brand-badge">
        <span class="brand-icon">EP</span>
        <span>EasyPub</span>
        <span class="brand-ver">{{ store.version }}</span>
      </div>

      <div class="segmented" role="tablist" aria-label="工作模式">
        <button
          type="button"
          :class="{ active: store.mode === 'single' }"
          @click="store.mode = 'single'"
        >
          单本精排
        </button>
        <button
          type="button"
          :class="{ active: store.mode === 'batch' }"
          @click="store.mode = 'batch'"
        >
          批量目录
        </button>
      </div>
    </div>

    <div class="topbar-right">
      <button class="btn btn-soft" type="button" :disabled="store.busy" @click="pickTxt">
        导入 TXT
      </button>
      <button class="btn" type="button" :disabled="store.busy" @click="loadExternalConfig">
        读取配置
      </button>
      <button class="btn" type="button" :disabled="store.busy" @click="store.saveConfig">
        保存配置
      </button>
      <button
        v-if="store.mode === 'single'"
        class="btn"
        :class="{ 'btn-active': store.previewOpen }"
        type="button"
        @click="store.previewOpen = !store.previewOpen"
      >
        {{ store.previewOpen ? '收起预览' : '展开预览' }}
      </button>
      <button
        class="btn"
        :class="{ 'btn-active': store.config.advanced.alwaysOnTop }"
        type="button"
        @click="store.toggleAlwaysOnTop"
      >
        置顶
      </button>
    </div>
  </header>
</template>
