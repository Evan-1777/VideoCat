package ffmpeg

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// 钉死的不可变 release（tag 保证可复现，checksums.sha256 保证完整性）。
// 升级时同步修改这两个常量与下方的 url 前缀。
const (
	FFmpegReleaseTag = "autobuild-2026-10-10-13-04"
	FFmpegAssetName  = "ffmpeg-n9.0.2-25-g67b60c310b-win64-gpl-9.0.zip"

	releaseBaseURL = "https://github.com/BtbN/FFmpeg-Builds/releases/download/" + FFmpegReleaseTag + "/"
)

// Ensure 确保 ffmpeg 可用。缺失时仅在 Windows 触发下载；其他平台返回可读错误。
func Ensure(ctx context.Context, userPath string, onDownload func(done, total int64)) (Tools, error) {
	if t, err := Locate(userPath); err == nil {
		return t, nil
	}
	if runtime.GOOS != "windows" {
		return Tools{}, fmt.Errorf("未找到 ffmpeg/ffprobe，请安装并加入 PATH，或在设置中手动指定路径")
	}
	if err := downloadAndInstall(ctx, onDownload); err != nil {
		return Tools{}, err
	}
	return Locate(userPath)
}

func downloadAndInstall(ctx context.Context, onDownload func(done, total int64)) error {
	dir, err := CacheDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	sums, err := fetchChecksums(ctx)
	if err != nil {
		return fmt.Errorf("获取校验文件失败：%w", err)
	}
	want, ok := sums[FFmpegAssetName]
	if !ok {
		return fmt.Errorf("校验文件中缺少资产 %s", FFmpegAssetName)
	}

	tmpZip := filepath.Join(dir, "download.tmp")
	defer os.Remove(tmpZip)
	got, err := downloadFile(ctx, releaseBaseURL+FFmpegAssetName, tmpZip, onDownload)
	if err != nil {
		return fmt.Errorf("下载失败：%w", err)
	}
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("完整性校验失败：期望 %s，实际 %s", want, got)
	}
	if err := extractBinaries(tmpZip, dir); err != nil {
		return fmt.Errorf("解压失败：%w", err)
	}
	return nil
}

func fetchChecksums(ctx context.Context) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseBaseURL+"checksums.sha256", nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	sums := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			sums[fields[1]] = fields[0]
		}
	}
	return sums, nil
}

// downloadFile 下载到 dst，边下边算 SHA-256，返回十六进制摘要。
func downloadFile(ctx context.Context, url, dst string, onProgress func(done, total int64)) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	total := resp.ContentLength
	buf := make([]byte, 64*1024)
	var done int64
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return "", werr
			}
			h.Write(buf[:n])
			done += int64(n)
			if onProgress != nil {
				onProgress(done, total)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return "", rerr
		}
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// extractBinaries 从 zip 中取出 ffmpeg(.exe) 与 ffprobe(.exe) 写入 dir。
func extractBinaries(zipPath, dir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	targets := map[string]bool{exeName("ffmpeg"): true, exeName("ffprobe"): true}
	found := 0
	for _, zf := range zr.File {
		base := filepath.Base(zf.Name)
		if !targets[base] {
			continue
		}
		if err := extractOne(zf, filepath.Join(dir, base)); err != nil {
			return err
		}
		found++
	}
	if found < len(targets) {
		return fmt.Errorf("压缩包中缺少 ffmpeg/ffprobe")
	}
	return nil
}

func extractOne(zf *zip.File, dst string) error {
	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	tmp := dst + ".part"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}
