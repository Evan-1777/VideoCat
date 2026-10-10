// Package testutil 提供测试用的样例生成与工具定位辅助（仅测试引用，不进入交付产物）。
package testutil

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"videocat/internal/ffmpeg"
)

// Tools 返回开发机上的 ffmpeg/ffprobe；不可用时跳过测试。
func Tools(t *testing.T) ffmpeg.Tools {
	t.Helper()
	tools, err := ffmpeg.Locate("")
	if err != nil {
		t.Skipf("跳过：未找到 ffmpeg/ffprobe（%v）", err)
	}
	return tools
}

// Sample 用 ffmpeg 现场生成样例 mp4（分数帧率 24000/1001，时长 2s）。
// withAudio 控制是否含音轨。返回文件路径。
func Sample(t *testing.T, tools ffmpeg.Tools, dir, name string, withAudio bool) string {
	t.Helper()
	return SampleSize(t, tools, dir, name, 640, 360, 2, withAudio)
}

// SampleSize 与 Sample 相同，但可指定源分辨率与时长（秒）。
func SampleSize(t *testing.T, tools ffmpeg.Tools, dir, name string, w, h int, seconds float64, withAudio bool) string {
	t.Helper()
	out := filepath.Join(dir, name)
	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", fmt.Sprintf("testsrc=size=%dx%d:rate=24000/1001", w, h),
	}
	if withAudio {
		args = append(args, "-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=440:duration=%g", seconds))
	}
	args = append(args, "-t", fmt.Sprintf("%g", seconds), "-c:v", "libx264", "-pix_fmt", "yuv420p")
	if withAudio {
		args = append(args, "-c:a", "aac", "-shortest")
	}
	args = append(args, out)

	if combined, err := exec.CommandContext(context.Background(), tools.FFmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("生成样例失败：%v\n%s", err, combined)
	}
	return out
}

// Duration 用 ffprobe 读取媒体时长（秒）。
func Duration(t *testing.T, tools ffmpeg.Tools, path string) float64 {
	t.Helper()
	out, err := exec.Command(tools.FFprobe,
		"-v", "error", "-show_entries", "format=duration",
		"-of", "default=nw=1:nk=1", path).Output()
	if err != nil {
		t.Fatalf("读取时长失败：%v", err)
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		t.Fatalf("解析时长失败：%v", err)
	}
	return v
}
