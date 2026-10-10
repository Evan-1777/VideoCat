// Package ffmpeg 负责 ffmpeg/ffprobe 的定位、下载、能力探测与进程执行。
package ffmpeg

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"videocat/internal/config"
)

// Tools 是解析出的 ffmpeg / ffprobe 可执行文件路径。
type Tools struct {
	FFmpeg  string
	FFprobe string
}

func exeName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

// CacheDir 返回 ffmpeg 缓存目录（<应用数据目录>/ffmpeg）。
func CacheDir() (string, error) {
	dir, err := config.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ffmpeg"), nil
}

// Locate 按 用户指定 → 应用缓存 → 系统 PATH 顺序解析 ffmpeg/ffprobe。
func Locate(userPath string) (Tools, error) {
	if userPath != "" {
		if t, err := locateIn(userPath); err == nil {
			return t, nil
		}
	}
	if dir, err := CacheDir(); err == nil {
		if t, err := locateIn(dir); err == nil {
			return t, nil
		}
	}
	ff, err1 := exec.LookPath(exeName("ffmpeg"))
	fp, err2 := exec.LookPath(exeName("ffprobe"))
	if err1 == nil && err2 == nil {
		return Tools{FFmpeg: ff, FFprobe: fp}, nil
	}
	return Tools{}, errors.New("未找到 ffmpeg/ffprobe")
}

// locateIn 在目录（或 ffmpeg 可执行文件所在目录）中查找两个可执行文件。
func locateIn(p string) (Tools, error) {
	info, err := os.Stat(p)
	if err != nil {
		return Tools{}, err
	}
	dir := p
	if !info.IsDir() {
		dir = filepath.Dir(p)
	}
	ff := filepath.Join(dir, exeName("ffmpeg"))
	fp := filepath.Join(dir, exeName("ffprobe"))
	if isExecutable(ff) && isExecutable(fp) {
		return Tools{FFmpeg: ff, FFprobe: fp}, nil
	}
	return Tools{}, fmt.Errorf("%s 中未找到 ffmpeg/ffprobe", dir)
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}
