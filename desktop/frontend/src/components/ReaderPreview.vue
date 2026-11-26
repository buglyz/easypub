<script setup lang="ts">
import { computed } from 'vue'
import { useWorkspaceStore } from '@/stores/useWorkspaceStore'

const store = useWorkspaceStore()

const alignMap: Record<number, 'justify' | 'left' | 'center' | 'right'> = {
  0: 'justify',
  1: 'left',
  2: 'center',
  3: 'right',
}

const paragraphStyle = computed(() => ({
  fontSize: `${store.config.style.fontSize}%`,
  lineHeight: `${store.config.style.lineHeight}%`,
  marginTop: `${store.config.style.marginTop}px`,
  textIndent: store.config.style.indent > 0 ? `${store.config.style.indent}em` : '0em',
  textAlign: alignMap[store.config.style.textAlign] ?? 'justify',
}))
</script>

<template>
  <aside class="preview-pane" aria-label="实时阅读预览">
    <div class="pane-toolbar">
      <div class="segmented">
        <button
          type="button"
          :class="{ active: store.previewMode === 'reader' }"
          @click="store.previewMode = 'reader'"
        >
          排版预览
        </button>
        <button
          type="button"
          :class="{ active: store.previewMode === 'raw' }"
          @click="store.previewMode = 'raw'"
        >
          原始行号
        </button>
      </div>

      <span v-if="store.preview" class="field-hint">
        行 {{ store.preview.startLine }}~{{ store.preview.endLine }} ·
        {{ store.preview.charCount.toLocaleString() }} 字
      </span>
    </div>

    <div v-if="!store.preview" class="empty-pane">
      <span>在中间列表点击任意章节查看实时排版效果</span>
    </div>

    <div v-else class="reader-scroll">
      <div v-if="store.previewMode === 'reader'" class="reader-page">
        <h2 v-if="store.preview.title" class="reader-chapter-title">
          {{ store.preview.title }}
        </h2>
        <p
          v-for="(para, idx) in store.preview.paragraphs"
          :key="idx"
          class="reader-para"
          :style="paragraphStyle"
        >
          {{ para || '\u00A0' }}
        </p>
        <div v-if="store.preview.truncated" class="file-meta" style="margin-top: 12px">
          （本章较长，预览仅展示前 300 段）
        </div>
      </div>

      <div v-else class="reader-page">
        <div class="raw-pre">
          <div
            v-for="(line, idx) in store.preview.rawLines"
            :key="idx"
            class="raw-line"
          >
            <span class="raw-num">{{ store.preview.startLine + idx }}</span>
            <span>{{ line }}</span>
          </div>
        </div>
      </div>
    </div>
  </aside>
</template>
