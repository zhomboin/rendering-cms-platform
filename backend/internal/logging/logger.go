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

const defaultLogPrefix = "backend"

type dailyFileWriter struct {
	mu      sync.Mutex
	dir     string
	prefix  string
	now     func() time.Time
	date    string
	current *os.File
}

func NewDailyFileLogger(logDir string) (*slog.Logger, io.Closer) {
	writer := newDailyFileWriter(logDir, defaultLogPrefix, time.Now)
	return slog.New(slog.NewJSONHandler(writer, nil)), writer
}

func newDailyFileWriter(logDir string, prefix string, now func() time.Time) *dailyFileWriter {
	if strings.TrimSpace(logDir) == "" {
		logDir = "logs"
	}
	if strings.TrimSpace(prefix) == "" {
		prefix = defaultLogPrefix
	}
	if now == nil {
		now = time.Now
	}
	return &dailyFileWriter{
		dir:    logDir,
		prefix: prefix,
		now:    now,
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
	return nil
}
