# Project

> 项目的单一事实来源，供 Agent 动手前建立全局认知。定位与设计原则见 `SCOPE.md`，工作流规则见 `AGENTS.md`。

## 概述

- **定位**：把长视频抽帧重渲染为轻量短视频、并分离出压缩音频的 Windows 桌面工具（Wails + Go + React/Fluent）。
- **阶段**：首个版本交付完成
- **非目标**：视频剪辑 / 合成、内容理解与摘要、云端 AI 调用、ASR 转写、跨平台交付（详见 `SCOPE.md`）

## 环境与运行

- **交付平台**：Windows 11 x64（GUI exe）
- **开发环境**：Linux（WSL），仅用于后端逻辑与前端构建验证；本机禁止全量构建
- **运行时**：Go 1.25（模块名 `videocat`）；Node 22 / npm；Wails CLI v2.16.0
- **前端依赖**：React 19 + TypeScript 5 + Vite 7 + `@fluentui/react-components` / `@fluentui/react-icons`
- **依赖管理**：Go 模块（`go.mod`）；前端 `frontend/package.json`
- **构建**：`wails build`（云端 CI 产出 exe）；本机验证用 `go build ./...` + `cd frontend && npm run build`
- **运行**：`wails dev`（需本机具备 webkit2gtk/gtk3/WebView2 运行依赖）
- **测试**：`go test ./...`；依赖 ffmpeg 的用例在无 ffmpeg 时自动 skip（`internal/testutil.Tools`）
- ★ WSL 缺 `libgtk-3-dev` / `libwebkit2gtk`，`wails dev` 无法开窗；开发期改用「Go 单测 + Vite 前端构建」双轨验证
- ★ ffmpeg 不随包分发，运行时下载并缓存到应用数据目录

## 结构

```
videocat/
├─ main.go            # Wails 入口，嵌入 frontend/dist，注册拖放
├─ app.go             # Wails 绑定方法（Probe/StartJob/Cancel/Settings/对话框）与事件推送
├─ internal/
│  ├─ ffmpeg/         # 路径定位、下载校验缓存、能力探测、进程执行与进度
│  ├─ probe/          # ffprobe 元数据解析
│  ├─ pipeline/       # 命令构建、任务编排、元数据 JSON 输出
│  ├─ config/         # 参数模型、校验、设置持久化、数据目录
│  └─ testutil/       # 测试样例生成（仅测试引用）
├─ frontend/          # React(TS) + Fluent UI v9
│  └─ src/{App.tsx,pages,components,lib,api.ts,theme.ts,styles.css}
├─ build/windows/     # Wails 打包资源（图标 / 清单 / 安装器脚本）
└─ .github/workflows/ # Windows 构建流水线
```

- **数据流**：前端 `api.ts` → Wails 绑定（`app.go`）→ `probe` 读源元数据 → `pipeline.Runner` 构建并执行 ffmpeg（`internal/ffmpeg`）→ 输出视频 / 音频 / meta.json；进度经 `job:phase` / `job:progress` / `job:done` / `job:error` 事件回推前端。
- **依赖方向**：`app → pipeline → probe/ffmpeg`，`config` 为共享层；`ffmpeg` 依赖 `config`（数据目录），其余不反向依赖。

## 约定

- **命名**：Go 导出符号 PascalCase，内部 camelCase；前端组件 PascalCase、其余 camelCase
- **语言**：用户可见文本用中文；代码标识符用英文；注释按需
- **错误处理**：`config` 用字段级 `Fields`（map）承载校验错误；`pipeline` 冻结 ffmpeg stderr 尾部拼入错误；前端经 `MessageBar` 展示并支持重试
- **提交**：Conventional Commits 英文类型前缀 + 中文描述

## 约束与决策

