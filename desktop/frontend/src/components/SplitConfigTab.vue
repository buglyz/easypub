<script setup lang="ts">
import { computed } from 'vue'
import { useWorkspaceStore } from '@/stores/useWorkspaceStore'

const store = useWorkspaceStore()

const additionalText = computed({
  get: () => store.config.split.additionalReg.join('\n'),
  set: (val: string) => {
    store.config.split.additionalReg = val
      .split('\n')
      .map((s) => s.trim())
      .filter(Boolean)
  },
})

function onPresetSelect(event: Event) {
  const target = event.target as HTMLSelectElement
  if (target.value) {
    store.config.split.fullReg = target.value
    target.value = ''
  }
}
</script>

<template>
  <div>
    <div class="field">
      <span class="field-label">章节识别模式</span>
      <select v-model="store.config.split.mode" class="select">
        <option value="auto">智能自动识别（内置中英文常见章回）</option>
        <option value="simple">简易三段式正则（原版经典）</option>
        <option value="full">完整正则表达式</option>
        <option value="count">按固定字数均分</option>
        <option value="whole">整本合并为一章（不分章）</option>
      </select>
    </div>

    <template v-if="store.config.split.mode === 'simple'">
      <div class="grid-2">
        <div class="field">
          <span class="field-label">前段字符类</span>
          <input v-model="store.config.split.simpleRegP1" class="input" type="text" />
        </div>
        <div class="field">
          <span class="field-label">数字风格</span>
          <select v-model.number="store.config.split.simpleRegP2" class="select">
            <option :value="0">阿拉伯数字 (0-9)</option>
            <option :value="1">中文数字 (一二三...)</option>
          </select>
        </div>
      </div>
      <div class="grid-2">
        <div class="field">
          <span class="field-label">尾段字符类</span>
          <input v-model="store.config.split.simpleRegP3" class="input" type="text" />
        </div>
        <div class="field">
          <span class="field-label">尾段扩展</span>
          <input
            v-model="store.config.split.simpleRegExt"
            class="input"
            type="text"
            placeholder="可选"
          />
        </div>
      </div>
      <div class="field">
        <label class="check-item">
          <input v-model="store.config.split.simpleRegLeadingSpace" type="checkbox" />
          <span>允许标题行首存在空白字符</span>
        </label>
      </div>
    </template>

    <template v-else-if="store.config.split.mode === 'full'">
      <div v-if="store.config.presets.fullRegs.length" class="field">
        <span class="field-label">预定义正则模板</span>
        <select class="select" @change="onPresetSelect">
          <option value="">选择模板填入下方...</option>
          <option v-for="(reg, idx) in store.config.presets.fullRegs" :key="idx" :value="reg">
            {{ reg }}
          </option>
        </select>
      </div>
      <div class="field">
        <span class="field-label">完整正则表达式</span>
        <textarea
          v-model="store.config.split.fullReg"
          class="textarea"
          placeholder="例如：^\s*第\s*[0-9一二三四五六七八九十百千]+\s*章.*"
        ></textarea>
      </div>
    </template>

    <template v-else-if="store.config.split.mode === 'count'">
      <div class="field">
        <span class="field-label">
          <span>每章目标字数</span>
          <span class="field-hint">最小 100 字</span>
        </span>
        <input
          v-model.number="store.config.split.splitCount"
          class="input"
          type="number"
          min="100"
          max="500000"
          step="500"
        />
      </div>
    </template>

    <div
      v-if="store.config.split.mode !== 'count' && store.config.split.mode !== 'whole'"
      class="field"
    >
      <span class="field-label">
        <span>附加标题正则</span>
        <span class="field-hint">每行一条，用于序言/番外等</span>
      </span>
      <textarea
        v-model="additionalText"
        class="textarea"
        placeholder="^\s*(楔子|序言|后记|番外)"
      ></textarea>
    </div>

    <div class="divider"></div>

    <div class="field">
      <label class="check-item">
        <input v-model="store.config.split.removeBlankLine" type="checkbox" />
        <span>删除原文本中的多余空行</span>
      </label>
    </div>

    <div class="field">
      <label class="check-item">
        <input v-model="store.config.split.addSpace" type="checkbox" />
        <span>段落之间插入空行（关闭「删除空行」时生效）</span>
      </label>
    </div>

    <div v-if="store.config.split.addSpace && !store.config.split.removeBlankLine" class="field">
      <span class="field-label">插入空行数量</span>
      <input
        v-model.number="store.config.split.addSpaceCount"
        class="input"
        type="number"
        min="1"
        max="10"
      />
    </div>

    <div class="field">
      <label class="check-item">
        <input v-model="store.config.split.forceEmptyChapter" type="checkbox" />
        <span>保留无正文的空标题章节（如「第一部」卷标）</span>
      </label>
    </div>

    <button
      class="btn btn-primary"
      style="width: 100%"
      type="button"
      :disabled="!store.book || store.busy"
      @click="store.detectChapters"
    >
      重新识别章节（重置手动编辑）
    </button>
  </div>
</template>
