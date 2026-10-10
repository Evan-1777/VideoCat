// Package config 定义任务参数模型、默认值、字段级校验与应用设置持久化。
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Resolution 输出分辨率枚举。source 表示保持原始分辨率。
type Resolution string

const (
	ResSource Resolution = "source"
	Res480    Resolution = "480"
	Res720    Resolution = "720"
	Res1080   Resolution = "1080"
)

// AudioFormat 音频输出格式枚举。
type AudioFormat string

const (
	AudioAAC  AudioFormat = "aac"
	AudioMP3  AudioFormat = "mp3"
	AudioFLAC AudioFormat = "flac"
)

// Encoder 视频编码器选择。auto 由能力探测决定。
type Encoder string

const (
	EncoderAuto  Encoder = "auto"
	EncoderNVENC Encoder = "h264_nvenc"
	EncoderQSV   Encoder = "h264_qsv"
	EncoderAMF   Encoder = "h264_amf"
	EncoderCPU   Encoder = "libx264"
)

// Params 单次转换任务参数。
type Params struct {
	Input           string      `json:"input"`
	FramesPerSecond int         `json:"framesPerSecond"` // 每秒保留帧数 m
	Resolution      Resolution  `json:"resolution"`
	OutputFPS       float64     `json:"outputFps"` // 0 表示跟随源帧率
	ExtractAudio    bool        `json:"extractAudio"`
	AudioFormat     AudioFormat `json:"audioFormat"`
	AudioBitrate    string      `json:"audioBitrate"` // 仅 aac / mp3 生效
	Encoder         Encoder     `json:"encoder"`
	HardwareDecode  bool        `json:"hardwareDecode"`
	CRF             int         `json:"crf"`
	OutputDir       string      `json:"outputDir"` // 空表示源文件所在目录
}

// DefaultParams 返回给定输入的默认参数（720p / AAC / m=4 / 编码器 auto）。
func DefaultParams(input string) Params {
	return Params{
		Input:           input,
		FramesPerSecond: 4,
		Resolution:      Res720,
		OutputFPS:       0,
		ExtractAudio:    true,
		AudioFormat:     AudioAAC,
		AudioBitrate:    "128k",
		Encoder:         EncoderAuto,
		HardwareDecode:  true,
		CRF:             23,
	}
}

// Fields 字段级校验错误，键为字段名。
type Fields map[string]string

func (f Fields) Error() string {
	if len(f) == 0 {
		return "参数校验失败"
	}
	keys := make([]string, 0, len(f))
	for k := range f {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+": "+f[k])
	}
	return strings.Join(parts, "; ")
}

// Validate 校验参数。srcFPS 为源视频平均帧率，用于约束 m 的上界；未知时传 0。
func (p Params) Validate(srcFPS float64) error {
	f := Fields{}

	if strings.TrimSpace(p.Input) == "" {
		f["input"] = "请选择输入视频"
	}

	maxM := int(srcFPS)
	if p.FramesPerSecond < 1 {
		f["framesPerSecond"] = "每秒保留帧数至少为 1"
	} else if maxM > 0 && p.FramesPerSecond > maxM {
		f["framesPerSecond"] = fmt.Sprintf("每秒保留帧数不能超过源帧率 %d", maxM)
	}

	switch p.Resolution {
	case ResSource, Res480, Res720, Res1080:
	default:
		f["resolution"] = "不支持的分辨率"
	}

	switch p.AudioFormat {
	case AudioAAC, AudioMP3, AudioFLAC:
	default:
		f["audioFormat"] = "不支持的音频格式"
	}

	switch p.Encoder {
	case EncoderAuto, EncoderNVENC, EncoderQSV, EncoderAMF, EncoderCPU:
	default:
		f["encoder"] = "不支持的视频编码器"
	}

	if p.CRF < 0 || p.CRF > 51 {
		f["crf"] = "CRF 需在 0–51 之间"
	}

	if p.OutputDir != "" {
		if err := ensureWritableDir(p.OutputDir); err != nil {
			f["outputDir"] = "输出目录不可写：" + err.Error()
		}
	}

	if len(f) > 0 {
		return f
	}
	return nil
}

// ensureWritableDir 确认目录可创建且可写。
func ensureWritableDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".writetest-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	tmp.Close()
	os.Remove(name)
	return nil
}

// DataDir 返回应用数据目录：Windows 为 %LOCALAPPDATA%\video cat，其他平台为 os.UserCacheDir()/video cat。
func DataDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "video cat"), nil
}
