import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { desktopApi } from '@/services/bridge'
import { createDefaultConfig } from '@/utils/defaults'
import type {
  BatchResponse,
  ChapterItem,
  ChapterPreview,
  ChapterState,
  ConfigTab,
  ConvertResponse,
  CoverInfo,
  DesktopConfig,
  LoadedBook,
  PreviewMode,
  WorkspaceMode,
} from '@/types/desktop'

export const useWorkspaceStore = defineStore('workspace', () => {
  const version = ref('dev')
  const mode = ref<WorkspaceMode>('single')
  const activeTab = ref<ConfigTab>('book')
  const previewOpen = ref(true)
  const previewMode = ref<PreviewMode>('reader')

  const config = ref<DesktopConfig>(createDefaultConfig())
  const book = ref<LoadedBook | null>(null)
  const title = ref('')
  const author = ref('')
  const outputPath = ref('')
  const cover = ref<CoverInfo | null>(null)

  const chapters = ref<ChapterItem[]>([])
  const selectedIds = ref<number[]>([])
  const activeChapterId = ref<number | null>(null)
  const preview = ref<ChapterPreview | null>(null)
  const previewLoading = ref(false)

  const busy = ref(false)
  const statusText = ref('就绪 · 拖拽 TXT 文件或文件夹到窗口即可开始')
  const statusTone = ref<'idle' | 'busy' | 'ok' | 'error'>('idle')
  const lastResult = ref<ConvertResponse | null>(null)

  const batchInputDir = ref('')
  const batchOutputDir = ref('')
  const batchSkipExisting = ref(true)
  const batchJobs = ref(0)
  const batchResult = ref<BatchResponse | null>(null)

  const activeChapterCount = computed(
    () => chapters.value.filter((c) => c.state === 'normal').length,
  )
  const excludedCount = computed(
    () => chapters.value.filter((c) => c.state === 'excluded').length,
  )
  const purgedCount = computed(
    () => chapters.value.filter((c) => c.state === 'purged').length,
  )

  function setStatus(text: string, tone: 'idle' | 'busy' | 'ok' | 'error' = 'idle') {
    statusText.value = text
    statusTone.value = tone
  }

  function errMsg(err: unknown): string {
    return err instanceof Error ? err.message : String(err)
  }

  async function init() {
    try {
      const [ver, cfg] = await Promise.all([
        desktopApi.getVersion(),
        desktopApi.loadDesktopConfig(),
      ])
      version.value = ver
      config.value = cfg
      if (cfg.advanced.outputFolder) {
        batchOutputDir.value = cfg.advanced.outputFolder
      }
    } catch (e) {
      setStatus(`初始化配置失败：${errMsg(e)}`, 'error')
    }
  }

  async function openTxtPath(path: string) {
    if (!path || busy.value) return
    busy.value = true
    setStatus('正在读取并识别文件编码...', 'busy')
    try {
      const loaded = await desktopApi.loadTxtFile(path)
      book.value = loaded
      title.value = loaded.inferredTitle
      outputPath.value = ''
      lastResult.value = null
      mode.value = 'single'
      await detectChapters()
      setStatus(`已载入 ${loaded.fileName}（${loaded.encoding}）`, 'ok')
    } catch (e) {
      setStatus(`导入失败：${errMsg(e)}`, 'error')
    } finally {
      busy.value = false
    }
  }

  async function detectChapters() {
    if (!book.value) return
    busy.value = true
    setStatus('正在识别章节结构...', 'busy')
    try {
      const list = await desktopApi.detectChapters(config.value.split)
      chapters.value = list
      selectedIds.value = list.length > 0 ? [list[0].id] : []
      activeChapterId.value = list.length > 0 ? list[0].id : null
      if (list.length > 0) {
        await loadPreview(list[0])
      } else {
        preview.value = null
      }
      setStatus(`识别完成：共 ${list.length} 章`, 'ok')
    } catch (e) {
      setStatus(`章节识别失败：${errMsg(e)}`, 'error')
    } finally {
      busy.value = false
    }
  }

  async function loadPreview(item: ChapterItem | null) {
    if (!item || !book.value) {
      preview.value = null
      return
    }
    activeChapterId.value = item.id
    previewLoading.value = true
    try {
      preview.value = await desktopApi.getChapterPreview(item, config.value.split)
    } catch (e) {
      setStatus(`预览加载失败：${errMsg(e)}`, 'error')
    } finally {
      previewLoading.value = false
    }
  }

  function updateChaptersState(ids: number[], state: ChapterState) {
    const idSet = new Set(ids)
    chapters.value = chapters.value.map((c) =>
      idSet.has(c.id) ? { ...c, state } : c,
    )
  }

  function toggleChapterExcluded(id: number) {
    chapters.value = chapters.value.map((c) => {
      if (c.id !== id) return c
      return { ...c, state: c.state === 'normal' ? 'excluded' : 'normal' }
    })
  }

  function renameChapter(id: number, nextTitle: string) {
    const trimmed = nextTitle.trim()
    chapters.value = chapters.value.map((c) =>
      c.id === id ? { ...c, title: trimmed } : c,
    )
    if (activeChapterId.value === id && preview.value) {
      preview.value = { ...preview.value, title: trimmed }
    }
  }

  async function insertChapter(lineNo: number, chapterTitle: string) {
    if (!book.value) return
    busy.value = true
    try {
      const updated = await desktopApi.insertChapterAtLine(
        chapters.value,
        lineNo,
        chapterTitle,
        config.value.split,
      )
      chapters.value = updated
      const inserted = updated.find((c) => c.startLine === lineNo)
      if (inserted) {
        selectedIds.value = [inserted.id]
        await loadPreview(inserted)
      }
      setStatus(`已在第 ${lineNo} 行插入章节`, 'ok')
    } catch (e) {
      setStatus(`插入章节失败：${errMsg(e)}`, 'error')
    } finally {
      busy.value = false
    }
  }

  async function convertSingle() {
    if (!book.value || busy.value) return
    busy.value = true
    setStatus('正在生成电子书...', 'busy')
    try {
      const res = await desktopApi.convertCurrent({
        title: title.value.trim(),
        author: author.value.trim(),
        outputPath: outputPath.value.trim(),
        coverImagePath: cover.value?.path ?? '',
        split: config.value.split,
        style: config.value.style,
        advanced: config.value.advanced,
        chapters: chapters.value,
      })
      lastResult.value = res
      await desktopApi.saveDesktopConfig(config.value).catch(() => {})
      const sec = (res.durationMs / 1000).toFixed(2)
      const mobiTip = res.mobiPath ? ' + MOBI' : ''
      setStatus(`生成完成（EPUB${mobiTip} · ${res.chapterCount} 章 · ${sec}s）`, 'ok')
    } catch (e) {
      setStatus(`生成失败：${errMsg(e)}`, 'error')
    } finally {
      busy.value = false
    }
  }

  async function runBatch() {
    if (!batchInputDir.value || busy.value) return
    busy.value = true
    setStatus('正在批量转换目录...', 'busy')
    try {
      const res = await desktopApi.runBatchConvert({
        inputDir: batchInputDir.value,
        outputDir: batchOutputDir.value,
        author: author.value.trim(),
        skipExisting: batchSkipExisting.value,
        jobs: batchJobs.value,
        enableMobi: config.value.advanced.enableMobi,
        fontIndex: config.value.style.fontIndex,
        configPath: config.value.configPath,
        ereadersPath: config.value.ereadersPath,
        advanced: config.value.advanced,
      })
      batchResult.value = res
      const sec = (res.durationMs / 1000).toFixed(2)
      setStatus(
        `批量完成：成功 ${res.succeeded} / 跳过 ${res.skipped} / 失败 ${res.failed}（${sec}s）`,
        res.failed > 0 ? 'error' : 'ok',
      )
    } catch (e) {
      setStatus(`批量转换失败：${errMsg(e)}`, 'error')
    } finally {
      busy.value = false
    }
  }

  async function saveConfig() {
    try {
      await desktopApi.saveDesktopConfig(config.value)
      setStatus(`配置已保存到 ${config.value.configPath || 'config.xml'}`, 'ok')
    } catch (e) {
      setStatus(`保存配置失败：${errMsg(e)}`, 'error')
    }
  }

  async function toggleAlwaysOnTop() {
    const next = !config.value.advanced.alwaysOnTop
    config.value.advanced.alwaysOnTop = next
    await desktopApi.setAlwaysOnTop(next)
    setStatus(next ? '已开启窗口置顶' : '已取消窗口置顶', 'idle')
  }

  return {
    version,
    mode,
    activeTab,
    previewOpen,
    previewMode,
    config,
    book,
    title,
    author,
    outputPath,
    cover,
    chapters,
    selectedIds,
    activeChapterId,
    preview,
    previewLoading,
    busy,
    statusText,
    statusTone,
    lastResult,
    batchInputDir,
    batchOutputDir,
    batchSkipExisting,
    batchJobs,
    batchResult,
    activeChapterCount,
    excludedCount,
    purgedCount,
    setStatus,
    init,
    openTxtPath,
    detectChapters,
    loadPreview,
    updateChaptersState,
    toggleChapterExcluded,
    renameChapter,
    insertChapter,
    convertSingle,
    runBatch,
    saveConfig,
    toggleAlwaysOnTop,
  }
})
