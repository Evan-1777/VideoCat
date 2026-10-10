package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"sync"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"videocat/internal/config"
	"videocat/internal/ffmpeg"
	"videocat/internal/pipeline"
	"videocat/internal/probe"
)

// App 是暴露给前端的后端对象。
type App struct {
	ctx      context.Context
	detector *ffmpeg.Detector

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	tools   *ffmpeg.Tools
}

// NewApp 创建 App。
func NewApp() *App {
	return &App{detector: ffmpeg.NewDetector()}
}

// startup 保存 Wails 运行期上下文。
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// Probe 读取源视频元数据。
func (a *App) Probe(path string) (probe.VideoMeta, error) {
	tools, err := a.ensureTools(a.ctx)
	if err != nil {
		return probe.VideoMeta{}, err
	}
	return probe.Probe(a.ctx, tools.FFprobe, path)
}

// DefaultParams 返回给定输入的默认参数。
func (a *App) DefaultParams(path string) config.Params {
	return config.DefaultParams(path)
}

// SelectVideo 打开系统文件对话框选择视频文件，返回所选路径（取消时为空串）。
func (a *App) SelectVideo() (string, error) {
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择视频文件",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "视频文件", Pattern: "*.mp4;*.mkv;*.mov;*.avi;*.webm;*.flv;*.m4v;*.ts"},
			{DisplayName: "所有文件", Pattern: "*.*"},
		},
	})
}

// SelectDirectory 打开目录选择对话框，返回所选目录（取消时为空串）。
func (a *App) SelectDirectory() (string, error) {
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{Title: "选择输出目录"})
}

// SelectFFmpeg 打开文件对话框选择 ffmpeg 可执行文件，返回所选路径（取消时为空串）。
func (a *App) SelectFFmpeg() (string, error) {
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择 ffmpeg 可执行文件",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "可执行文件", Pattern: "*.exe;ffmpeg"},
			{DisplayName: "所有文件", Pattern: "*.*"},
		},
	})
}

// EnsureFFmpeg 确保 ffmpeg 就绪，返回 ffmpeg 可执行文件路径；下载过程通过 ffmpeg:download 事件上报。
func (a *App) EnsureFFmpeg() (string, error) {
	tools, err := a.ensureTools(a.ctx)
	if err != nil {
		return "", err
	}
	return tools.FFmpeg, nil
}

// Capabilities 返回编解码能力（编码器列表、是否可用硬解）。
func (a *App) Capabilities() (ffmpeg.Capabilities, error) {
	tools, err := a.ensureTools(a.ctx)
	if err != nil {
		return ffmpeg.Capabilities{}, err
	}
	caps, err := a.detector.Get(a.ctx, tools)
	if err != nil {
		return ffmpeg.Capabilities{}, err
	}
	return *caps, nil
}

// StartJob 校验参数并异步启动任务；单任务并发保护。
func (a *App) StartJob(p config.Params) error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return errors.New("已有任务正在运行")
	}
	a.mu.Unlock()

	tools, err := a.ensureTools(a.ctx)
	if err != nil {
		return err
	}
	meta, err := probe.Probe(a.ctx, tools.FFprobe, p.Input)
	if err != nil {
		return err
	}
	if err := p.Validate(meta.FPS); err != nil {
		return err
	}
	caps, err := a.detector.Get(a.ctx, tools)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(a.ctx)
	a.mu.Lock()
	a.running = true
	a.cancel = cancel
	a.mu.Unlock()

	go a.runJob(ctx, tools, caps, p, meta)
	return nil
}

func (a *App) runJob(ctx context.Context, tools ffmpeg.Tools, caps *ffmpeg.Capabilities, p config.Params, meta probe.VideoMeta) {
	defer func() {
		a.mu.Lock()
		a.running = false
		a.cancel = nil
		a.mu.Unlock()
	}()

	runner := &pipeline.Runner{Tools: tools, Caps: caps}
	res, err := runner.Run(ctx, p, meta, pipeline.Sink{
		OnPhase: func(phase string) {
			wailsruntime.EventsEmit(a.ctx, "job:phase", map[string]any{"phase": phase})
		},
		OnProgress: func(phase string, value float64) {
			wailsruntime.EventsEmit(a.ctx, "job:progress", map[string]any{"phase": phase, "value": value})
		},
	})
	if err != nil {
		msg := err.Error()
		if errors.Is(err, context.Canceled) {
			msg = "任务已取消"
		}
		wailsruntime.EventsEmit(a.ctx, "job:error", map[string]any{"message": msg})
		return
	}
	wailsruntime.EventsEmit(a.ctx, "job:done", res)
}

// CancelJob 取消当前任务。
func (a *App) CancelJob() {
	a.mu.Lock()
	cancel := a.cancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// OpenOutputDir 在系统文件管理器中打开目录。
func (a *App) OpenOutputDir(path string) error {
	if path == "" {
		return errors.New("路径为空")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

// GetSettings 读取应用设置。
func (a *App) GetSettings() (config.Settings, error) {
	return config.LoadSettings()
}

// SaveSettings 保存应用设置，并清除已缓存的工具路径。
func (a *App) SaveSettings(s config.Settings) error {
	if err := s.Save(); err != nil {
		return err
	}
	a.mu.Lock()
	a.tools = nil
	a.mu.Unlock()
	return nil
}

// ensureTools 定位 ffmpeg；缺失时（仅 Windows）触发下载并缓存。
func (a *App) ensureTools(ctx context.Context) (ffmpeg.Tools, error) {
	a.mu.Lock()
	cached := a.tools
	a.mu.Unlock()
	if cached != nil {
		return *cached, nil
	}

	s, err := config.LoadSettings()
	if err != nil {
		return ffmpeg.Tools{}, err
	}
	tools, err := ffmpeg.Ensure(ctx, s.FFmpegPath, func(done, total int64) {
		wailsruntime.EventsEmit(a.ctx, "ffmpeg:download", map[string]int64{"done": done, "total": total})
	})
	if err != nil {
		return ffmpeg.Tools{}, fmt.Errorf("ffmpeg 未就绪：%w", err)
	}
	a.mu.Lock()
	a.tools = &tools
	a.mu.Unlock()
	return tools, nil
}
