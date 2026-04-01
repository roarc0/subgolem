package audio_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/roarc0/subgolem/internal/audio"
)

func TestExtractor_Extract(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not in PATH")
	}

	// Generate 1s of silence as a test source
	src := filepath.Join(t.TempDir(), "silence.wav")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "anullsrc=r=44100:cl=mono",
		"-t", "1", "-y", src)
	if err := cmd.Run(); err != nil {
		t.Fatalf("create test audio: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "out.pcm")
	e := audio.NewExtractor()
	if err := e.Extract(context.Background(), src, dst); err != nil {
		t.Fatalf("Extract: %v", err)
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("output missing: %v", err)
	}
	// 1s at 16kHz mono f32le = 16000 * 4 = 64000 bytes
	if info.Size() < 60000 {
		t.Errorf("output too small: %d bytes (want ≥60000)", info.Size())
	}
}
