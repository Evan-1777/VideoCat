// 与 Wails 后端绑定的类型化封装。运行期通过 window.go / window.runtime 注入，无需编译期生成代码。

export interface VideoMeta {
  path: string
  fps: number
  duration: number
  width: number
  height: number
  videoCodec: string
  hasAudio: boolean
  audioCodec: string
  sizeBytes: number
}

export type Resolution = 'source' | '480' | '720' | '1080'
export type AudioFormat = 'aac' | 'mp3' | 'flac'
export type Encoder = 'auto' | 'h264_nvenc' | 'h264_qsv' | 'h264_amf' | 'libx264'

export interface Params {
  input: string
  framesPerSecond: number
  resolution: Resolution
  outputFps: number
  extractAudio: boolean
  audioFormat: AudioFormat
  audioBitrate: string
  encoder: Encoder
  hardwareDecode: boolean
  crf: number
  outputDir: string
}

export interface Settings {
  defaultOutputDir: string
  ffmpegPath: string
}

export interface Capabilities {
  hwaccels: string[]
  hwEncoders: string[]
  hardwareDecode: boolean
}

export interface Outputs {
  video: string
  audio: string
  meta: string
}

export interface JobResult {
  outputs: Outputs
  encoder: string
  hardwareDecode: boolean
  outputDuration: number
  audioSkipped: boolean
}

interface AppBindings {
  Probe(path: string): Promise<VideoMeta>
  DefaultParams(path: string): Promise<Params>
  SelectVideo(): Promise<string>
  SelectDirectory(): Promise<string>
  SelectFFmpeg(): Promise<string>
  EnsureFFmpeg(): Promise<string>
  Capabilities(): Promise<Capabilities>
  StartJob(params: Params): Promise<void>
  CancelJob(): Promise<void>
  OpenOutputDir(path: string): Promise<void>
  GetSettings(): Promise<Settings>
  SaveSettings(s: Settings): Promise<void>
}

declare global {
  interface Window {
    go: { main: { App: AppBindings } }
    runtime: {
      EventsOn(name: string, callback: (...data: unknown[]) => void): void
      EventsOff(name: string): void
    }
  }
}

function bridge(): AppBindings {
  return window.go.main.App
}

export const api = {
  probe: (path: string) => bridge().Probe(path),
  defaultParams: (path: string) => bridge().DefaultParams(path),
  selectVideo: () => bridge().SelectVideo(),
  selectDirectory: () => bridge().SelectDirectory(),
  selectFFmpeg: () => bridge().SelectFFmpeg(),
  ensureFFmpeg: () => bridge().EnsureFFmpeg(),
  capabilities: () => bridge().Capabilities(),
  startJob: (p: Params) => bridge().StartJob(p),
  cancelJob: () => bridge().CancelJob(),
  openOutputDir: (path: string) => bridge().OpenOutputDir(path),
  getSettings: () => bridge().GetSettings(),
  saveSettings: (s: Settings) => bridge().SaveSettings(s),
}

// 订阅后端事件，返回取消订阅函数。
export function onEvent<T>(name: string, cb: (payload: T) => void): () => void {
  const handler = (...data: unknown[]) => cb(data[0] as T)
  window.runtime.EventsOn(name, handler)
  return () => window.runtime.EventsOff(name)
}
