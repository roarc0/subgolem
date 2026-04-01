package models_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/roarc0/subgolem/internal/models"
)

func TestManager_ModelPath(t *testing.T) {
	m := models.NewManager(t.TempDir())
	got := m.ModelPath("large-v3")
	if filepath.Base(got) != "ggml-large-v3.bin" {
		t.Errorf("ModelPath(large-v3) = %q, want base ggml-large-v3.bin", got)
	}
}

func TestManager_ModelPath_Unknown(t *testing.T) {
	m := models.NewManager(t.TempDir())
	if m.ModelPath("nonexistent") != "" {
		t.Error("ModelPath for unknown model should return empty string")
	}
}

func TestManager_IsDownloaded(t *testing.T) {
	dir := t.TempDir()
	m := models.NewManager(dir)

	if m.IsDownloaded("large-v3") {
		t.Error("expected false before file exists")
	}

	modelsDir := filepath.Join(dir, "models")
	os.MkdirAll(modelsDir, 0755)
	os.WriteFile(filepath.Join(modelsDir, "ggml-large-v3.bin"), []byte("dummy"), 0644)

	if !m.IsDownloaded("large-v3") {
		t.Error("expected true after file created")
	}
}

func TestManager_KnownModels(t *testing.T) {
	m := models.NewManager(t.TempDir())
	for _, name := range []string{"tiny", "base", "small", "medium", "large-v3"} {
		if m.ModelPath(name) == "" {
			t.Errorf("ModelPath(%q) = empty string, want a path", name)
		}
	}
}
