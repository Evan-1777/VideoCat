import { Button, Select, Slider, Switch } from '@fluentui/react-components'
import { Dismiss24Regular, FolderOpen24Regular } from '@fluentui/react-icons'

import { api, Capabilities, Encoder, Params } from '../api'

interface Props {
  open: boolean
  onClose: () => void
  params: Params
  patch: (p: Partial<Params>) => void
  caps: Capabilities | null
}

const encoderOptions: { value: Encoder; label: string }[] = [
  { value: 'auto', label: '自动（优先硬件）' },
  { value: 'h264_nvenc', label: 'NVIDIA NVENC' },
  { value: 'h264_qsv', label: 'Intel QSV' },
  { value: 'h264_amf', label: 'AMD AMF' },
  { value: 'libx264', label: 'CPU · x264' },
]

function encoderHint(caps: Capabilities | null): string {
  if (!caps) return '正在探测编码能力…'
  if (caps.hwEncoders.length === 0) return '未检测到可用的硬件编码器，将使用 CPU（libx264）。'
  return `已检测到：${caps.hwEncoders.join('、')}${caps.hardwareDecode ? '，硬解可用' : '，硬解不可用'}`
}

export default function AdvancedDrawer({ open, onClose, params, patch, caps }: Props) {
  async function chooseDir() {
    try {
      const dir = await api.selectDirectory()
      if (dir) patch({ outputDir: dir })
    } catch {
      /* 忽略 */
    }
  }

  return (
    <>
      <div className={`overlay ${open ? 'open' : ''}`} onClick={onClose} />
      <aside className={`drawer ${open ? 'open' : ''}`} aria-hidden={!open}>
        <div className="drawer-head">
          <h2>高级参数</h2>
          <span className="spacer" />
          <Button appearance="subtle" icon={<Dismiss24Regular />} aria-label="关闭" onClick={onClose} />
        </div>

        <div className="drawer-body">
          <div className="field">
            <label htmlFor="enc">视频编码器</label>
            <Select
              id="enc"
              value={params.encoder}
              onChange={(_, data) => patch({ encoder: data.value as Encoder })}
            >
              {encoderOptions.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </Select>
            <div className="hint">{encoderHint(caps)}</div>
          </div>

          <div className="field">
            <Switch
              label="硬件解码"
              checked={params.hardwareDecode && !!caps?.hardwareDecode}
              disabled={!caps?.hardwareDecode}
              onChange={(_, data) => patch({ hardwareDecode: data.checked })}
            />
            {!caps?.hardwareDecode && <div className="hint">当前环境未通过硬解自检，已使用软件解码。</div>}
          </div>

          <div className="field">
            <label htmlFor="crf">质量（CRF 18–28，越低越清晰）</label>
            <Slider id="crf" min={18} max={28} step={1} value={params.crf} onChange={(_, data) => patch({ crf: data.value })} />
            <div className="hint">CRF {params.crf} · {params.crf <= 20 ? '高' : params.crf <= 24 ? '均衡' : '小体积'}</div>
          </div>

          <div className="field">
            <label htmlFor="outdir">输出目录</label>
            <div className="hint" style={{ marginTop: 0, marginBottom: 8, wordBreak: 'break-all' }}>
              {params.outputDir || '默认：源文件所在目录'}
            </div>
            <Button icon={<FolderOpen24Regular />} onClick={chooseDir}>
              选择目录
            </Button>
          </div>
        </div>

        <div className="drawer-foot">
          <Button onClick={() => patch({ encoder: 'auto', hardwareDecode: true, crf: 23, outputDir: '' })}>重置</Button>
          <Button appearance="primary" onClick={onClose}>
            保存
          </Button>
        </div>
      </aside>
    </>
  )
}
