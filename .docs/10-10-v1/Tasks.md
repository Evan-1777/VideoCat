# Tasks：video cat —— 长视频抽帧重渲染与音频分离工具

**关联 Plan**：`Plan.md` —— video cat v1.0
**状态**：DONE（2026-10-10）
**总计 Task**：16 个
**执行主体**：交付段主线（单一会话，不再派发子 agent）
**回归测试结论**：`go test ./...` 全通过（config / ffmpeg / probe / pipeline），`go vet ./...` 无告警，`frontend` `tsc && vite build` 通过。

> 目标目录结构（交付段据此落盘）：
>
> ```
> videocat/
> ├─ main.go            # Wails 入口，嵌入 frontend/dist
> ├─ app.go             # Wails 绑定方法（Probe / StartJob / Cancel / Settings）
> ├─ internal/
> │  ├─ ffmpeg/         # 定位、下载、能力探测、执行与进度
> │  ├─ probe/          # ffprobe 元数据
> │  ├─ pipeline/       # 命令构建与任务编排
> │  └─ config/         # 参数模型、校验、持久化
> ├─ frontend/          # React(TS) + Fluent UI React v9
> └─ build/             # Wails 打包资源
> ```

---

## Phase 1：项目骨架与 ffmpeg 基建

### TASK-001：初始化 Wails + React(TS) + Fluent UI v9 骨架

- **Status**：DONE
- **Executor**：主线
- **Description**：在仓库根建立 Wails v2 工程，Go 侧按 `main.go` / `app.go` / `internal/*` 分层，前端接入 React + TypeScript + Fluent UI React v9。
- **Details**：
  - `frontend/` 采用 Vite + React + TS，安装 `@fluentui/react-components`、`@fluentui/react-icons`
  - 根 `main.go` 用 `//go:embed frontend/dist` 嵌入前端产物；`app.go` 暴露占位绑定方法
  - `go.mod` 模块名、`wails.json` 基础字段就位
- **Acceptance Criteria**：
  - `go build ./...` 通过
  - `cd frontend && npm run build` 通过
  - `wails dev` 能启动窗口并渲染 FluentProvider 包裹的空白页

### TASK-002：实现 ffmpeg 运行时解析与自动下载

