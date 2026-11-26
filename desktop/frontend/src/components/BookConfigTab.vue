<script setup lang="ts">
import { useWorkspaceStore } from '@/stores/useWorkspaceStore'
import { desktopApi } from '@/services/bridge'

const store = useWorkspaceStore()

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1048576).toFixed(2)} MB`
}

async function selectTxt() {
  const path = await desktopApi.pickTxtFile()
  if (path) {
    await store.openTxtPath(path)
  }
}

async function selectCover() {
  try {
    const info = await desktopApi.pickCoverImage()
    if (info) {
      store.cover = info
      store.setStatus(`已选择封面图：${info.name}`, 'ok')
    }
  } catch (e) {
    store.setStatus(`封面加载失败：${e instanceof Error ? e.message : String(e)}`, 'error')
  }
}

async function selectOutFolder() {
  const dir = await desktopApi.pickDirectory('选择 EPUB 输出目录')
  if (dir) {
    store.config.advanced.outputFolder = dir
    store.config.advanced.outputToSrc = false
  }
}
</script>

<template>
  <div>
    <div class="file-box" :class="{ loaded: !!store.book }">
      <template v-if="store.book">
        <div class="file-title">{{ store.book.fileName }}</div>
        <div class="file-meta">
          {{ formatSize(store.book.sizeBytes) }} · {{ store.book.encoding }} ·
          {{ store.book.totalLines.toLocaleString() }} 行 ·
          {{ store.book.totalChars.toLocaleString() }} 字
        </div>
        <div class="file-meta">{{ store.book.path }}</div>
        <div style="margin-top: 8px">
          <button class="btn btn-sm" type="button" :disabled="store.busy" @click="selectTxt">
            更换 TXT
          </button>
        </div>
      </template>
      <template v-else>
        <div class="file-title">尚未导入 TXT 源文件</div>
        <div class="file-meta">可将 TXT 文件或图片直接拖入窗口，或点击下方选择</div>
        <div style="margin-top: 8px">
          <button class="btn btn-soft btn-sm" type="button" @click="selectTxt">
            选择本地 TXT
          </button>
        </div>
      </template>
    </div>

    <div class="field">
      <span class="field-label">
        <span>书名</span>
        <span class="field-hint">留空自动推断</span>
      </span>
      <input v-model="store.title" class="input" type="text" placeholder="电子书标题" />
    </div>

    <div class="field">
      <span class="field-label">作者</span>
      <input v-model="store.author" class="input" type="text" placeholder="可选" />
    </div>

    <div class="field">
      <span class="field-label">
        <span>封面设置</span>
        <span class="field-hint">{{ store.cover ? '图片封面' : '默认文字封面' }}</span>
      </span>
      <div class="cover-box">
        <div class="cover-thumb">
          <img v-if="store.cover" :src="store.cover.dataUrl" :alt="store.cover.name" />
          <span v-else>文字</span>
        </div>
        <div class="cover-actions">
          <div class="file-meta">
            {{ store.cover ? store.cover.name : '使用原版经典右侧竖线文字封面' }}
          </div>
          <div class="toolbar-group">
            <button class="btn btn-sm" type="button" @click="selectCover">选择图片</button>
            <button
              v-if="store.cover"
              class="btn btn-sm"
              type="button"
              @click="store.cover = null"
            >
              恢复文字封面
            </button>
          </div>
        </div>
      </div>
    </div>

    <div class="divider"></div>

    <div class="field">
      <label class="check-item">
        <input v-model="store.config.advanced.outputToSrc" type="checkbox" />
        <span>输出到 TXT 源文件所在目录</span>
      </label>
    </div>

    <div v-if="!store.config.advanced.outputToSrc" class="field">
      <span class="field-label">指定输出目录</span>
      <div class="input-row">
        <input
          v-model="store.config.advanced.outputFolder"
          class="input"
          type="text"
          placeholder="选择或输入输出文件夹"
        />
        <button class="btn" type="button" @click="selectOutFolder">浏览</button>
      </div>
    </div>

    <div class="field">
      <label class="check-item">
        <input v-model="store.config.advanced.enableMobi" type="checkbox" />
        <span>同时调用 kindlegen 输出 MOBI 格式</span>
      </label>
    </div>
  </div>
</template>
