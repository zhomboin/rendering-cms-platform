package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	defaultLogPrefix     = "backend"
	defaultRetentionDays = 30
	dateLayout           = "2006-01-02"
)

type dailyFileWriter struct {
	mu            sync.Mutex
	dir           string
	prefix        string
	retentionDays int
	now           func() time.Time
	date          string
	current       *os.File
}

func NewDailyFileLogger(logDir string, retentionDays int) (*slog.Logger, io.Closer) {
	writer := newDailyFileWriter(logDir, defaultLogPrefix, time.Now, retentionDays)
	return slog.New(slog.NewJSONHandler(writer, nil)), writer
}

func newDailyFileWriter(logDir string, prefix string, now func() time.Time, retentionDays int) *dailyFileWriter {
	if strings.TrimSpace(logDir) == "" {
		logDir = "logs"
	}
	if strings.TrimSpace(prefix) == "" {
		prefix = defaultLogPrefix
	}
	if now == nil {
		now = time.Now
	}
	if retentionDays <= 0 {
		retentionDays = defaultRetentionDays
	}
	return &dailyFileWriter{
		dir:           logDir,
		prefix:        prefix,
		retentionDays: retentionDays,
		now:           now,
	}
}

func (writer *dailyFileWriter) Write(payload []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()

	if err := writer.rotateLocked(); err != nil {
		return len(payload), nil
	}
	if _, err := writer.current.Write(payload); err != nil {
		_ = writer.current.Close()
		writer.current = nil
		writer.date = ""
		return len(payload), nil
	}
	return len(payload), nil
}

func (writer *dailyFileWriter) Close() error {
	writer.mu.Lock()
	defer writer.mu.Unlock()

	if writer.current == nil {
		return nil
	}
	err := writer.current.Close()
	writer.current = nil
	writer.date = ""
	return err
}

func (writer *dailyFileWriter) rotateLocked() error {
	date := writer.now().Format("2006-01-02")
	if writer.current != nil && writer.date == date {
		return nil
	}

	if writer.current != nil {
		_ = writer.current.Close()
		writer.current = nil
		writer.date = ""
	}
	if err := os.MkdirAll(writer.dir, 0o755); err != nil {
		return err
	}

	logPath := filepath.Join(writer.dir, writer.prefix+"-"+date+".log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	writer.current = file
	writer.date = date
	writer.pruneLocked()
	return nil
}

// pruneLocked 删除文件名日期早于保留窗口的按日日志。只处理严格匹配
// prefix-YYYY-MM-DD.log 的文件，其余内容一律不动；删除失败静默忽略，
// 与 Write 的失败策略一致（日志不影响主业务链路）。
func (writer *dailyFileWriter) pruneLocked() {
	entries, err := os.ReadDir(writer.dir)
	if err != nil {
		return
	}
	prefix := writer.prefix + "-"
	cutoff := writer.now().AddDate(0, 0, -writer.retentionDays).Format(dateLayout)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".log") {
			continue
		}
		date := strings.TrimSuffix(strings.TrimPrefix(name, prefix), ".log")
		if _, err := time.Parse(dateLayout, date); err != nil {
			continue
		}
		if date >= cutoff {
			continue
		}
		_ = os.Remove(filepath.Join(writer.dir, name))
	}
}
