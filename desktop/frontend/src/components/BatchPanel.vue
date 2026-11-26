<script setup lang="ts">
import { useWorkspaceStore } from '@/stores/useWorkspaceStore'
import { desktopApi } from '@/services/bridge'

const store = useWorkspaceStore()

async function pickInputDir() {
  const dir = await desktopApi.pickDirectory('选择包含 TXT 文件的目录')
  if (dir) {
    store.batchInputDir = dir
    if (!store.batchOutputDir) {
      store.batchOutputDir = dir
    }
  }
}

async function pickOutputDir() {
  const dir = await desktopApi.pickDirectory('选择 EPUB 批量输出目录')
  if (dir) {
    store.batchOutputDir = dir
  }
}

async function openTarget(path: string) {
  if (!path) return
  try {
    await desktopApi.openInExplorer(path)
  } catch (e) {
    store.setStatus(`打开路径失败：${e instanceof Error ? e.message : String(e)}`, 'error')
  }
}
</script>

<template>
  <main class="batch-wrap" aria-label="批量转换工作台">
    <aside class="batch-config">
      <h3 style="margin: 0 0 12px; font-size: 15px">批量目录转换</h3>
      <p class="file-meta" style="margin: 0 0 14px">
        扫描目录内所有 .txt / .utf8 / .gbk 文件并批量生成 EPUB，支持多核并发与幂等跳过。
      </p>

      <div class="field">
        <span class="field-label">TXT 输入目录</span>
        <div class="input-row">
          <input
            v-model="store.batchInputDir"
            class="input"
            type="text"
            placeholder="拖入文件夹或点击浏览"
          />
          <button class="btn" type="button" @click="pickInputDir">浏览</button>
        </div>
      </div>

      <div class="field">
        <span class="field-label">
          <span>EPUB 输出目录</span>
          <span class="field-hint">留空=输入目录</span>
        </span>
        <div class="input-row">
          <input
            v-model="store.batchOutputDir"
            class="input"
            type="text"
            placeholder="默认输出到源目录"
          />
          <button class="btn" type="button" @click="pickOutputDir">浏览</button>
        </div>
      </div>

      <div class="grid-2">
        <div class="field">
          <span class="field-label">并发线程数</span>
          <select v-model.number="store.batchJobs" class="select">
            <option :value="0">自动（按 CPU 核数）</option>
            <option :value="1">1（单线程顺序）</option>
            <option :value="2">2 线程</option>
            <option :value="4">4 线程</option>
            <option :value="8">8 线程</option>
          </select>
        </div>

        <div class="field">
          <span class="field-label">统一作者（可选）</span>
          <input v-model="store.author" class="input" type="text" placeholder="留空则不写" />
        </div>
      </div>

      <div class="field">
        <label class="check-item">
          <input v-model="store.batchSkipExisting" type="checkbox" />
          <span>跳过已存在的同名 EPUB（幂等增量）</span>
        </label>
      </div>

      <div class="field">
        <label class="check-item">
          <input v-model="store.config.advanced.enableMobi" type="checkbox" />
          <span>同时生成 MOBI（调用 kindlegen）</span>
        </label>
      </div>

      <button
        class="btn btn-primary"
        style="width: 100%; height: 36px; margin-top: 6px"
        type="button"
        :disabled="!store.batchInputDir || store.busy"
        @click="store.runBatch"
      >
        {{ store.busy ? '正在批量转换...' : '开始批量转换' }}
      </button>
    </aside>

    <section class="batch-results">
      <div class="pane-toolbar">
        <div class="toolbar-group">
          <strong>转换结果明细</strong>
          <span v-if="store.batchResult" class="field-hint">
            共 {{ store.batchResult.total }} 本 · 成功 {{ store.batchResult.succeeded }} ·
            跳过 {{ store.batchResult.skipped }} · 失败 {{ store.batchResult.failed }} ·
            耗时 {{ (store.batchResult.durationMs / 1000).toFixed(2) }}s
          </span>
        </div>

        <button
          v-if="store.batchResult"
          class="btn btn-sm"
          type="button"
          @click="openTarget(store.batchResult.outputDir)"
        >
          打开输出文件夹
        </button>
      </div>

      <div v-if="!store.batchResult" class="empty-pane">
        <strong>尚未执行批量任务</strong>
        <span>将整个小说文件夹拖入窗口即可自动填入路径</span>
      </div>

      <div v-else class="chapter-table-wrap">
        <table class="chapter-table">
          <thead>
            <tr>
              <th style="width: 72px">状态</th>
              <th>源文件</th>
              <th style="width: 90px">编码</th>
              <th style="width: 76px; text-align: right">章节数</th>
              <th>输出路径 / 说明</th>
              <th style="width: 76px; text-align: center">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(it, idx) in store.batchResult.items" :key="idx">
              <td>
                <span v-if="it.status === 'ok'" class="tag tag-ok">成功</span>
                <span v-else-if="it.status === 'skipped'" class="tag tag-warn">跳过</span>
                <span v-else class="tag tag-danger">失败</span>
              </td>
              <td>{{ it.fileName }}</td>
              <td>{{ it.encoding || '-' }}</td>
              <td style="text-align: right">{{ it.chapters || '-' }}</td>
              <td :title="it.error || it.output">{{ it.error || it.output }}</td>
              <td style="text-align: center">
                <button
                  v-if="it.status === 'ok'"
                  class="btn btn-sm"
                  type="button"
                  @click="openTarget(it.output)"
                >
                  定位
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </main>
</template>
