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

// 事件总线路由：每个事件名只向 Wails 注册一个监听，内部按订阅者分发。
// 规避 Wails EventsOff(name) 会移除该事件名全部监听、导致多订阅者互相误杀的问题。
type EventCallback = (...data: unknown[]) => void
const listeners = new Map<string, Set<EventCallback>>()

function subscribe(name: string, cb: EventCallback): () => void {
  let set = listeners.get(name)
  if (!set) {
    set = new Set()
    listeners.set(name, set)
  }
  if (set.size === 0) {
    window.runtime.EventsOn(name, (...data: unknown[]) => {
      listeners.get(name)?.forEach((fn) => fn(...data))
    })
  }
  set.add(cb)
  return () => {
    const s = listeners.get(name)
    if (!s) return
    s.delete(cb)
    if (s.size === 0) {
      listeners.delete(name)
      window.runtime.EventsOff(name)
    }
  }
}

// 订阅后端事件并只取首个参数，返回取消订阅函数。
export function onEvent<T>(name: string, cb: (payload: T) => void): () => void {
  return subscribe(name, (...data: unknown[]) => cb(data[0] as T))
}

// 订阅后端事件并透传全部参数（如 wails:file-drop 的 (x, y, paths)）。
export function onRawEvent(name: string, cb: EventCallback): () => void {
  return subscribe(name, cb)
}
