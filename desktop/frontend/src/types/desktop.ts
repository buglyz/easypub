export type SplitMode = 'auto' | 'simple' | 'full' | 'count' | 'whole'
export type ChapterState = 'normal' | 'excluded' | 'purged'
export type WorkspaceMode = 'single' | 'batch'
export type ConfigTab = 'book' | 'split' | 'style' | 'advanced'
export type PreviewMode = 'reader' | 'raw'

export interface SplitConfig {
  mode: SplitMode
  splitCount: number
  fullReg: string
  simpleRegP1: string
  simpleRegP2: number
  simpleRegP3: string
  simpleRegExt: string
  simpleRegLeadingSpace: boolean
  additionalReg: string[]
  removeBlankLine: boolean
  addSpace: boolean
  addSpaceCount: number
  forceEmptyChapter: boolean
}

export interface StyleConfig {
  lineHeight: number
  fontSize: number
  marginTop: number
  indent: number
  textAlign: number
  fontIndex: number
}

export interface AdvancedConfig {
  enableMobi: boolean
  outputToSrc: boolean
  outputFolder: string
  kindleGenExe: string
  kindleGenCompress: number
  kindleGenOption: string
  alwaysOnTop: boolean
}

export interface EReaderOption {
  index: number
  name: string
  fonts: string[]
}

export interface PresetConfig {
  fullRegs: string[]
  additionalRegs: string[]
}

export interface DesktopConfig {
  workDir: string
  configPath: string
  ereadersPath: string
  split: SplitConfig
  style: StyleConfig
  advanced: AdvancedConfig
  presets: PresetConfig
  ereaders: EReaderOption[]
}

export interface LoadedBook {
  path: string
  fileName: string
  sizeBytes: number
  encoding: string
  totalLines: number
  totalChars: number
  inferredTitle: string
}

export interface ChapterItem {
  id: number
  title: string
  startLine: number
  endLine: number
  lineCount: number
  charCount: number
  state: ChapterState
  custom: boolean
}

export interface ChapterPreview {
  title: string
  startLine: number
  endLine: number
  charCount: number
  paragraphs: string[]
  rawLines: string[]
  truncated: boolean
}

export interface CoverInfo {
  path: string
  name: string
  dataUrl: string
}

export interface ConvertRequest {
  title: string
  author: string
  outputPath: string
  coverImagePath: string
  split: SplitConfig
  style: StyleConfig
  advanced: AdvancedConfig
  chapters: ChapterItem[]
}

export interface ConvertResponse {
  epubPath: string
  mobiPath: string
  chapterCount: number
  encoding: string
  durationMs: number
}

export interface BatchRequest {
  inputDir: string
  outputDir: string
  author: string
  skipExisting: boolean
  jobs: number
  enableMobi: boolean
  fontIndex: number
  configPath: string
  ereadersPath: string
  advanced: AdvancedConfig
}

export interface BatchItemResult {
  input: string
  fileName: string
  output: string
  encoding: string
  chapters: number
  status: 'ok' | 'skipped' | 'failed'
  error: string
}

export interface BatchResponse {
  outputDir: string
  total: number
  succeeded: number
  skipped: number
  failed: number
  durationMs: number
  items: BatchItemResult[]
}
