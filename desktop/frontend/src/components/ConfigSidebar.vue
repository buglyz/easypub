<script setup lang="ts">
import { useWorkspaceStore } from '@/stores/useWorkspaceStore'
import type { ConfigTab } from '@/types/desktop'
import BookConfigTab from './BookConfigTab.vue'
import SplitConfigTab from './SplitConfigTab.vue'
import StyleConfigTab from './StyleConfigTab.vue'
import AdvancedConfigTab from './AdvancedConfigTab.vue'

const store = useWorkspaceStore()

const tabs: { key: ConfigTab; label: string }[] = [
  { key: 'book', label: '书籍与输出' },
  { key: 'split', label: '章节分割' },
  { key: 'style', label: '版式字体' },
  { key: 'advanced', label: '高级选项' },
]
</script>

<template>
  <aside class="sidebar" aria-label="参数设置">
    <nav class="sidebar-tabs">
      <button
        v-for="tab in tabs"
        :key="tab.key"
        type="button"
        :class="{ active: store.activeTab === tab.key }"
        @click="store.activeTab = tab.key"
      >
        {{ tab.label }}
      </button>
    </nav>

    <div class="sidebar-body">
      <BookConfigTab v-if="store.activeTab === 'book'" />
      <SplitConfigTab v-else-if="store.activeTab === 'split'" />
      <StyleConfigTab v-else-if="store.activeTab === 'style'" />
      <AdvancedConfigTab v-else />
    </div>
  </aside>
</template>
