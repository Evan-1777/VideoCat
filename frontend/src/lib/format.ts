// 展示层格式化与估算工具。

export function formatDuration(seconds: number): string {
  if (!isFinite(seconds) || seconds <= 0) return '0:00'
  const m = Math.floor(seconds / 60)
  const s = Math.round(seconds % 60)
  return `${m}:${String(s).padStart(2, '0')}`
}

export function formatBytes(n: number): string {
  if (!n || n <= 0) return '—'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let v = n
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v >= 10 || i === 0 ? Math.round(v) : v.toFixed(1)} ${units[i]}`
}

export function baseName(path: string): string {
  const parts = path.split(/[\\/]/)
  return parts[parts.length - 1] || path
}

export function dirName(path: string): string {
  const idx = Math.max(path.lastIndexOf('\\'), path.lastIndexOf('/'))
  return idx > 0 ? path.slice(0, idx) : path
}

// 输出时长 ≈ 源时长 × m / 源帧率；压缩比 ≈ 源帧率 / m。
export function estimate(srcDuration: number, srcFps: number, m: number) {
  if (srcFps <= 0 || m <= 0) return { outSeconds: 0, ratio: 0 }
  return {
    outSeconds: (srcDuration * m) / srcFps,
    ratio: srcFps / m,
  }
}

export function formatRatio(ratio: number): string {
  if (ratio <= 0) return '—'
  return `1 / ${Number.isInteger(ratio) ? ratio : ratio.toFixed(1)}`
}
