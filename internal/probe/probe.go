// Package probe 通过 ffprobe 读取视频元数据。
package probe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// VideoMeta 是解析出的源视频元数据。
type VideoMeta struct {
	Path       string  `json:"path"`
	FPS        float64 `json:"fps"`
	Duration   float64 `json:"duration"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	VideoCodec string  `json:"videoCodec"`
	HasAudio   bool    `json:"hasAudio"`
	AudioCodec string  `json:"audioCodec"`
	SizeBytes  int64   `json:"sizeBytes"`
}

type ffprobeResult struct {
	Streams []struct {
		CodecType    string `json:"codec_type"`
		CodecName    string `json:"codec_name"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		AvgFrameRate string `json:"avg_frame_rate"`
		RFrameRate   string `json:"r_frame_rate"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
		Size     string `json:"size"`
	} `json:"format"`
}

// ErrNoVideo 表示输入不含视频流。
var ErrNoVideo = errors.New("输入文件不包含视频流")

// Probe 调用 ffprobe 解析元数据。
func Probe(ctx context.Context, ffprobePath, path string) (VideoMeta, error) {
	args := []string{"-v", "error", "-show_format", "-show_streams", "-of", "json", path}
	out, err := exec.CommandContext(ctx, ffprobePath, args...).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return VideoMeta{}, fmt.Errorf("ffprobe 解析失败：%s", strings.TrimSpace(string(ee.Stderr)))
		}
		return VideoMeta{}, fmt.Errorf("调用 ffprobe 失败：%w", err)
	}

	var res ffprobeResult
	if err := json.Unmarshal(out, &res); err != nil {
		return VideoMeta{}, fmt.Errorf("解析 ffprobe 输出失败：%w", err)
	}

	meta := VideoMeta{Path: path}
	for _, s := range res.Streams {
		switch s.CodecType {
		case "video":
			if meta.VideoCodec == "" {
				meta.Width = s.Width
				meta.Height = s.Height
				meta.VideoCodec = s.CodecName
				meta.FPS = rate(s.AvgFrameRate)
				if meta.FPS == 0 {
					meta.FPS = rate(s.RFrameRate)
				}
			}
		case "audio":
			if !meta.HasAudio {
				meta.HasAudio = true
				meta.AudioCodec = s.CodecName
			}
		}
	}
	if meta.VideoCodec == "" {
		return VideoMeta{}, ErrNoVideo
	}

	meta.Duration = parseFloat(res.Format.Duration)
	if n, err := strconv.ParseInt(res.Format.Size, 10, 64); err == nil {
		meta.SizeBytes = n
	}
	return meta, nil
}

// rate 解析 "num/den" 形式的帧率为浮点；非法输入返回 0。
func rate(s string) float64 {
	if s == "" {
		return 0
	}
	num, den, ok := strings.Cut(s, "/")
	if !ok {
		return parseFloat(s)
	}
	n, err1 := strconv.ParseFloat(num, 64)
	d, err2 := strconv.ParseFloat(den, 64)
	if err1 != nil || err2 != nil || d == 0 {
		return 0
	}
	return n / d
}

func parseFloat(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v
}
