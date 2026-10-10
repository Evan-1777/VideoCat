package ffmpeg

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// ProgressFunc 报告 ffmpeg 已处理的输出时间（秒）。done 为 true 时表示本阶段结束，秒数无意义。
type ProgressFunc func(outSeconds float64, done bool)

// tailWriter 保留最近 limit 字节，用于错误诊断。
type tailWriter struct {
	mu    sync.Mutex
	buf   []byte
	limit int
}

func (w *tailWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	if len(w.buf) > w.limit {
		w.buf = w.buf[len(w.buf)-w.limit:]
	}
	return len(p), nil
}

func (w *tailWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return string(w.buf)
}

// Run 执行 ffmpeg 并解析 -progress pipe:1 输出。
// args 须包含 -progress pipe:1 -nostats。返回 stderr 尾部与错误（取消时返回 ctx.Err()）。
func Run(ctx context.Context, exe string, args []string, onProgress ProgressFunc) (string, error) {
	cmd := exec.CommandContext(ctx, exe, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	tail := &tailWriter{limit: 8192}
	cmd.Stderr = tail
	if err := cmd.Start(); err != nil {
		return "", err
	}

	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if v, ok := strings.CutPrefix(line, "out_time_us="); ok {
			if us, e := strconv.ParseInt(v, 10, 64); e == nil && onProgress != nil {
				onProgress(float64(us)/1e6, false)
			}
		} else if line == "progress=end" {
			if onProgress != nil {
				onProgress(0, true)
			}
		}
	}

	werr := cmd.Wait()
	if ctx.Err() != nil {
		return tail.String(), ctx.Err()
	}
	if werr != nil {
		return tail.String(), fmt.Errorf("ffmpeg 退出异常：%v", werr)
	}
	return tail.String(), nil
}
