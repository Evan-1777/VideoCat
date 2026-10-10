// Package pipeline 负责 ffmpeg 命令构建、任务编排与元数据输出。
package pipeline

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"videocat/internal/config"
	"videocat/internal/ffmpeg"
	"videocat/internal/probe"
)

// Outputs 是三个输出文件的路径。
type Outputs struct {
	Video string `json:"video"`
	Audio string `json:"audio"`
	Meta  string `json:"meta"`
}

// AudioExt 返回音频格式对应的容器扩展名。
func AudioExt(f config.AudioFormat) string {
	switch f {
	case config.AudioMP3:
		return "mp3"
	case config.AudioFLAC:
		return "flac"
	default:
		return "m4a"
	}
}

// audioCodec 返回格式 → 编码器名；usesBitrate 指示是否注入 -b:a。
func audioCodec(f config.AudioFormat) (codec string, usesBitrate bool) {
	switch f {
	case config.AudioMP3:
		return "libmp3lame", true
	case config.AudioFLAC:
		return "flac", false
	default:
		return "aac", true
	}
}

// OutputPaths 计算输出路径（默认与源文件同目录）。
func OutputPaths(p config.Params, meta probe.VideoMeta) Outputs {
	dir := p.OutputDir
	if dir == "" {
		dir = filepath.Dir(meta.Path)
	}
	name := strings.TrimSuffix(filepath.Base(meta.Path), filepath.Ext(meta.Path))
	return Outputs{
		Video: filepath.Join(dir, name+".cat.mp4"),
		Audio: filepath.Join(dir, name+".cat."+AudioExt(p.AudioFormat)),
		Meta:  filepath.Join(dir, name+".cat.meta.json"),
	}
}

func targetHeight(res config.Resolution) int {
	switch res {
	case config.Res480:
		return 480
	case config.Res720:
		return 720
	case config.Res1080:
		return 1080
	default:
		return 0
	}
}

// VideoFilter 构建抽帧重渲染滤镜链：随机抽帧 → 重打包 → 缩放。
func VideoFilter(p config.Params, meta probe.VideoMeta) string {
	a := meta.FPS
	if a <= 0 {
		a = 30
	}
	pKeep := float64(p.FramesPerSecond) / a
	parts := []string{
		fmt.Sprintf("select='lt(random(0),%g)'", pKeep),
		fmt.Sprintf("setpts=N/(%g*TB)", a),
	}
	if h := targetHeight(p.Resolution); h > 0 {
		// 高度取偶数，兼容 yuv420p（源高为奇数时不置于报错）。
		parts = append(parts, fmt.Sprintf("scale=-2:'trunc(min(ih,%d)/2)*2'", h))
	}
	return strings.Join(parts, ",")
}

// BuildVideoArgs 构建压缩视频重渲染的 ffmpeg 参数（不含可执行文件）。
func BuildVideoArgs(p config.Params, meta probe.VideoMeta, encoder string, hwDecode bool, output string) []string {
	a := meta.FPS
	if a <= 0 {
		a = 30
	}
	outFPS := p.OutputFPS
	if outFPS <= 0 {
		outFPS = a
	}

	args := []string{"-hide_banner", "-loglevel", "error", "-y", "-progress", "pipe:1", "-nostats"}
	if hwDecode {
		args = append(args, "-hwaccel", "auto")
	}
	args = append(args, "-i", meta.Path)
	args = append(args, "-vf", VideoFilter(p, meta))
	args = append(args, "-an")
	args = append(args, "-c:v", encoder)
	args = append(args, ffmpeg.QualityArgs(encoder, p.CRF)...)
	args = append(args, "-r", formatFloat(outFPS))
	args = append(args, output)
	return args
}

// BuildAudioArgs 构建音频提取的 ffmpeg 参数。调用方须先确认源含音轨。
func BuildAudioArgs(p config.Params, meta probe.VideoMeta, output string) []string {
	codec, usesBitrate := audioCodec(p.AudioFormat)
	args := []string{"-hide_banner", "-loglevel", "error", "-y", "-progress", "pipe:1", "-nostats"}
	args = append(args, "-i", meta.Path, "-vn", "-c:a", codec)
	if usesBitrate && p.AudioBitrate != "" {
		args = append(args, "-b:a", p.AudioBitrate)
	}
	args = append(args, output)
	return args
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
