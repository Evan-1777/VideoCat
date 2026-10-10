package pipeline

import (
	"context"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"videocat/internal/config"
	"videocat/internal/ffmpeg"
	"videocat/internal/probe"
	"videocat/internal/testutil"
)

func newRunner(t *testing.T) (*Runner, context.Context) {
	t.Helper()
	tools := testutil.Tools(t)
	ctx := context.Background()
	caps, err := ffmpeg.Probe(ctx, tools)
	if err != nil {
		t.Fatalf("能力探测失败：%v", err)
	}
	return &Runner{Tools: tools, Caps: caps}, ctx
}

func TestRunEndToEnd(t *testing.T) {
	runner, ctx := newRunner(t)
	dir := t.TempDir()
	src := testutil.SampleSize(t, runner.Tools, dir, "src720.mp4", 1280, 720, 10, true)
	meta, err := probe.Probe(ctx, runner.Tools.FFprobe, src)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}

	p := config.DefaultParams(src)
	p.FramesPerSecond = 20
	p.Resolution = config.Res720
	p.OutputDir = filepath.Join(dir, "out")

	var phases []string
	res, err := runner.Run(ctx, p, meta, Sink{OnPhase: func(ph string) { phases = append(phases, ph) }})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(phases) != 2 || phases[0] != PhaseRender || phases[1] != PhaseAudio {
		t.Errorf("phases = %v, want [render audio]", phases)
	}
	for _, f := range []string{res.Outputs.Video, res.Outputs.Audio, res.Outputs.Meta} {
		if _, err := os.Stat(f); err != nil {
			t.Fatalf("missing output %s: %v", f, err)
		}
	}

	outMeta, err := probe.Probe(ctx, runner.Tools.FFprobe, res.Outputs.Video)
	if err != nil {
		t.Fatalf("probe output: %v", err)
	}
	if outMeta.Height != 720 {
		t.Errorf("output height = %d, want 720", outMeta.Height)
	}
	if outMeta.HasAudio {
		t.Error("output video must not contain audio")
	}
	wantDur := meta.Duration * float64(p.FramesPerSecond) / meta.FPS
	if tol := wantDur * 0.05; outMeta.Duration < wantDur-tol || outMeta.Duration > wantDur+tol {
		t.Errorf("output duration = %.3f, want ~%.3f (±5%%)", outMeta.Duration, wantDur)
	}

	audioDur := testutil.Duration(t, runner.Tools, res.Outputs.Audio)
	if tol := meta.Duration * 0.02; audioDur < meta.Duration-tol || audioDur > meta.Duration+tol {
		t.Errorf("audio duration = %.3f, want ~%.3f (±2%%)", audioDur, meta.Duration)
	}

	// 元数据的输出时长/压缩比须与 ffprobe 实测一致（而非理论预测值）。
	if !res.Meta.OutputDurationMeasured {
		t.Error("meta.outputDurationMeasured = false，期望用实测值回填")
	}
	if diff := res.Meta.OutputDuration - outMeta.Duration; math.Abs(diff) > outMeta.Duration*0.01 {
		t.Errorf("meta outputDuration %.3f 与实测 %.3f 不一致（>1%%）", res.Meta.OutputDuration, outMeta.Duration)
	}
	wantRatio := meta.Duration / outMeta.Duration
	if math.Abs(res.Meta.CompressionRatio-wantRatio) > wantRatio*0.01 {
		t.Errorf("meta compressionRatio %.3f 与实测推算 %.3f 不一致（>1%%）", res.Meta.CompressionRatio, wantRatio)
	}
}

func TestRunSkipsAudioWhenSourceHasNone(t *testing.T) {
	runner, ctx := newRunner(t)
	dir := t.TempDir()
	src := testutil.SampleSize(t, runner.Tools, dir, "noaudio.mp4", 640, 360, 2, false)
	meta, err := probe.Probe(ctx, runner.Tools.FFprobe, src)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}

	p := config.DefaultParams(src)
	p.OutputDir = filepath.Join(dir, "out")
	res, err := runner.Run(ctx, p, meta, Sink{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !res.AudioSkipped || res.Outputs.Audio != "" {
		t.Errorf("expected audio skipped with empty path, got skipped=%v path=%q", res.AudioSkipped, res.Outputs.Audio)
	}
}

func TestRunDoesNotUpscaleSmallSource(t *testing.T) {
	runner, ctx := newRunner(t)
	dir := t.TempDir()
	src := testutil.SampleSize(t, runner.Tools, dir, "small.mp4", 640, 360, 2, false)
	meta, err := probe.Probe(ctx, runner.Tools.FFprobe, src)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}

	p := config.DefaultParams(src)
	p.Resolution = config.Res720
	p.ExtractAudio = false
	p.OutputDir = filepath.Join(dir, "out")
	res, err := runner.Run(ctx, p, meta, Sink{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	outMeta, err := probe.Probe(ctx, runner.Tools.FFprobe, res.Outputs.Video)
	if err != nil {
		t.Fatalf("probe output: %v", err)
	}
	if outMeta.Height != 360 {
		t.Errorf("height = %d, want 360 (no upscale)", outMeta.Height)
	}
}

// TestRenderDeterminism 断言 ffmpeg random() 对同一输入的确定性（.docs/Project.md 记录的不变式）。
func TestRenderDeterminism(t *testing.T) {
	runner, ctx := newRunner(t)
	dir := t.TempDir()
	src := testutil.SampleSize(t, runner.Tools, dir, "det.mp4", 320, 240, 2, false)
	meta, err := probe.Probe(ctx, runner.Tools.FFprobe, src)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	p := config.DefaultParams(src)
	p.FramesPerSecond = 8
	p.ExtractAudio = false

	sums := make([]string, 2)
	for i := range sums {
		out := filepath.Join(dir, "out"+string(rune('0'+i))+".mp4")
		if _, err := ffmpeg.Run(ctx, runner.Tools.FFmpeg, BuildVideoArgs(p, meta, "libx264", false, out), nil); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		cmd := exec.Command(runner.Tools.FFmpeg, "-hide_banner", "-loglevel", "error", "-i", out, "-f", "framemd5", "-")
		data, err := cmd.Output()
		if err != nil {
			t.Fatalf("framemd5 %d: %v", i, err)
		}
		sums[i] = string(data)
	}
	// ffmpeg random() 对同一输入确定性；若该不变式被打破，须同步更新 .docs/Project.md。
	if sums[0] != sums[1] {
		t.Error("随机抽帧不再确定：两次运行所选帧不一致；若为 ffmpeg 行为变化，请更新 .docs/Project.md")
	}
}

// TestRunCancel 验证取消能在数秒内终止 ffmpeg 进程。
func TestRunCancel(t *testing.T) {
	runner, _ := newRunner(t)
	dir := t.TempDir()
	src := testutil.SampleSize(t, runner.Tools, dir, "cancel.mp4", 1280, 720, 60, true)
	meta, err := probe.Probe(context.Background(), runner.Tools.FFprobe, src)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}

	p := config.DefaultParams(src)
	p.FramesPerSecond = 20
	p.Resolution = config.Res720
	p.OutputDir = filepath.Join(dir, "out")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := runner.Run(ctx, p, meta, Sink{})
		done <- err
	}()

	time.Sleep(500 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("取消未能在 5 秒内终止 ffmpeg 进程")
	}
}
