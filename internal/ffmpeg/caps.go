package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"videocat/internal/config"
)

// 硬件编码器候选：H.264 优先（兼容性），HEVC 备选；同编码器内 NVENC → QSV → AMF。
var hwEncoderCandidates = []string{
	"h264_nvenc", "h264_qsv", "h264_amf",
	"hevc_nvenc", "hevc_qsv", "hevc_amf",
}

const cpuEncoder = "libx264"

// 代表性全链路滤镜链：随机抽帧 + 重打包 + 缩放。
const selfCheckFilter = "select='lt(random(0),0.5)',setpts=N/(30*TB),scale=-2:'min(ih,720)'"

// 自检使用的代表性质量值，使探测覆盖真实质量参数组合。
const selfCheckCRF = 23

// Capabilities 记录探测到的编解码能力。
type Capabilities struct {
	HWAccels       []string `json:"hwaccels"`
	HWEncoders     []string `json:"hwEncoders"`     // 通过全链路自检的硬件编码器，有序
	HardwareDecode bool     `json:"hardwareDecode"` // 硬解 + 软件滤镜链自检通过
}

func (c *Capabilities) has(name string) bool {
	for _, e := range c.HWEncoders {
		if e == name {
			return true
		}
	}
	return false
}

// EncoderNames 返回有序编码器列表：硬件在前，libx264 回退在后。
func (c *Capabilities) EncoderNames() []string {
	out := append([]string{}, c.HWEncoders...)
	return append(out, cpuEncoder)
}

// Resolve 依据用户选择与探测结果返回实际使用的 ffmpeg 编码器名。
func (c *Capabilities) Resolve(enc config.Encoder) string {
	switch enc {
	case config.EncoderCPU:
		return cpuEncoder
	case config.EncoderNVENC:
		if c.has("h264_nvenc") {
			return "h264_nvenc"
		}
	case config.EncoderQSV:
		if c.has("h264_qsv") {
			return "h264_qsv"
		}
	case config.EncoderAMF:
		if c.has("h264_amf") {
			return "h264_amf"
		}
	case config.EncoderAuto:
		if len(c.HWEncoders) > 0 {
			return c.HWEncoders[0]
		}
	}
	return cpuEncoder
}

// Detector 缓存探测结果，避免每次任务重复探测。
type Detector struct {
	mu    sync.Mutex
	cache map[string]*Capabilities
}

// NewDetector 创建带缓存的能力探测器。
func NewDetector() *Detector {
	return &Detector{cache: map[string]*Capabilities{}}
}

// Get 返回（可能已缓存的）能力结果。
func (d *Detector) Get(ctx context.Context, t Tools) (*Capabilities, error) {
	d.mu.Lock()
	if c, ok := d.cache[t.FFmpeg]; ok {
		d.mu.Unlock()
		return c, nil
	}
	d.mu.Unlock()

	c, err := Probe(ctx, t)
	if err != nil {
		return nil, err
	}
	d.mu.Lock()
	d.cache[t.FFmpeg] = c
	d.mu.Unlock()
	return c, nil
}

// Probe 执行一次能力探测：解析 hwaccels / encoders，并对候选做代表性全链路自检。
func Probe(ctx context.Context, t Tools) (*Capabilities, error) {
	caps := &Capabilities{}

	if out, err := output(ctx, t.FFmpeg, "-hide_banner", "-hwaccels"); err == nil {
		caps.HWAccels = parseHWAccels(out)
	}

	out, err := output(ctx, t.FFmpeg, "-hide_banner", "-encoders")
	if err != nil {
		return nil, fmt.Errorf("读取编码器列表失败：%w", err)
	}
	available := parseEncoders(out)

	clip, cleanup, err := makeProbeClip(ctx, t.FFmpeg)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	// 硬解 + 软件滤镜链 + CPU 编码自检：通过才允许运行期注入 -hwaccel。
	if selfCheck(ctx, t.FFmpeg, clip, cpuEncoder, true) == nil {
		caps.HardwareDecode = true
	}

	// 各候选硬件编码器全链路自检（沿用同一解码路径）。
	for _, name := range hwEncoderCandidates {
		if !available[name] {
			continue
		}
		if selfCheck(ctx, t.FFmpeg, clip, name, caps.HardwareDecode) == nil {
			caps.HWEncoders = append(caps.HWEncoders, name)
		}
	}
	return caps, nil
}

// selfCheck 运行 解码(+可选硬解) → 滤镜链 → 编码 → null 的全链路，1 帧输出。
func selfCheck(ctx context.Context, exe, input, encoder string, hwaccel bool) error {
	args := []string{"-hide_banner", "-loglevel", "error"}
	if hwaccel {
		args = append(args, "-hwaccel", "auto")
	}
	args = append(args,
		"-i", input,
		"-vf", selfCheckFilter,
		"-frames:v", "1",
		"-c:v", encoder,
	)
	args = append(args, QualityArgs(encoder, selfCheckCRF)...)
	args = append(args, "-f", "null", "-")
	out, err := exec.CommandContext(ctx, exe, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s 自检失败：%v %s", encoder, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// makeProbeClip 生成 1 秒测试片段用于自检；返回路径与清理函数。
func makeProbeClip(ctx context.Context, exe string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "videocat-probe-")
	if err != nil {
		return "", func() {}, err
	}
	clip := filepath.Join(dir, "probe.mp4")
	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=30",
		"-t", "1", "-c:v", cpuEncoder, "-pix_fmt", "yuv420p", clip,
	}
	if out, err := exec.CommandContext(ctx, exe, args...).CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		return "", func() {}, fmt.Errorf("生成探测样例失败：%v %s", err, strings.TrimSpace(string(out)))
	}
	return clip, func() { os.RemoveAll(dir) }, nil
}

func output(ctx context.Context, exe string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, exe, args...).Output()
	return string(out), err
}

func parseHWAccels(out string) []string {
	var res []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasSuffix(line, ":") {
			continue
		}
		res = append(res, line)
	}
	return res
}

func parseEncoders(out string) map[string]bool {
	m := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && len(fields[0]) >= 2 && fields[0][0] == 'V' && !strings.Contains(fields[1], "=") {
			m[fields[1]] = true
		}
	}
	return m
}
