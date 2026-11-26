import type {
  BatchRequest,
  BatchResponse,
  ChapterItem,
  ChapterPreview,
  ConvertRequest,
  ConvertResponse,
  CoverInfo,
  DesktopConfig,
  LoadedBook,
  SplitConfig,
} from '@/types/desktop'

interface WailsAppBindings {
  GetVersion(): Promise<string>
  SetAlwaysOnTop(on: boolean): Promise<void>
  PickTxtFile(): Promise<string>
  PickCoverImage(): Promise<CoverInfo | null>
  LoadCoverPreview(path: string): Promise<CoverInfo | null>
  PickDirectory(title: string): Promise<string>
  PickConfigFile(): Promise<string>
  PickKindleGenExe(): Promise<string>
  OpenInExplorer(target: string): Promise<void>
  IsDirectory(path: string): Promise<boolean>
  LoadDesktopConfig(): Promise<DesktopConfig>
  LoadDesktopConfigFrom(cfgPath: string): Promise<DesktopConfig>
  SaveDesktopConfig(cfg: DesktopConfig): Promise<void>
  LoadTxtFile(path: string): Promise<LoadedBook>
  DetectChapters(cfg: SplitConfig): Promise<ChapterItem[]>
  GetChapterPreview(item: ChapterItem, cfg: SplitConfig): Promise<ChapterPreview>
  InsertChapterAtLine(
    existing: ChapterItem[],
    lineNo: number,
    title: string,
    cfg: SplitConfig,
  ): Promise<ChapterItem[]>
  ConvertCurrent(req: ConvertRequest): Promise<ConvertResponse>
  RunBatchConvert(req: BatchRequest): Promise<BatchResponse>
}

interface WailsRuntime {
  OnFileDrop(callback: (x: number, y: number, paths: string[]) => void, useDropTarget: boolean): void
  OnFileDropOff(): void
}

declare global {
  interface Window {
    go?: {
      main?: {
        App?: WailsAppBindings
      }
    }
    runtime?: WailsRuntime
  }
}

function getApp(): WailsAppBindings {
  const app = window.go?.main?.App
  if (!app) {
    throw new Error('未检测到 Wails 桌面运行时，请通过 EasyPub 桌面客户端启动')
  }
  return app
}

export const desktopApi = {
  getVersion: (): Promise<string> => getApp().GetVersion(),
  setAlwaysOnTop: (on: boolean): Promise<void> => getApp().SetAlwaysOnTop(on),
  pickTxtFile: (): Promise<string> => getApp().PickTxtFile(),
  pickCoverImage: (): Promise<CoverInfo | null> => getApp().PickCoverImage(),
  loadCoverPreview: (path: string): Promise<CoverInfo | null> => getApp().LoadCoverPreview(path),
  pickDirectory: (title: string): Promise<string> => getApp().PickDirectory(title),
  pickConfigFile: (): Promise<string> => getApp().PickConfigFile(),
  pickKindleGenExe: (): Promise<string> => getApp().PickKindleGenExe(),
  openInExplorer: (target: string): Promise<void> => getApp().OpenInExplorer(target),
  isDirectory: (path: string): Promise<boolean> => getApp().IsDirectory(path),
  loadDesktopConfig: (): Promise<DesktopConfig> => getApp().LoadDesktopConfig(),
  loadDesktopConfigFrom: (cfgPath: string): Promise<DesktopConfig> => getApp().LoadDesktopConfigFrom(cfgPath),
  saveDesktopConfig: (cfg: DesktopConfig): Promise<void> => getApp().SaveDesktopConfig(cfg),
  loadTxtFile: (path: string): Promise<LoadedBook> => getApp().LoadTxtFile(path),
  detectChapters: (cfg: SplitConfig): Promise<ChapterItem[]> => getApp().DetectChapters(cfg),
  getChapterPreview: (item: ChapterItem, cfg: SplitConfig): Promise<ChapterPreview> =>
    getApp().GetChapterPreview(item, cfg),
  insertChapterAtLine: (
    existing: ChapterItem[],
    lineNo: number,
    title: string,
    cfg: SplitConfig,
  ): Promise<ChapterItem[]> => getApp().InsertChapterAtLine(existing, lineNo, title, cfg),
  convertCurrent: (req: ConvertRequest): Promise<ConvertResponse> => getApp().ConvertCurrent(req),
  runBatchConvert: (req: BatchRequest): Promise<BatchResponse> => getApp().RunBatchConvert(req),
  onFileDrop: (cb: (paths: string[]) => void): void => {
    window.runtime?.OnFileDrop((_x, _y, paths) => cb(paths), false)
  },
  offFileDrop: (): void => {
    window.runtime?.OnFileDropOff()
  },
}
