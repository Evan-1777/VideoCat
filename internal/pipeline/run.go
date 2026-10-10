package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"videocat/internal/config"
	"videocat/internal/ffmpeg"
	"videocat/internal/probe"
)

// 阶段名，与前端 job:phase 事件一致。
const (
	PhaseRender = "render"
	PhaseAudio  = "audio"
)

// Sink 接收阶段切换与阶段内进度（0..1）。
type Sink struct {
	OnPhase    func(phase string)
	OnProgress func(phase string, value float64)
}

// Runner 执行一次完整任务。
type Runner struct {
	Tools ffmpeg.Tools
	Caps  *ffmpeg.Capabilities
}

// Result 是任务执行结果。
type Result struct {
	Outputs        Outputs    `json:"outputs"`
	Encoder        string     `json:"encoder"`
	HardwareDecode bool       `json:"hardwareDecode"`
	OutputDuration float64    `json:"outputDuration"`
	AudioSkipped   bool       `json:"audioSkipped"`
	Meta           MetaRecord `json:"meta"`
}

// Run 依次执行抽帧重渲染与音频提取，并写出元数据 JSON。
func (r *Runner) Run(ctx context.Context, p config.Params, meta probe.VideoMeta, sink Sink) (Result, error) {
	outputs := OutputPaths(p, meta)
	if err := os.MkdirAll(filepath.Dir(outputs.Video), 0o755); err != nil {
		return Result{}, fmt.Errorf("创建输出目录失败：%w", err)
	}
	encoder := r.Caps.Resolve(p.Encoder)
	hwDecode := p.HardwareDecode && r.Caps.HardwareDecode
	phaseSeconds := map[string]float64{}

	srcFPS := meta.FPS
	if srcFPS <= 0 {
		srcFPS = 30
	}
	expectedOut := meta.Duration * float64(p.FramesPerSecond) / srcFPS

	// 阶段 1：抽帧重渲染
	if sink.OnPhase != nil {
		sink.OnPhase(PhaseRender)
	}
	start := time.Now()
	stderr, err := ffmpeg.Run(ctx, r.Tools.FFmpeg, BuildVideoArgs(p, meta, encoder, hwDecode, outputs.Video),
		func(sec float64, done bool) {
			emit(sink.OnProgress, PhaseRender, sec, expectedOut, done)
		})
	if err != nil {
		return Result{}, phaseError("抽帧重渲染", err, stderr)
	}
	phaseSeconds[PhaseRender] = time.Since(start).Seconds()

	// 以 ffprobe 实测输出视频时长回填元数据与结果（TASK-010 要求与实测一致；随机抽帧的理论值不等于实测值）。
	outputDuration := expectedOut
	durationMeasured := false
	if vp, perr := probe.Probe(ctx, r.Tools.FFprobe, outputs.Video); perr == nil && vp.Duration > 0 {
		outputDuration = vp.Duration
		durationMeasured = true
	}

	// 阶段 2：音频提取（源无音轨或未开启时跳过）
	audioSkipped := true
	if p.ExtractAudio && meta.HasAudio {
		audioSkipped = false
		if sink.OnPhase != nil {
			sink.OnPhase(PhaseAudio)
		}
		start = time.Now()
		stderr, err = ffmpeg.Run(ctx, r.Tools.FFmpeg, BuildAudioArgs(p, meta, outputs.Audio),
			func(sec float64, done bool) {
				emit(sink.OnProgress, PhaseAudio, sec, meta.Duration, done)
			})
		if err != nil {
			return Result{}, phaseError("音频提取", err, stderr)
		}
		phaseSeconds[PhaseAudio] = time.Since(start).Seconds()
	} else {
		outputs.Audio = ""
	}

	ratio := 0.0
	if outputDuration > 0 {
		ratio = meta.Duration / outputDuration
	}
	rec := MetaRecord{
		Source:                 meta,
		Params:                 p,
		Encoder:                encoder,
		HardwareDecode:         hwDecode,
		Outputs:                outputs,
		SourceDuration:         meta.Duration,
		OutputDuration:         outputDuration,
		OutputDurationMeasured: durationMeasured,
		CompressionRatio:       ratio,
		PhaseSeconds:           phaseSeconds,
		AudioSkipped:           audioSkipped,
	}
	if err := WriteMeta(outputs.Meta, rec); err != nil {
		return Result{}, fmt.Errorf("写入元数据失败：%w", err)
	}

	return Result{
		Outputs:        outputs,
		Encoder:        encoder,
		HardwareDecode: hwDecode,
		OutputDuration: outputDuration,
		AudioSkipped:   audioSkipped,
		Meta:           rec,
	}, nil
}

func emit(onProgress func(string, float64), phase string, sec, total float64, done bool) {
	if onProgress == nil {
		return
	}
	if done {
		onProgress(phase, 1)
		return
	}
	v := 0.0
	if total > 0 {
		v = sec / total
	}
	if v < 0 {
		v = 0
	} else if v > 1 {
		v = 1
	}
	onProgress(phase, v)
}

// phaseError 组合阶段名、错误与 stderr 摘要。
func phaseError(phase string, err error, stderr string) error {
	msg := strings.TrimSpace(stderr)
	if msg != "" {
		return fmt.Errorf("%s失败：%w（%s）", phase, err, lastLine(msg))
	}
	return fmt.Errorf("%s失败：%w", phase, err)
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
