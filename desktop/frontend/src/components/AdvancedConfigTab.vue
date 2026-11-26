<script setup lang="ts">
import { useWorkspaceStore } from '@/stores/useWorkspaceStore'
import { desktopApi } from '@/services/bridge'

const store = useWorkspaceStore()

async function browseKindleGen() {
  const path = await desktopApi.pickKindleGenExe()
  if (path) {
    store.config.advanced.kindleGenExe = path
  }
}

async function openWorkDir() {
  if (!store.config.workDir) return
  try {
    await desktopApi.openInExplorer(store.config.workDir)
  } catch (e) {
    store.setStatus(`打开目录失败：${e instanceof Error ? e.message : String(e)}`, 'error')
  }
}
</script>

<template>
  <div>
    <div class="field">
      <span class="field-label">
        <span>kindlegen 可执行文件</span>
        <span class="field-hint">生成 MOBI 时调用</span>
      </span>
      <div class="input-row">
        <input
          v-model="store.config.advanced.kindleGenExe"
          class="input"
          type="text"
          placeholder="kindlegen_v2.9.exe"
        />
        <button class="btn" type="button" @click="browseKindleGen">浏览</button>
      </div>
    </div>

    <div class="field">
      <span class="field-label">MOBI 压缩等级</span>
      <select v-model.number="store.config.advanced.kindleGenCompress" class="select">
        <option :value="0">不压缩 (-c0 · 速度最快)</option>
        <option :value="1">标准 DOC 压缩 (-c1 · 推荐)</option>
        <option :value="2">Kindle Huffdic 高压缩 (-c2 · 体积最小但较慢)</option>
      </select>
    </div>

    <div class="field">
      <span class="field-label">
        <span>kindlegen 附加参数</span>
        <span class="field-hint">可选</span>
      </span>
      <input
        v-model="store.config.advanced.kindleGenOption"
        class="input"
        type="text"
        placeholder="例如：-verbose"
      />
    </div>

    <div class="divider"></div>

    <div class="field">
      <span class="field-label">当前配置文件路径</span>
      <div class="file-box loaded" style="margin-bottom: 8px">
        <div class="file-meta"><strong>config.xml：</strong>{{ store.config.configPath }}</div>
        <div class="file-meta"><strong>ereaders.xml：</strong>{{ store.config.ereadersPath }}</div>
      </div>
      <div class="toolbar-group">
        <button class="btn btn-sm" type="button" @click="store.saveConfig">
          保存当前参数到 config.xml
        </button>
        <button class="btn btn-sm" type="button" @click="openWorkDir">
          打开配置目录
        </button>
      </div>
    </div>
  </div>
</template>
