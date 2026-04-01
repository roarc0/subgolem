package models

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const huggingFaceBase = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/"

var knownModels = map[string]string{
	"tiny":     "ggml-tiny.bin",
	"base":     "ggml-base.bin",
	"small":    "ggml-small.bin",
	"medium":   "ggml-medium.bin",
	"large-v3": "ggml-large-v3.bin",
}

// Manager handles whisper model lifecycle: path resolution and downloading.
type Manager struct {
	dataDir string
}

func NewManager(dataDir string) *Manager { return &Manager{dataDir: dataDir} }

func (m *Manager) DataDir() string { return m.dataDir }

// ModelPath returns the absolute path for a known model, or "" if unknown.
func (m *Manager) ModelPath(model string) string {
	filename, ok := knownModels[model]
	if !ok {
		return ""
	}
	return filepath.Join(m.dataDir, "models", filename)
}

// IsDownloaded reports whether the model file exists and is non-empty.
func (m *Manager) IsDownloaded(model string) bool {
	p := m.ModelPath(model)
	if p == "" {
		return false
	}
	info, err := os.Stat(p)
	return err == nil && info.Size() > 0
}

// EnsureDownloaded downloads the model if not already present.
// onProgress is called with (bytesWritten, totalBytes) during download; pass nil to silence it.
func (m *Manager) EnsureDownloaded(ctx context.Context, model string, onProgress func(done, total int64)) error {
	if m.IsDownloaded(model) {
		return nil
	}
	filename, ok := knownModels[model]
	if !ok {
		return fmt.Errorf("unknown model %q — valid: tiny, base, small, medium, large-v3", model)
	}

	dest := m.ModelPath(model)
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("create models dir: %w", err)
	}

	url := huggingFaceBase + filename

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", model, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", model, resp.StatusCode)
	}

	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer func() {
		f.Close()
		os.Remove(tmp)
	}()

	var dst io.Writer = f
	if onProgress != nil {
		dst = io.MultiWriter(f, &progressWriter{
			total:    resp.ContentLength,
			callback: onProgress,
		})
	}

	if _, err := io.Copy(dst, resp.Body); err != nil {
		return fmt.Errorf("write model: %w", err)
	}
	f.Close()

	if err := os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("finalize model: %w", err)
	}
	return nil
}

// progressWriter reports download progress via a callback.
type progressWriter struct {
	done     int64
	total    int64
	callback func(done, total int64)
}

func (pw *progressWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	pw.done += int64(n)
	pw.callback(pw.done, pw.total)
	return n, nil
}
