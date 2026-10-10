package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateFieldErrors(t *testing.T) {
	p := DefaultParams("in.mp4")
	p.FramesPerSecond = 0
	p.Resolution = "4k"
	p.AudioFormat = "ogg"
	p.Encoder = "h265_magic"
	p.CRF = 99

	err := p.Validate(24)
	if err == nil {
		t.Fatal("expected validation error")
	}
	var f Fields
	if !errors.As(err, &f) {
		t.Fatalf("expected Fields, got %T", err)
	}
	for _, key := range []string{"framesPerSecond", "resolution", "audioFormat", "encoder", "crf"} {
		if _, ok := f[key]; !ok {
			t.Errorf("missing field error for %q", key)
		}
	}
}

func TestValidateFramesUpperBound(t *testing.T) {
	p := DefaultParams("in.mp4")
	p.FramesPerSecond = 24
	if err := p.Validate(23.976); err == nil {
		t.Fatal("expected error when m > floor(fps)=23")
	}
	p.FramesPerSecond = 23
	if err := p.Validate(23.976); err != nil {
		t.Fatalf("m = floor(fps) should pass, got %v", err)
	}
}

func TestValidateUnknownFPSAllowsAnyPositiveM(t *testing.T) {
	p := DefaultParams("in.mp4")
	p.FramesPerSecond = 60
	if err := p.Validate(0); err != nil {
		t.Fatalf("unknown fps should skip upper bound, got %v", err)
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("LOCALAPPDATA", cache) // Windows：os.UserCacheDir 读 %LocalAppData%
	want := Settings{DefaultOutputDir: filepath.Join(t.TempDir(), "out"), FFmpegPath: "/usr/bin/ffmpeg"}
	if err := want.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadSettings()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got != want {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, want)
	}
}

func TestLoadSettingsMissingFile(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("LOCALAPPDATA", cache) // Windows：os.UserCacheDir 读 %LocalAppData%
	got, err := LoadSettings()
	if err != nil {
		t.Fatalf("missing file should not error, got %v", err)
	}
	if (got != Settings{}) {
		t.Fatalf("expected zero settings, got %+v", got)
	}
	// 目录不应被读取操作意外创建。
	dir, _ := DataDir()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("DataDir should not exist after read-only load: %v", err)
	}
}
