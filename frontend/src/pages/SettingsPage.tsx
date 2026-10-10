import { useCallback, useEffect, useState } from 'react'
import {
  Button,
  MessageBar,
  MessageBarBody,
  ProgressBar,
  Toast,
  Toaster,
  ToastTitle,
  useId,
  useToastController,
} from '@fluentui/react-components'
import { ArrowSync24Regular, FolderOpen24Regular, Document24Regular } from '@fluentui/react-icons'

import { api, onEvent, Settings } from '../api'

type FFmpegStatus = 'checking' | 'ready' | 'missing'

export default function SettingsPage() {
  const [settings, setSettings] = useState<Settings>({ defaultOutputDir: '', ffmpegPath: '' })
  const [resolvedPath, setResolvedPath] = useState('')
  const [status, setStatus] = useState<FFmpegStatus>('checking')
  const [message, setMessage] = useState('')
  const [download, setDownload] = useState<{ done: number; total: number } | null>(null)

  const toasterId = useId('toaster')
  const { dispatchToast } = useToastController(toasterId)

  useEffect(() => {
    api.getSettings().then(setSettings).catch(() => {})
  }, [])

  const check = useCallback(async () => {
    setStatus('checking')
    setMessage('')
    try {
      const path = await api.ensureFFmpeg()
      setResolvedPath(path)
      setStatus('ready')
    } catch (e) {
      setStatus('missing')
      setMessage(errText(e))
    }
  }, [])

  useEffect(() => {
    check()
  }, [check])

  useEffect(
    () =>
      onEvent<{ done: number; total: number }>('ffmpeg:download', (p) =>
        setDownload(p.total > 0 && p.done >= p.total ? null : p),
      ),
    [],
  )

  async function chooseDir() {
    const dir = await api.selectDirectory()
    if (dir) setSettings((s) => ({ ...s, defaultOutputDir: dir }))
  }

  async function chooseFFmpeg() {
    const path = await api.selectFFmpeg()
    if (path) setSettings((s) => ({ ...s, ffmpegPath: path }))
  }

  async function save() {
    try {
      await api.saveSettings(settings)
      dispatchToast(
        <Toast>
          <ToastTitle>设置已保存</ToastTitle>
        </Toast>,
        { intent: 'success' },
      )
      if (settings.ffmpegPath) await check()
    } catch (e) {
      setMessage(errText(e))
    }
  }

  return (
    <>
      <Toaster toasterId={toasterId} />
      <h1 className="page-title">设置</h1>

      <section className="card">
        <div className="card-head">
          <h2>输出</h2>
        </div>
        <div className="field">
          <label>默认输出目录</label>
          <div className="hint" style={{ marginTop: 0, marginBottom: 8, wordBreak: 'break-all' }}>
            {settings.defaultOutputDir || '未设置（每次默认为源文件所在目录）'}
          </div>
          <div style={{ display: 'flex', gap: 8 }}>
            <Button icon={<FolderOpen24Regular />} onClick={chooseDir}>
              选择目录
            </Button>
            {settings.defaultOutputDir && (
              <Button appearance="subtle" onClick={() => setSettings((s) => ({ ...s, defaultOutputDir: '' }))}>
                清除
              </Button>
            )}
          </div>
        </div>
      </section>

      <section className="card">
        <div className="card-head">
          <h2>ffmpeg</h2>
        </div>

        {status === 'checking' && <div className="hint">正在检查 ffmpeg…</div>}
        {status === 'ready' && (
          <div className="kv">
            <span>状态</span>
            <span>就绪</span>
          </div>
        )}
        {status === 'ready' && (
          <div className="kv">
            <span>路径</span>
            <span>{resolvedPath}</span>
          </div>
        )}
        {status === 'missing' && (
          <MessageBar intent="warning">
            <MessageBarBody>{message || 'ffmpeg 未就绪'}</MessageBarBody>
          </MessageBar>
        )}

        {download && (
          <div style={{ marginTop: 12 }}>
            <div className="hint">
              正在下载 ffmpeg…{download.total > 0 ? ` ${Math.round((download.done / download.total) * 100)}%` : ''}
            </div>
            <ProgressBar value={download.total > 0 ? download.done / download.total : undefined} />
          </div>
        )}

        <div className="field" style={{ marginTop: 14 }}>
          <label>手动指定 ffmpeg 路径（可选）</label>
          <div className="hint" style={{ marginTop: 0, marginBottom: 8, wordBreak: 'break-all' }}>
            {settings.ffmpegPath || '自动：应用缓存 → 系统 PATH'}
          </div>
          <div style={{ display: 'flex', gap: 8 }}>
            <Button icon={<Document24Regular />} onClick={chooseFFmpeg}>
              选择 ffmpeg
            </Button>
            {settings.ffmpegPath && (
              <Button appearance="subtle" onClick={() => setSettings((s) => ({ ...s, ffmpegPath: '' }))}>
                清除
              </Button>
            )}
            <Button icon={<ArrowSync24Regular />} onClick={check} disabled={status === 'checking' || !!download}>
              重新检测 / 下载
            </Button>
          </div>
        </div>
      </section>

      <div className="actions">
        <Button appearance="primary" onClick={save}>
          保存设置
        </Button>
      </div>
    </>
  )
}

function errText(e: unknown): string {
  if (e instanceof Error) return e.message
  return String(e)
}
