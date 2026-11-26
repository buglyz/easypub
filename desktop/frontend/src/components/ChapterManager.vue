<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { useWorkspaceStore } from '@/stores/useWorkspaceStore'
import type { ChapterItem } from '@/types/desktop'
import InsertChapterModal from './InsertChapterModal.vue'

const store = useWorkspaceStore()

const keyword = ref('')
const shortLimit = ref(150)
const highlightShort = ref(false)

const editingId = ref<number | null>(null)
const editingTitle = ref('')
const editInputRef = ref<HTMLInputElement | null>(null)
const insertModalOpen = ref(false)

const filteredChapters = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return store.chapters
  return store.chapters.filter(
    (c) => c.title.toLowerCase().includes(kw) || String(c.startLine).includes(kw),
  )
})

function isSelected(id: number): boolean {
  return store.selectedIds.includes(id)
}

function selectRow(item: ChapterItem, event: MouseEvent) {
  if (event.shiftKey && store.selectedIds.length > 0) {
    const anchor = store.selectedIds[store.selectedIds.length - 1]
    const list = filteredChapters.value
    const idxA = list.findIndex((c) => c.id === anchor)
    const idxB = list.findIndex((c) => c.id === item.id)
    if (idxA >= 0 && idxB >= 0) {
      const [from, to] = idxA < idxB ? [idxA, idxB] : [idxB, idxA]
      store.selectedIds = list.slice(from, to + 1).map((c) => c.id)
    }
  } else if (event.ctrlKey || event.metaKey) {
    store.selectedIds = isSelected(item.id)
      ? store.selectedIds.filter((id) => id !== item.id)
      : [...store.selectedIds, item.id]
  } else {
    store.selectedIds = [item.id]
  }
  void store.loadPreview(item)
}

async function beginRename(item: ChapterItem) {
  editingId.value = item.id
  editingTitle.value = item.title
  await nextTick()
  editInputRef.value?.focus()
  editInputRef.value?.select()
}

function commitRename() {
  if (editingId.value !== null) {
    store.renameChapter(editingId.value, editingTitle.value)
    editingId.value = null
  }
}

function selectShortChapters() {
  highlightShort.value = true
  const ids = store.chapters
    .filter((c) => c.state === 'normal' && c.charCount < shortLimit.value)
    .map((c) => c.id)
  store.selectedIds = ids
  store.setStatus(`已选中 ${ids.length} 个少于 ${shortLimit.value} 字的章节`, 'idle')
}

function toggleSelectedExcluded() {
  if (!store.selectedIds.length) return
  const targets = store.chapters.filter((c) => store.selectedIds.includes(c.id))
  const allNormal = targets.every((c) => c.state === 'normal')
  store.updateChaptersState(store.selectedIds, allNormal ? 'excluded' : 'normal')
}

function onTableKeydown(event: KeyboardEvent) {
  if (editingId.value !== null || !store.chapters.length) return
  if (event.key === 'Enter') {
    event.preventDefault()
    toggleSelectedExcluded()
  } else if (event.key === 'Delete') {
    event.preventDefault()
    store.updateChaptersState(store.selectedIds, event.shiftKey ? 'purged' : 'excluded')
  } else if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    const list = filteredChapters.value
    const curIdx = list.findIndex((c) => c.id === store.activeChapterId)
    const nextIdx = event.key === 'ArrowDown' ? curIdx + 1 : curIdx - 1
    if (nextIdx >= 0 && nextIdx < list.length) {
      store.selectedIds = [list[nextIdx].id]
      void store.loadPreview(list[nextIdx])
    }
  }
}

async function onInsertSubmit(lineNo: number, chapterTitle: string) {
  await store.insertChapter(lineNo, chapterTitle)
  insertModalOpen.value = false
}
</script>

<template>
  <section class="center-pane" aria-label="章节管理器">
    <div class="pane-toolbar">
      <div class="toolbar-group">
        <input v-model="keyword" class="toolbar-input" style="width: 148px" type="text" placeholder="搜索章节或行号..." />
        <input v-model.number="shortLimit" class="toolbar-input" style="width: 68px" type="number" min="1" title="短章节字数阈值" />
        <button class="btn btn-sm" type="button" :disabled="!store.chapters.length" @click="selectShortChapters">筛选短章</button>
      </div>

      <div class="toolbar-group">
        <button class="btn btn-sm" type="button" :disabled="!store.selectedIds.length" @click="toggleSelectedExcluded">合并/恢复</button>
        <button class="btn btn-sm btn-danger" type="button" :disabled="!store.selectedIds.length" @click="store.updateChaptersState(store.selectedIds, 'purged')">彻底删除</button>
        <button class="btn btn-sm" type="button" :disabled="!store.selectedIds.length" @click="store.updateChaptersState(store.selectedIds, 'normal')">全部恢复</button>
        <button class="btn btn-sm btn-soft" type="button" :disabled="!store.book" @click="insertModalOpen = true">插入章节</button>
      </div>
    </div>

    <div v-if="!store.book" class="empty-pane">
      <strong>拖拽 TXT 到此处，或点击左上角「导入 TXT」</strong>
      <span>导入后可在此双击改名、合并误切目录、筛选短章及实时预览排版</span>
    </div>

    <div v-else class="chapter-table-wrap" tabindex="0" @keydown="onTableKeydown">
      <table class="chapter-table">
        <thead>
          <tr>
            <th class="col-idx">序号</th>
            <th>章节标题（双击重命名）</th>
            <th class="col-state">状态</th>
            <th class="col-line">起始行</th>
            <th class="col-chars">字数</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(item, idx) in filteredChapters"
            :key="item.id"
            :class="{
              selected: isSelected(item.id),
              'active-row': store.activeChapterId === item.id,
              'state-excluded': item.state === 'excluded',
              'state-purged': item.state === 'purged',
              'is-short': highlightShort && item.charCount < shortLimit,
            }"
            @click="selectRow(item, $event)"
          >
            <td class="col-idx" title="双击合并或恢复目录" @dblclick.stop="store.toggleChapterExcluded(item.id)">{{ idx + 1 }}</td>
            <td @dblclick.stop="beginRename(item)">
              <input
                v-if="editingId === item.id"
                ref="editInputRef"
                v-model="editingTitle"
                class="inline-edit"
                type="text"
                @blur="commitRename"
                @keydown.enter.stop="commitRename"
                @keydown.esc.stop="editingId = null"
              />
              <span v-else>{{ item.title || '（前置无标题段落）' }}</span>
            </td>
            <td class="col-state">
              <span v-if="item.state === 'normal'" class="tag tag-ok">正常</span>
              <span v-else-if="item.state === 'excluded'" class="tag tag-muted">合并</span>
              <span v-else class="tag tag-danger">已删</span>
            </td>
            <td class="col-line" :title="`共 ${item.lineCount} 行`">{{ item.startLine }}</td>
            <td class="col-chars">{{ item.charCount.toLocaleString() }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="shortcut-bar">
      <span>快捷键：Enter 合并/恢复 · Del 合并上一章 · Shift+Del 彻底删除 · 双击标题改名</span>
      <span>
        有效 {{ store.activeChapterCount }} 章
        <template v-if="store.excludedCount"> · 已合并 {{ store.excludedCount }}</template>
        <template v-if="store.purgedCount"> · 已删 {{ store.purgedCount }}</template>
      </span>
    </div>

    <InsertChapterModal
      :open="insertModalOpen"
      :max-lines="store.book?.totalLines ?? 1"
      @close="insertModalOpen = false"
      @submit="onInsertSubmit"
    />
  </section>
</template>
