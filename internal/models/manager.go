package models

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func logProgress(msg string) {
	f, err := os.OpenFile("data/subgolem.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "[%s] %s\n", time.Now().Format("15:04:05"), msg)
}

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
	dataDir    string
	customURLs map[string]string
}

func NewManager(dataDir string, customURLs map[string]string) *Manager {
	return &Manager{
		dataDir:    dataDir,
		customURLs: customURLs,
	}
}

func (m *Manager) DataDir() string { return m.dataDir }

// ModelPath returns the absolute path for a model.
// 1. If 'model' is an existing local file, returns it as-is.
// 2. If 'model' is a known name or custom alias, returns path in dataDir/models.
// 3. Otherwise returns "".
func (m *Manager) ModelPath(model string) string {
	// If it's already a valid local file path, use it
	if info, err := os.Stat(model); err == nil && !info.IsDir() {
		return model
	}

	var filename string
	if f, ok := knownModels[model]; ok {
		filename = f
	} else if _, ok := m.customURLs[model]; ok {
		// If it's a custom URL, use the model key as the filename basis.
		// This keeps files organized and avoids clashing base names like 'ggml-model.bin'.
		filename = model
		if filepath.Ext(filename) != ".bin" {
			filename += ".bin"
		}
	}

	if filename == "" {
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
	// If it's a local file path that already exists, we're done
	if m.IsDownloaded(model) {
		return nil
	}

	var url string
	if u, ok := m.customURLs[model]; ok {
		url = u
	} else if filename, ok := knownModels[model]; ok {
		url = huggingFaceBase + filename
	}

	if url == "" {
		return fmt.Errorf("unknown model %q — define it in 'whisper_models' config or provide absolute path to .bin file", model)
	}

	dest := m.ModelPath(model)
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("create models dir: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	client := &http.Client{
		Timeout: 30 * time.Minute, // Large for slow downloads, but prevents infinite hangs
	}
	logProgress(fmt.Sprintf("Starting request to %s", url))
	resp, err := client.Do(req)
	if err != nil {
		logProgress(fmt.Sprintf("Request failed: %v", err))
		return fmt.Errorf("download %s: %w", model, err)
	}
	defer resp.Body.Close()
	logProgress(fmt.Sprintf("HTTP %d, Content-Length: %d", resp.StatusCode, resp.ContentLength))

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

	logProgress("Starting transfer...")
	n, err := io.Copy(dst, resp.Body)
	if err != nil {
		logProgress(fmt.Sprintf("Transfer failed after %d bytes: %v", n, err))
		return fmt.Errorf("write model: %w", err)
	}
	logProgress(fmt.Sprintf("Transfer complete: %d bytes", n))
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
