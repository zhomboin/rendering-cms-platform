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
	})
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
	})
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
	writer := newDailyFileWriter(logDir, "backend", time.Now)
	defer writer.Close()

	n, err := writer.Write([]byte("request log\n"))
	if err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}
	if n != len("request log\n") {
		t.Fatalf("Write() bytes = %d, want %d", n, len("request log\n"))
	}
}
