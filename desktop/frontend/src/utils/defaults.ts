import type { DesktopConfig } from '@/types/desktop'

export function createDefaultConfig(): DesktopConfig {
  return {
    workDir: '',
    configPath: '',
    ereadersPath: '',
    split: {
      mode: 'auto',
      splitCount: 10000,
      fullReg: '',
      simpleRegP1: '[第卷]',
      simpleRegP2: 0,
      simpleRegP3: '[章回卷节集部]',
      simpleRegExt: '',
      simpleRegLeadingSpace: true,
      additionalReg: [],
      removeBlankLine: true,
      addSpace: false,
      addSpaceCount: 1,
      forceEmptyChapter: true,
    },
    style: {
      lineHeight: 120,
      fontSize: 100,
      marginTop: 5,
      indent: 0,
      textAlign: 0,
      fontIndex: 0,
    },
    advanced: {
      enableMobi: false,
      outputToSrc: true,
      outputFolder: '',
      kindleGenExe: 'kindlegen_v2.9.exe',
      kindleGenCompress: 1,
      kindleGenOption: '',
      alwaysOnTop: false,
    },
    presets: { fullRegs: [], additionalRegs: [] },
    ereaders: [],
  }
}