- 2026-10-10 处理引擎用 ffmpeg/ffprobe 子进程，不用 cgo 绑定——成熟稳定、原生支持全格式与 GPU 编解码、免交叉编译
- 2026-10-10 抽帧策略：每秒随机保留 `m` 帧（概率 `p = m/a`），`setpts=N/(a*TB)` 重打包、输出 `-r a`——避免固定相位运动混叠；实测 `random()` 对同一输入**确定性**（同输入两次运行所选帧一致）
- 2026-10-10 输出命名固定后缀 `<源名>.cat.mp4` / `.cat.<aext>` / `.cat.meta.json`，同名直接覆盖——可机械判定
- 2026-10-10 ffmpeg 来源钉死不可变 release：tag `autobuild-2026-10-10-13-04`、资产 `ffmpeg-n9.0.2-25-g67b60c310b-win64-gpl-9.0.zip`、SHA-256 `1b196e14bd13b76c0d33411f8ef4511f8708a459f682b58ca4c0285a793138cf`；升级即改 `internal/ffmpeg/download.go` 三个常量
- 2026-10-10 硬解仅在能力探测的「硬解 + 软件滤镜链 + 编码」全链路自检通过时才注入 `-hwaccel auto`，且不设 `-hwaccel_output_format`——保证帧回传系统内存以兼容软件滤镜
- 2026-10-10 编码器优先序：H.264（`h264_nvenc → h264_qsv → h264_amf`）→ HEVC 备选 → `libx264` 回退——兼容性优先，利于 AI 消费
- 2026-10-10 质量参数映射（`ffmpeg.QualityArgs`）由能力探测自检与正式任务共用——使探测覆盖真实编码参数组合（含 AMF 的 `-qp_i/-qp_p`）
- 2026-10-10 meta.json 与结果卡的输出时长 / 压缩比取渲染后 ffprobe **实测值**（`outputDurationMeasured` 标记），不用抽帧理论值——随机采样下二者不等
- 2026-10-10 缩放高度取偶数：`scale=-2:'trunc(min(ih,H)/2)*2'`——源高为奇数时可兼容 yuv420p 与硬件编码器
- 2026-10-10 数字陷阱：ffmpeg `out_time_ms` 实测与 `out_time_us` 同值（均为微秒），进度一律按微秒换算
- 2026-10-10 应用数据目录取 `os.UserCacheDir()`（Windows 为 `%LOCALAPPDATA%\video cat`），ffmpeg 缓存与 `settings.json` 同处其下
- 2026-10-10 Windows 产物由 GitHub Actions（`windows-latest`）构建，CI 不打包 ffmpeg
- 2026-10-10 公共仓库 `Evan-1777/VideoCat`（https://github.com/Evan-1777/VideoCat），remote origin 为 https 协议；工作流对 push master / tag `v*` / 手动触发均自动运行单测与构建，push 触发的首次运行已验证通过（run 38062962931）

## 外部集成

- **BtbN/FFmpeg-Builds**：运行时下载 ffmpeg/ffprobe（Windows），按固定 tag 取资产并用同 release 的 `checksums.sha256` 校验完整性。
- **GitHub Actions**：`.github/workflows/build-windows.yml` 对 push master / tag `v*` / 手动触发均构建并上传 exe 产物；首次运行已验证通过。

## 术语

- **cat 后缀** = 输出文件命名后缀（`.cat.`），用于标示「经本工具压缩预处理」的产物
- **抽帧** = 按概率 `p = m/a` 随机保留源帧，再以源帧率重打包
- **EncoderPlan** = 能力探测产出的有序编码器方案（硬件优先，`libx264` 回退）
- **阶段（phase）** = 任务内的 ffmpeg 子步骤：`render`（抽帧重渲染）、`audio`（音频提取），各自进度归一为 0..1

## Windows 手动验收清单

> 交付平台为 Windows，需在 Windows 11 上逐项验证并记录结论（本机 WSL 不具备 GPU 与开窗条件）。

| # | 项 | 结论 |
|---|----|------|
| 1 | 启动 exe，窗口正常渲染 Fluent 界面 | 待验收 |
| 2 | 首次启动触发 ffmpeg 下载并显示进度；二次启动复用缓存不重复下载 | 待验收 |
| 3 | 选择样例 mp4，信息卡 fps / 时长 / 分辨率 / 大小正确 | 待验收 |
| 4 | 每秒保留帧数 Slider 上限 = `floor(源 fps)`，输出时长 / 压缩比实时联动 | 待验收 |
| 5 | 开始处理，进度按阶段推进；取消可终止 | 待验收 |
| 6 | 输出视频为目标分辨率、无音轨、时长 ≈ `n·m/a`（±5%），可被 ffprobe 解析 | 待验收 |
| 7 | 音频文件时长 ≈ 源（±2%），容器 / 编码符合所选格式 | 待验收 |
| 8 | GPU 编码生效（NVENC / QSV / AMF 在能力提示中出现并用于输出） | 待验收 |
| 9 | 硬解注入后滤镜链可用（TASK-003 探测通过才注入） | 待验收 |
| 10 | `meta.json` 字段齐全、数值与实测一致 | 待验收 |
| 11 | 设置页保存默认输出目录 / ffmpeg 路径后，重启仍生效 | 待验收 |
| 12 | 完成后「打开输出目录」正确定位 | 待验收 |

## 维护

| 变更 | 同步章节 |
|---|---|
| 运行、构建、测试方式或环境前提变化 | 环境与运行 |
| 依赖增删或升级 | 环境与运行；关键选型同步「约束与决策」 |
| 模块增删、重构，依赖方向变化 | 结构 |
| 命名、语言、提交规范调整 | 约定 |
| 新坑、新约束、选型定型 | 约束与决策 |
| 外部服务接入或下线 | 外部集成 |
| 新增项目特有名词 | 术语 |
