package ffmpeg

import (
	"testing"

	"videocat/internal/config"
)

const sampleEncoders = `Encoders:
 V..... = Video
 A..... = Audio
 ------
 V....D libx264              libx264 H.264 / AVC (codec h264)
 V....D h264_nvenc           NVIDIA NVENC H.264 encoder (codec h264)
 V....D h264_qsv             H.264 via Intel Quick Sync Video (codec h264)
 A....D aac                  AAC (Advanced Audio Coding)
`

func TestParseEncoders(t *testing.T) {
	m := parseEncoders(sampleEncoders)
	for _, want := range []string{"libx264", "h264_nvenc", "h264_qsv"} {
		if !m[want] {
			t.Errorf("expected encoder %q present", want)
		}
	}
	if m["="] {
		t.Error("legend line should not be parsed as encoder")
	}
	if m["aac"] {
		t.Error("audio encoder should not be counted as video")
	}
}

func TestParseHWAccels(t *testing.T) {
	out := "Hardware acceleration methods:\ncuda\nqsv\nd3d11va\n"
	got := parseHWAccels(out)
	want := []string{"cuda", "qsv", "d3d11va"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
	if parseHWAccels("Hardware acceleration methods:\n") != nil {
		t.Error("empty hwaccels should produce nil")
	}
}

func TestResolveEncoder(t *testing.T) {
	cpuOnly := &Capabilities{}
	if got := cpuOnly.Resolve(config.EncoderAuto); got != "libx264" {
		t.Errorf("auto with no hw = %q, want libx264", got)
	}
	if got := cpuOnly.Resolve(config.EncoderNVENC); got != "libx264" {
		t.Errorf("nvenc unavailable should fall back, got %q", got)
	}

	withHW := &Capabilities{HWEncoders: []string{"h264_nvenc", "hevc_qsv"}}
	if got := withHW.Resolve(config.EncoderAuto); got != "h264_nvenc" {
		t.Errorf("auto should pick first hw = %q", got)
	}
	if got := withHW.Resolve(config.EncoderQSV); got != "libx264" {
		t.Errorf("qsv requested but only hevc_qsv present should fall back: got %q", got)
	}
	if got := withHW.Resolve(config.EncoderCPU); got != "libx264" {
		t.Errorf("explicit cpu = %q", got)
	}
	names := withHW.EncoderNames()
	if names[len(names)-1] != "libx264" {
		t.Errorf("last encoder should be libx264 fallback, got %v", names)
	}
}
