import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Button,
  MessageBar,
  MessageBarActions,
  MessageBarBody,
  ProgressBar,
  Select,
  Slider,
  Switch,
  Toast,
  Toaster,
  ToastTitle,
  useId,
  useToastController,
} from '@fluentui/react-components'
import {
  ArrowUpload24Regular,
  Dismiss24Regular,
  FolderOpen24Regular,
  VideoClip24Regular,
} from '@fluentui/react-icons'

import { api, Capabilities, JobResult, onEvent, Params, Settings, VideoMeta } from '../api'
import { baseName, dirName, estimate, formatBytes, formatDuration, formatRatio } from '../lib/format'
import AdvancedDrawer from '../components/AdvancedDrawer'

type Status = 'idle' | 'processing' | 'done' | 'error'
type Phase = 'render' | 'audio'

export default function ConvertPage() {
  const [settings, setSettings] = useState<Settings | null>(null)
  const [caps, setCaps] = useState<Capabilities | null>(null)
  const [ffmpegError, setFfmpegError] = useState('')
  const [download, setDownload] = useState<{ done: number; total: number } | null>(null)

  const [source, setSource] = useState<VideoMeta | null>(null)
  const [params, setParams] = useState<Params | null>(null)
  const [loadingSource, setLoadingSource] = useState(false)

  const [status, setStatus] = useState<Status>('idle')
  const [phase, setPhase] = useState<Phase | null>(null)
  const [progress, setProgress] = useState<Record<Phase, number>>({ render: 0, audio: 0 })
  const [error, setError] = useState('')
  const [result, setResult] = useState<JobResult | null>(null)
  const [drawerOpen, setDrawerOpen] = useState(false)

  const toasterId = useId('toaster')
  const { dispatchToast } = useToastController(toasterId)

  const settingsRef = useRef<Settings | null>(null)
  settingsRef.current = settings

  const pickSource = useCallback(async (path: string) => {
    setLoadingSource(true)
    setError('')
    setStatus('idle')
    setResult(null)
    try {
      const meta = await api.probe(path)
      const p = await api.defaultParams(path)
      const maxM = Math.max(1, Math.floor(meta.fps))
      if (p.framesPerSecond > maxM) p.framesPerSecond = maxM
      if (settingsRef.current?.defaultOutputDir) p.outputDir = settingsRef.current.defaultOutputDir
      setSource(meta)
      setParams(p)
    } catch (e) {
      setError(errText(e))
      setStatus('error')
    } finally {
      setLoadingSource(false)
    }
  }, [])

  // 初始化：加载设置与能力（首次可能触发 ffmpeg 下载）。
  useEffect(() => {
    let alive = true
    api
      .getSettings()
      .then((s) => alive && setSettings(s))
      .catch(() => {})
    api
      .capabilities()
      .then((c) => alive && (setCaps(c), setFfmpegError('')))
      .catch((e) => alive && setFfmpegError(errText(e)))
    return () => {
      alive = false
    }
  }, [])

  // 订阅后端事件。
  useEffect(() => {
    const offs = [
      onEvent<{ phase: Phase }>('job:phase', (p) => setPhase(p.phase)),
      onEvent<{ phase: Phase; value: number }>('job:progress', (p) =>
        setProgress((prev) => ({ ...prev, [p.phase]: p.value })),
      ),
      onEvent<JobResult>('job:done', (r) => {
        setResult(r)
        setStatus('done')
        setPhase(null)
        dispatchToast(
          <Toast>
            <ToastTitle>处理完成</ToastTitle>
          </Toast>,
          { intent: 'success' },
        )
      }),
      onEvent<{ message: string }>('job:error', (p) => {
        setError(p.message)
        setStatus(p.message === '任务已取消' ? 'idle' : 'error')
        setPhase(null)
      }),
      onEvent<{ done: number; total: number }>('ffmpeg:download', (p) => {
        setDownload(p.total > 0 && p.done >= p.total ? null : p)
      }),
    ]
    const onDrop = (...args: unknown[]) => {
      const paths = args[2] as string[] | undefined
      if (paths && paths.length > 0) pickSource(paths[0])
    }
    window.runtime.EventsOn('wails:file-drop', onDrop)
    return () => {
      offs.forEach((off) => typeof off === 'function' && off())
      window.runtime.EventsOff('wails:file-drop')
    }
  }, [pickSource, dispatchToast])

  const maxM = source ? Math.max(1, Math.floor(source.fps)) : 1
  const est = useMemo(
    () => (source && params ? estimate(source.duration, source.fps, params.framesPerSecond) : { outSeconds: 0, ratio: 0 }),
    [source, params],
  )

  async function chooseVideo() {
    try {
      const path = await api.selectVideo()
      if (path) await pickSource(path)
    } catch (e) {
      setError(errText(e))
    }
  }

  async function start() {
    if (!params) return
    setError('')
    setProgress({ render: 0, audio: 0 })
    setPhase('render')
    setStatus('processing')
    try {
      await api.startJob(params)
    } catch (e) {
      setError(errText(e))
      setStatus('error')
      setPhase(null)
    }
  }

  async function cancel() {
    try {
      await api.cancelJob()
    } catch {
      /* 忽略取消错误 */
    }
  }

  function patch(p: Partial<Params>) {
    setParams((prev) => (prev ? { ...prev, ...p } : prev))
  }

  const shownSource = status === 'processing' || status === 'done'

  return (
    <>
      <Toaster toasterId={toasterId} />
      <h1 className="page-title">转换</h1>

      {ffmpegError && (
        <MessageBar intent="warning">
          <MessageBarBody>{ffmpegError}（可在设置中重新下载或指定路径）</MessageBarBody>
        </MessageBar>
      )}
      {download && (
        <div style={{ marginBottom: 16 }}>
          <div className="hint">正在下载 ffmpeg…{download.total > 0 ? ` ${Math.round((download.done / download.total) * 100)}%` : ''}</div>
          <ProgressBar value={download.total > 0 ? download.done / download.total : undefined} />
        </div>
      )}

      {status === 'error' && (
        <MessageBar intent="error">
          <MessageBarBody>{error || '处理失败'}</MessageBarBody>
          <MessageBarActions
            containerAction={<Button appearance="transparent" icon={<Dismiss24Regular />} aria-label="关闭" onClick={() => setStatus('idle')} />}
          >
            {params && <Button appearance="subtle" onClick={start}>重试</Button>}
          </MessageBarActions>
        </MessageBar>
      )}

      <section className="card">
        <div className="card-head">
          <h2>源视频</h2>
        </div>

        {!source ? (
          <div className="drop">
            <ArrowUpload24Regular style={{ fontSize: 26, color: 'var(--brand)' }} />
            <div>拖放视频文件到此处</div>
            <Button appearance="primary" onClick={chooseVideo} disabled={loadingSource}>
              {loadingSource ? '载入中…' : '选择视频'}
            </Button>
          </div>
        ) : (
          <div className="file-row">
            <span className="file-icon" aria-hidden="true">
              <VideoClip24Regular />
            </span>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div className="file-name">{baseName(source.path)}</div>
              <div className="tags">
                <span className="tag">时长 {formatDuration(source.duration)}</span>
                <span className="tag">
                  {source.width}×{source.height}
                </span>
                <span className="tag">{source.fps.toFixed(2)} fps</span>
                <span className="tag">{formatBytes(source.sizeBytes)}</span>
                {!source.hasAudio && <span className="tag">无音轨</span>}
              </div>
            </div>
            <Button appearance="subtle" onClick={chooseVideo} disabled={shownSource}>
              更换
            </Button>
          </div>
        )}
      </section>

      {source && params && (status === 'idle' || status === 'error') && (
        <>
          <section className="card">
            <div className="card-head">
              <h2>输出设置</h2>
            </div>

            <div className="field">
              <label htmlFor="mSlider">每秒保留帧数</label>
              <Slider
                id="mSlider"
                min={1}
                max={maxM}
                step={1}
                value={params.framesPerSecond}
                onChange={(_, data) => patch({ framesPerSecond: data.value })}
              />
              <div className="hint">
                输出时长约 <b>{formatDuration(est.outSeconds)}</b> · 压缩至 <b>{formatRatio(est.ratio)}</b> · 源{' '}
                {source.fps.toFixed(2)} fps
              </div>
            </div>

            <div className="grid-2">
              <div className="field">
                <label htmlFor="res">输出分辨率</label>
                <Select
                  id="res"
                  value={params.resolution}
                  onChange={(_, data) => patch({ resolution: data.value as Params['resolution'] })}
                >
                  <option value="source">原始</option>
                  <option value="1080">1080p</option>
                  <option value="720">720p</option>
                  <option value="480">480p</option>
                </Select>
              </div>
              <div className="field">
                <label htmlFor="afmt">音频格式</label>
                <Select
                  id="afmt"
                  value={params.audioFormat}
                  disabled={!params.extractAudio || !source.hasAudio}
                  onChange={(_, data) => patch({ audioFormat: data.value as Params['audioFormat'] })}
                >
                  <option value="aac">AAC（.m4a）</option>
                  <option value="mp3">MP3</option>
                  <option value="flac">FLAC</option>
                </Select>
              </div>
            </div>

            <div className="field">
              <Switch
                label="提取音频"
                checked={params.extractAudio}
                disabled={!source.hasAudio}
                onChange={(_, data) => patch({ extractAudio: data.checked })}
              />
              {!source.hasAudio && <div className="hint">源视频不含音轨，将跳过音频提取。</div>}
            </div>

            <Button appearance="subtle" onClick={() => setDrawerOpen(true)}>
              高级参数
            </Button>
          </section>

          <div className="actions">
            <Button appearance="primary" onClick={start}>
              开始处理
            </Button>
          </div>
        </>
      )}

      {status === 'processing' && (
        <section className="card">
          <div className="card-head">
            <h2>处理中</h2>
          </div>
          <div className={`phase ${progress.render >= 1 && phase !== 'render' ? 'done' : phase === 'render' ? 'active' : ''}`}>
            <span className="dot" />
            抽帧重渲染{progress.render >= 1 && phase !== 'render' ? ' · 完成' : ''}
          </div>
          {params?.extractAudio && source?.hasAudio && (
            <div className={`phase ${phase === 'audio' ? 'active' : ''}`}>
              <span className="dot" />
              提取音频{phase === 'audio' && progress.audio >= 1 ? ' · 完成' : ''}
            </div>
          )}
          <div style={{ margin: '12px 0' }}>
            <ProgressBar value={phase ? progress[phase] : undefined} />
          </div>
          <div className="actions" style={{ margin: 0 }}>
            <Button onClick={cancel}>取消</Button>
          </div>
        </section>
      )}

      {status === 'done' && result && (
        <section className="card">
          <div className="card-head">
            <h2>输出文件</h2>
          </div>
          <div className="kv">
            <span>重渲染视频</span>
            <span>{baseName(result.outputs.video)}</span>
          </div>
          {!result.audioSkipped && result.outputs.audio && (
            <div className="kv">
              <span>音频</span>
              <span>{baseName(result.outputs.audio)}</span>
            </div>
          )}
          {result.audioSkipped && (
            <div className="kv">
              <span>音频</span>
              <span>已跳过</span>
            </div>
          )}
          <div className="kv">
            <span>输出时长</span>
            <span>{formatDuration(result.outputDuration)}</span>
          </div>
          <div className="kv">
            <span>压缩比</span>
            <span>{formatRatio(source && result.outputDuration > 0 ? source.duration / result.outputDuration : 0)}</span>
          </div>
          <div className="kv">
            <span>编码器</span>
            <span>
              {result.encoder}
              {result.hardwareDecode ? ' · 硬解' : ''}
            </span>
          </div>
          <div className="actions" style={{ marginTop: 14, marginBottom: 0 }}>
            <Button icon={<FolderOpen24Regular />} onClick={() => api.openOutputDir(dirName(result.outputs.video))}>
              打开输出目录
            </Button>
            <Button
              appearance="subtle"
              onClick={() => {
                setStatus('idle')
                setResult(null)
              }}
            >
              继续处理
            </Button>
          </div>
        </section>
      )}

      {source && params && (
        <AdvancedDrawer
          open={drawerOpen}
          onClose={() => setDrawerOpen(false)}
          params={params}
          patch={patch}
          caps={caps}
        />
      )}
    </>
  )
}

function errText(e: unknown): string {
  if (e instanceof Error) return e.message
  return String(e)
}