- **Status**：DONE
- **Executor**：主线
- **Description**：在 `internal/ffmpeg` 实现 ffmpeg/ffprobe 的定位、下载、校验与缓存。
- **Details**：
  - 定位顺序：用户指定路径 → 应用数据缓存目录 → 系统 PATH
  - 缓存目录：`%LOCALAPPDATA%\video cat\ffmpeg\`（Linux dev 用 `os.UserCacheDir()`）
  - 来源：BtbN/FFmpeg-Builds 的日期化 autobuild tag（不可变，如 `autobuild-2026-10-08-13-05`）；资产名内嵌提交号（分支构建形如 `ffmpeg-n9.0.2-23-g27b46f0fbc-win64-gpl-9.0.zip`，master 构建形如 `ffmpeg-N-127252-ga25ba44c0c-win64-gpl.zip`），实施时从所钉 release 的资产列表取准确名
  - 常量：`FFMPEG_RELEASE_TAG` 与 `FFMPEG_ASSET_NAME` 钉死 release 与文件名；升级即同步改这两个常量
  - 校验：下载同 release 的 `checksums.sha256`，按资产名查表比对（tag 供可复现性、checksum 供完整性）；不匹配、校验文件缺失或解压失败均返回可读错误
  - 下载过程通过回调上报进度
- **Acceptance Criteria**：
  - 缺少 ffmpeg 时触发下载、校验、缓存，返回可用路径
  - 二次运行直接复用缓存，不重复下载
  - 校验失败、校验文件缺失或网络失败时返回可读错误，不写入损坏文件
  - 指定路径存在且可执行时优先使用

### TASK-003：实现编解码能力探测

- **Status**：DONE
- **Executor**：主线
- **Description**：在 `internal/ffmpeg` 实现硬件编解码能力探测，产出有序编码器方案。
- **Details**：
  - 解析 `ffmpeg -hide_banner -hwaccels` 与 `ffmpeg -hide_banner -encoders`，识别 `nvenc` / `qsv` / `amf`
  - 自检走**代表性全链路**：硬解（`-hwaccel`）+ `select` + `setpts` + `scale` + 目标编码器，1 帧输入，确认该组合可用
  - 输出有序 `EncoderPlan`：优先 H.264（兼容性优先，利于 AI 消费），HEVC 备选，无可用硬件时回退 `libx264`
  - 探测结果缓存，避免每次任务重复探测
- **Acceptance Criteria**：
  - 返回有序编码器列表
  - 无 GPU 环境返回含 `libx264` 的回退方案
  - 自检失败的编码器或组合不出现在结果中
  - 自检覆盖「硬解 + 软件滤镜链」组合，探测通过即无需运行期错误分类回退
  - 探测结果可缓存复用

### TASK-004：实现 ffprobe 元数据读取

- **Status**：DONE
- **Executor**：主线
- **Description**：在 `internal/probe` 调用 `ffprobe -v error -show_format -show_streams -of json` 解析视频元数据。
- **Details**：
  - 解析为 `VideoMeta`：平均帧率 fps、时长、宽高、视频编解码、是否含音轨、音频编解码、文件大小
  - 非视频 / 无视频流的输入返回可读错误
  - fps 取 `avg_frame_rate`，非整数时按浮点保留
  - 测试样例：用 ffmpeg 现场生成固定参数样例（分数帧率 24000/1001、有音轨 / 无音轨各一），不入版本库；后续 TASK 的「样例 mp4」均指此约定
- **Acceptance Criteria**：
  - 对样例 mp4 返回正确的 fps / 时长 / 分辨率 / 音轨字段
  - 对非视频文件返回可读错误而非 panic
  - 单元测试覆盖 `avg_frame_rate` 分数解析（如 `24000/1001`）

---

## Phase 2：核心处理管线

### TASK-005：定义参数模型与校验

- **Status**：DONE
- **Executor**：主线
- **Description**：在 `internal/config` 定义任务参数模型、默认值、校验与应用设置持久化。
- **Details**：
  - `Params`：每秒保留帧数 m、输出分辨率（480/720/1080/原始）、输出帧率（默认跟随源）、音频开关、音频格式（aac/mp3/flac）、音频码率（仅 aac/mp3 生效）、视频编码器（auto/具体编码器）、硬件解码开关（控制 TASK-006 是否注入 `-hwaccel`）、质量（CRF）、输出目录
  - 输出目录：默认为源文件所在目录，可改；视频 / 音频 / meta.json 三个输出统一落该目录
  - 校验：`1 ≤ m ≤ floor(a)`（a 可能为分数帧率如 23.976）；分辨率、格式、编码器取自枚举；输出目录可写
  - 应用设置（默认输出目录、ffmpeg 路径）持久化到应用数据目录
- **Acceptance Criteria**：
  - 非法参数返回字段级错误信息
  - 默认值可用（720p、AAC、m 合理、编码器 auto）
  - 设置读写往返一致

### TASK-006：构建压缩视频重渲染命令

- **Status**：DONE
- **Executor**：主线
- **Description**：在 `internal/pipeline` 构建抽帧重渲染的 ffmpeg 参数。
- **Details**：
  - 抽帧：`select='lt(random(0), m/a)'`，随机保留概率 `p = m/a`
  - 重打包：`setpts=N/(a*TB)` 后以 `-r a` 输出，使保留帧连续
  - 缩放：`scale=-2:'min(ih,<目标高>)'`（720p 即 `min(ih,720)`，低于目标的源不放大；选「原始」则不缩放）
  - 硬件解码：开关开启且 TASK-003 全链路自检通过时注入 `-hwaccel auto`（Windows 上解析为 d3d11va / cuda / qsv），不设置 `-hwaccel_output_format`，保持帧回传系统内存以兼容软件滤镜；关闭或探测未通过时不注入
  - 编码：按 `EncoderPlan` 注入编码器与质量参数；`-an` 去除音轨
  - 输出：`<输出目录>/<源名>.cat.mp4`（输出目录默认为源目录），同名直接覆盖
  - 参数 `a` 取自 `VideoMeta`
- **Acceptance Criteria**：
  - 单元测试断言命令包含 `select`、`setpts`、`scale` 与目标编码器参数
  - 对样例 mp4 输出视频为 720p、无音轨、时长 ≈ `n·m/a`（容差 ±5%）
  - 低于目标分辨率的源不被放大
  - 硬解仅在 TASK-003 探测通过时注入（探测已覆盖解码 + 滤镜链组合）
  - 同输入连续两次运行，记录所选帧是否一致，用于确认 Plan §4「随机确定性」结论
  - 输出文件可被 ffprobe 正常解析

### TASK-007：构建音频提取命令

- **Status**：DONE
- **Executor**：主线
- **Description**：在 `internal/pipeline` 构建音频提取参数。
- **Details**：
  - `-vn -c:a <codec>`；码率 `-b:a` 仅对 aac / mp3 注入，flac 不注入；格式到编解码器 / 容器映射：aac→m4a、mp3→libmp3lame、flac→flac
  - 输出 `<输出目录>/<源名>.cat.<aext>`，与视频同目录，同名直接覆盖
  - 源无音轨时跳过并在结果中标记
- **Acceptance Criteria**：
  - 输出音频文件可被 ffprobe 解析，时长 ≈ 源（容差 ±2%）
  - 无音轨源不报错，任务标记音频跳过
  - 单元测试覆盖格式→编解码器映射，且断言 flac 命令不含 `-b:a`

### TASK-008：实现任务执行、进度解析与取消

- **Status**：DONE
- **Executor**：主线
- **Description**：在 `internal/pipeline` 实现 ffmpeg 进程执行、`-progress` 解析与取消。
- **Details**：
  - 以 `-progress pipe:1 -nostats` 运行，解析 `out_time_us` 与 `progress=end`
  - 单位陷阱：ffmpeg 的 `out_time_ms` 实测为微秒（与 `out_time_us` 同值），进度换算一律按微秒（`us / 1e6`），不按毫秒
  - 进度按阶段归一：每个 ffmpeg 阶段（抽帧重渲染 / 音频提取）各自 0..1，事件携带阶段名；前端按阶段切换展示
  - 基于 `context.Context` 取消：取消时终止进程
  - 保留 stderr 末尾用于错误诊断
- **Acceptance Criteria**：
  - 各阶段内进度单调不减并到达 1，阶段切换由 `job:phase` 事件驱动
  - 抽样核对中途进度值与 `ffmpeg` 自身日志一致，无 1000 倍量级偏差
  - 取消能在数秒内终止 ffmpeg 进程
  - ffmpeg 非零退出时返回含 stderr 摘要的错误

### TASK-009：实现 Wails 绑定 API 与进度事件

- **Status**：DONE
- **Executor**：主线
- **Description**：在 `app.go` 暴露后端能力，并通过 Wails 事件向前端推送状态。
- **Details**：
  - 绑定方法：`Probe(path)`、`EnsureFFmpeg()`、`StartJob(params)`、`CancelJob()`、`OpenOutputDir(path)`、`GetSettings()` / `SaveSettings()`
  - 事件：`ffmpeg:download`、`job:phase`（切换阶段）、`job:progress`（携带 `{phase, value}`，value 为该阶段内 0..1）、`job:done`、`job:error`
  - 单任务并发保护：同一时刻仅允许一个运行中任务
- **Acceptance Criteria**：
  - 前端可调用全部绑定方法并收到对应事件
  - 运行中再次 `StartJob` 被拒绝并返回可读错误
  - `job:done` 携带输出文件路径列表

### TASK-010：输出元数据 JSON

- **Status**：DONE
- **Executor**：主线
- **Description**：在 `internal/pipeline` 输出任务元数据 JSON。
- **Details**：
  - 内容：源元数据、生效参数、输出文件路径、源时长与输出时长、压缩比、各阶段耗时
  - 输出 `<输出目录>/<源名>.cat.meta.json`，同名直接覆盖
- **Acceptance Criteria**：
  - 生成合法 JSON 且字段齐全
  - 压缩比与输出时长数值与实测一致

---

## Phase 3：Fluent 图形界面

### TASK-011：搭建 Fluent 主题外壳与 Z 轴层级

- **Status**：DONE
- **Executor**：主线
- **Description**：在 `frontend/` 搭建 Fluent 应用外壳，按 `fluent-design` 令牌与 Layer 0–3 规划视觉层级。
- **Details**：
  - `FluentProvider` 包裹，接入 `webLightTheme` / `webDarkTheme`，品牌色 #0078D4
  - 布局：标题栏 + 左侧 `Nav`（转换 / 设置）+ 主内容区（单机单任务，不设任务历史页）
  - Layer 0 类 Mica 微渐变底色；Layer 1 内容卡片；Layer 2 侧栏 Acrylic + 1px 高光边框；Layer 3 抽屉 / 对话框深阴影 + 遮罩
  - 字体 Segoe UI，圆角 4–8px
- **Acceptance Criteria**：
  - 浅色 / 深色可切换且令牌正确
  - 各层的材质、阴影与模糊差异对齐 `temp/videocat-main.html`（以 Demo 为唯一视觉基准）

### TASK-012：实现主转换画布

- **Status**：DONE
- **Executor**：主线
- **Description**：在 `frontend/` 实现 P0 主转换画布，接通后端。
- **Details**：
  - 源视频：拖放区 + 选择按钮；载入后展示信息卡（文件名、时长、分辨率、帧率、大小）
  - 输出设置：每秒保留帧数 `Slider`（`max = floor(源帧率 a)`，`step = 1`，联动显示输出时长与压缩比）、输出分辨率 `Select`（默认 720p）、音频格式 `Select` + 提取音频 `Switch`
  - 操作：`开始处理` 主按钮；处理中展示阶段与 `ProgressBar` 及取消
  - 结果：输出视频 / 音频路径、压缩比、`打开输出目录`
- **Acceptance Criteria**：
  - 全流程可在界面内完成并调用真实后端
  - 帧数 `Slider` 上限等于 `floor(源帧率)`，无法拖出非法值
  - 抽帧参数变化时输出时长 / 压缩比实时正确
  - 空 / 加载 / 错误 / 成功四态完整

### TASK-013：实现高级参数抽屉与设置页

- **Status**：DONE
- **Executor**：主线
- **Description**：实现 P1 高级参数抽屉与 P2 设置页。
- **Details**：
  - 高级抽屉（Layer 3）：视频编码器 `Select`（自动 / NVENC / QSV / AMF / CPU x264）、硬件解码 `Switch`、质量 `Slider`、输出目录选择
  - 设置页：默认输出目录、ffmpeg 状态（就绪 / 未就绪）与重新下载、手动指定 ffmpeg 路径
  - 通过 `GetSettings` / `SaveSettings` 持久化
- **Acceptance Criteria**：
  - 抽屉可开合、带遮罩，未阻断底层上下文时主画布仍可见
  - 设置保存后重启仍生效
  - ffmpeg 未就绪状态可触发下载并显示进度

### TASK-014：状态反馈与取消闭环

- **Status**：DONE
- **Executor**：主线
- **Description**：完善运行时反馈与异常闭环。
- **Details**：
  - `ProgressBar` 按阶段展示；`MessageBar` 展示错误并给出重试；`Toaster` 提示完成
  - 取消按钮触发 `CancelJob` 并复位界面
  - ffmpeg 下载 / 探测过程中的中间态反馈
- **Acceptance Criteria**：
  - 错误可见且可重试，不泄露堆栈
  - 取消后界面回到空闲态且无残留任务
  - 完成态展示输出并可打开目录

---

## Phase 4：打包与平台验证

### TASK-015：Windows 构建配置与 CI 工作流

- **Status**：DONE
- **Executor**：主线
- **Description**：配置 Wails 打包与云端 CI，产出 Windows exe。
- **Details**：
  - 前置确认：仓库尚无 remote，CI 平台未定（暂按 GitHub Actions 假设）；落地前确认实际托管平台，若为其他平台（如 cnb.cool）则改写为对应流水线配置
  - `wails.json` 配置应用名、产物名、WebView2 策略
  - `build/windows/` 图标与清单资源
  - GitHub Actions：`windows-latest` + 安装 Go / Node / Wails CLI，执行 `wails build`
  - ffmpeg 在运行时下载，CI 不打包 ffmpeg
- **Acceptance Criteria**：
  - 实际 CI 平台已确认，工作流配置与之匹配
  - CI 成功产出可运行的 exe 产物
  - 产物在 Windows 11 上可启动且能触发 ffmpeg 下载

### TASK-016：端到端验证与 Project.md 同步

- **Status**：DONE
- **Executor**：主线
- **Description**：完成平台验证并同步文档。
- **Details**：
  - Linux：`go test ./...`、`cd frontend && npm run build`、`wails dev` 冒烟，后端管线对样例 mp4 跑通（需本地安装 ffmpeg）
  - Windows：手动验收清单——启动、ffmpeg 下载、抽帧重渲染时长与分辨率、音频提取、GPU 编码生效、硬解注入后滤镜链可用（由 TASK-003 探测判定）、取消
  - 按 `.docs/Project.example.md` 结构更新 `.docs/Project.md`
- **Acceptance Criteria**：
  - Go 单测与前端构建通过
  - Linux 管线端到端产出正确
  - Windows 验收清单逐项记录结论
  - `.docs/Project.md` 与代码库一致