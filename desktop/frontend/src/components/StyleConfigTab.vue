<script setup lang="ts">
import { computed } from 'vue'
import { useWorkspaceStore } from '@/stores/useWorkspaceStore'

const store = useWorkspaceStore()

const currentReader = computed(() =>
  store.config.ereaders.find((r) => r.index === store.config.style.fontIndex),
)

function resetDefaults() {
  store.config.style.lineHeight = 120
  store.config.style.fontSize = 100
  store.config.style.marginTop = 5
  store.config.style.indent = 0
  store.config.style.textAlign = 0
}
</script>

<template>
  <div>
    <div class="grid-2">
      <div class="field">
        <span class="field-label">
          <span>正文行高</span>
          <span class="field-hint">%</span>
        </span>
        <input
          v-model.number="store.config.style.lineHeight"
          class="input"
          type="number"
          min="80"
          max="300"
          step="5"
        />
      </div>

      <div class="field">
        <span class="field-label">
          <span>正文字号</span>
          <span class="field-hint">%</span>
        </span>
        <input
          v-model.number="store.config.style.fontSize"
          class="input"
          type="number"
          min="50"
          max="300"
          step="5"
        />
      </div>
    </div>

    <div class="grid-2">
      <div class="field">
        <span class="field-label">
          <span>段前间距</span>
          <span class="field-hint">px</span>
        </span>
        <input
          v-model.number="store.config.style.marginTop"
          class="input"
          type="number"
          min="0"
          max="50"
        />
      </div>

      <div class="field">
        <span class="field-label">
          <span>首行缩进</span>
          <span class="field-hint">em（0=保留全角空格）</span>
        </span>
        <input
          v-model.number="store.config.style.indent"
          class="input"
          type="number"
          min="0"
          max="4"
          step="0.5"
        />
      </div>
    </div>

    <div class="field">
      <span class="field-label">正文对齐方式</span>
      <select v-model.number="store.config.style.textAlign" class="select">
        <option :value="0">两端对齐 (justify · 默认)</option>
        <option :value="1">左对齐 (left)</option>
        <option :value="2">居中对齐 (center)</option>
        <option :value="3">右对齐 (right)</option>
      </select>
    </div>

    <div style="margin-bottom: 14px">
      <button class="btn btn-sm" type="button" @click="resetDefaults">
        恢复老牛经典默认版式
      </button>
    </div>

    <div class="divider"></div>

    <div class="field">
      <span class="field-label">
        <span>阅读器字体方案</span>
        <span class="field-hint">ereaders.xml</span>
      </span>
      <select v-model.number="store.config.style.fontIndex" class="select">
        <option v-for="er in store.config.ereaders" :key="er.index" :value="er.index">
          {{ er.name }}
        </option>
      </select>
    </div>

    <div v-if="currentReader" class="field">
      <span class="field-label">
        <span>当前方案字体引用路径</span>
        <span class="field-hint">共 {{ currentReader.fonts.length }} 条</span>
      </span>
      <div class="file-box loaded" style="max-height: 150px; overflow-y: auto; margin-bottom: 0">
        <div
          v-for="(f, idx) in currentReader.fonts"
          :key="idx"
          class="file-meta"
          style="font-family: var(--font-mono); margin-top: 2px"
        >
          {{ f }}
        </div>
      </div>
    </div>
  </div>
</template>
