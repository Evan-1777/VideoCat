package probe

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"

	"videocat/internal/testutil"
)

func TestRateParsing(t *testing.T) {
	cases := map[string]float64{
		"24000/1001": 23.976023976,
		"30/1":       30,
		"25":         25,
		"0/0":        0,
		"":           0,
		"nonsense":   0,
	}
	for in, want := range cases {
		got := rate(in)
		if diff := got - want; diff > 1e-6 || diff < -1e-6 {
			t.Errorf("rate(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestProbeSample(t *testing.T) {
	tools := testutil.Tools(t)
	ctx := context.Background()
	dir := t.TempDir()

	withAudio := testutil.Sample(t, tools, dir, "with-audio.mp4", true)
	meta, err := Probe(ctx, tools.FFprobe, withAudio)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if meta.Width != 640 || meta.Height != 360 {
		t.Errorf("resolution = %dx%d, want 640x360", meta.Width, meta.Height)
	}
	if meta.FPS < 23.97 || meta.FPS > 23.98 {
		t.Errorf("fps = %v, want ~23.976", meta.FPS)
	}
	if meta.Duration < 1.8 || meta.Duration > 2.2 {
		t.Errorf("duration = %v, want ~2", meta.Duration)
	}
	if !meta.HasAudio || meta.AudioCodec != "aac" {
		t.Errorf("audio = %v/%q, want true/aac", meta.HasAudio, meta.AudioCodec)
	}
	if meta.SizeBytes <= 0 {
		t.Errorf("size = %d, want > 0", meta.SizeBytes)
	}

	noAudio := testutil.Sample(t, tools, dir, "no-audio.mp4", false)
	meta2, err := Probe(ctx, tools.FFprobe, noAudio)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if meta2.HasAudio {
		t.Error("expected no audio track")
	}
}

func TestProbeAudioOnlyReturnsErrNoVideo(t *testing.T) {
	tools := testutil.Tools(t)
	ctx := context.Background()
	dir := t.TempDir()
	m4a := filepath.Join(dir, "audio-only.m4a")
	args := []string{"-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-c:a", "aac", m4a}
	if out, err := exec.CommandContext(ctx, tools.FFmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("gen audio-only: %v %s", err, out)
	}

	_, err := Probe(ctx, tools.FFprobe, m4a)
	if !errors.Is(err, ErrNoVideo) {
		t.Fatalf("err = %v, want ErrNoVideo", err)
	}
}
