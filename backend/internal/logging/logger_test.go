package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDailyFileLoggerWritesJSONLogToCurrentDateFile(t *testing.T) {
	logDir := t.TempDir()
	now := time.Date(2026, 5, 5, 10, 30, 0, 0, time.UTC)
	writer := newDailyFileWriter(logDir, "backend", func() time.Time {
		return now
	}, 30)
	defer writer.Close()

	logger := slog.New(slog.NewJSONHandler(writer, nil))
	logger.Info("http_request", "method", "GET", "path", "/api/v1/health", "status", 200)

	logFile := filepath.Join(logDir, "backend-2026-05-05.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	line := string(content)
	for _, want := range []string{
		`"msg":"http_request"`,
		`"method":"GET"`,
		`"path":"/api/v1/health"`,
		`"status":200`,
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("log file content %q does not contain %s", line, want)
		}
	}
}

func TestDailyFileLoggerRotatesFilesWhenDateChanges(t *testing.T) {
	logDir := t.TempDir()
	now := time.Date(2026, 5, 5, 23, 59, 0, 0, time.UTC)
	writer := newDailyFileWriter(logDir, "backend", func() time.Time {
		return now
	}, 30)
	defer writer.Close()

	logger := slog.New(slog.NewJSONHandler(writer, nil))
	logger.Info("http_request", "path", "/api/v1/health")

	now = now.Add(2 * time.Minute)
	logger.Info("http_request", "path", "/api/v1/articles")

	firstDay, err := os.ReadFile(filepath.Join(logDir, "backend-2026-05-05.log"))
	if err != nil {
		t.Fatalf("read first day log file: %v", err)
	}
	nextDay, err := os.ReadFile(filepath.Join(logDir, "backend-2026-05-06.log"))
	if err != nil {
		t.Fatalf("read next day log file: %v", err)
	}
	if !strings.Contains(string(firstDay), `"/api/v1/health"`) {
		t.Fatalf("first day log file content = %q, want health request", firstDay)
	}
	if !strings.Contains(string(nextDay), `"/api/v1/articles"`) {
		t.Fatalf("next day log file content = %q, want articles request", nextDay)
	}
}

func TestDailyFileWriterIgnoresOpenErrors(t *testing.T) {
	logDir := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(logDir, []byte("occupied"), 0o644); err != nil {
		t.Fatalf("prepare occupied path: %v", err)
	}
	writer := newDailyFileWriter(logDir, "backend", time.Now, 30)
	defer writer.Close()

	n, err := writer.Write([]byte("request log\n"))
	if err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}
	if n != len("request log\n") {
		t.Fatalf("Write() bytes = %d, want %d", n, len("request log\n"))
	}
}

func TestDailyFileWriterPrunesExpiredLogs(t *testing.T) {
	logDir := t.TempDir()
	now := time.Date(2026, 5, 5, 0, 5, 0, 0, time.UTC)
	for _, name := range []string{
		"backend-2026-03-30.log", // 早于保留窗口，应删除
		"backend-2026-04-10.log", // 保留窗口内，应保留
		"backend-not-a-date.log", // 命名不符合按日规则，不应被删除
	} {
		if err := os.WriteFile(filepath.Join(logDir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("prepare %s: %v", name, err)
		}
	}
	writer := newDailyFileWriter(logDir, "backend", func() time.Time {
		return now
	}, 30)
	defer writer.Close()

	if _, err := writer.Write([]byte("request log\n")); err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(logDir, "backend-2026-03-30.log")); !os.IsNotExist(err) {
		t.Fatalf("expired log should be removed, stat err = %v", err)
	}
	for _, name := range []string{"backend-2026-04-10.log", "backend-not-a-date.log", "backend-2026-05-05.log"} {
		if _, err := os.Stat(filepath.Join(logDir, name)); err != nil {
			t.Fatalf("log %s should remain: %v", name, err)
		}
	}
}

func TestDailyFileWriterKeepsAllLogsWithLargeRetention(t *testing.T) {
	logDir := t.TempDir()
	old := filepath.Join(logDir, "backend-2020-01-01.log")
	if err := os.WriteFile(old, []byte("x"), 0o644); err != nil {
		t.Fatalf("prepare old log: %v", err)
	}
	writer := newDailyFileWriter(logDir, "backend", time.Now, 36500)
	defer writer.Close()

	if _, err := writer.Write([]byte("request log\n")); err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}
	if _, err := os.Stat(old); err != nil {
		t.Fatalf("old log should remain under large retention: %v", err)
	}
}
