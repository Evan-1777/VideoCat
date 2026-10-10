package pipeline

import (
	"strings"
	"testing"

	"videocat/internal/config"
	"videocat/internal/probe"
)

func sampleMeta() probe.VideoMeta {
	return probe.VideoMeta{
		Path:       "/videos/demo.mp4",
		FPS:        23.976023976,
		Duration:   120,
		Width:      1920,
		Height:     1080,
		VideoCodec: "h264",
		HasAudio:   true,
		AudioCodec: "aac",
	}
}

func TestBuildVideoArgs(t *testing.T) {
	p := config.DefaultParams("/videos/demo.mp4")
	p.FramesPerSecond = 8
	args := BuildVideoArgs(p, sampleMeta(), "h264_nvenc", true, "/out/demo.cat.mp4")
	joined := strings.Join(args, " ")

	for _, want := range []string{
		"select='lt(random(0),", "setpts=N/(", "scale=-2:'min(ih,720)'",
		"-an", "-c:v h264_nvenc", "-hwaccel auto", "-progress pipe:1", "-nostats",
		"/out/demo.cat.mp4",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("video args missing %q\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "-crf") {
		t.Errorf("nvenc should not use -crf: %s", joined)
	}
}

func TestBuildVideoArgsSourceResolutionNoScale(t *testing.T) {
	p := config.DefaultParams("/videos/demo.mp4")
	p.Resolution = config.ResSource
	joined := strings.Join(BuildVideoArgs(p, sampleMeta(), "libx264", false, "/o.mp4"), " ")
	if strings.Contains(joined, "scale=") {
		t.Errorf("source resolution must not scale: %s", joined)
	}
	if strings.Contains(joined, "-hwaccel") {
		t.Errorf("hw decode off must not inject -hwaccel: %s", joined)
	}
	if !strings.Contains(joined, "-crf") {
		t.Errorf("libx264 should use -crf: %s", joined)
	}
}

func TestAudioCodecMapping(t *testing.T) {
	cases := []struct {
		format      config.AudioFormat
		codec       string
		ext         string
		usesBitrate bool
	}{
		{config.AudioAAC, "aac", "m4a", true},
		{config.AudioMP3, "libmp3lame", "mp3", true},
		{config.AudioFLAC, "flac", "flac", false},
	}
	for _, c := range cases {
		if got := AudioExt(c.format); got != c.ext {
			t.Errorf("AudioExt(%s) = %s, want %s", c.format, got, c.ext)
		}
		p := config.DefaultParams("/videos/demo.mp4")
		p.AudioFormat = c.format
		p.AudioBitrate = "192k"
		joined := strings.Join(BuildAudioArgs(p, sampleMeta(), "/out/demo.cat."+c.ext), " ")
		if !strings.Contains(joined, "-c:a "+c.codec) {
			t.Errorf("%s: missing codec %s\n%s", c.format, c.codec, joined)
		}
		if c.usesBitrate && !strings.Contains(joined, "-b:a 192k") {
			t.Errorf("%s: expected -b:a\n%s", c.format, joined)
		}
		if !c.usesBitrate && strings.Contains(joined, "-b:a") {
			t.Errorf("%s: must not contain -b:a\n%s", c.format, joined)
		}
		if !strings.Contains(joined, "-vn") {
			t.Errorf("%s: missing -vn\n%s", c.format, joined)
		}
	}
}

func TestOutputPaths(t *testing.T) {
	p := config.DefaultParams("/videos/demo.mp4")
	p.AudioFormat = config.AudioAAC
	o := OutputPaths(p, sampleMeta())
	if o.Video != "/videos/demo.cat.mp4" {
		t.Errorf("video = %s", o.Video)
	}
	if o.Audio != "/videos/demo.cat.m4a" {
		t.Errorf("audio = %s", o.Audio)
	}
	if o.Meta != "/videos/demo.cat.meta.json" {
		t.Errorf("meta = %s", o.Meta)
	}

	p.OutputDir = "/custom"
	o = OutputPaths(p, sampleMeta())
	if o.Video != "/custom/demo.cat.mp4" {
		t.Errorf("custom video = %s", o.Video)
	}
}
