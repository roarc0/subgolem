package audio

import (
	"context"
	"fmt"
	"os/exec"
)

// Extractor extracts audio from video/audio files using FFmpeg.
type Extractor struct{}

func NewExtractor() *Extractor { return &Extractor{} }

// Extract extracts 16kHz mono float32 PCM from inputPath to outputPath.
// outputPath is a raw f32le file (no header) — pass directly to whisper.cpp.
func (e *Extractor) Extract(ctx context.Context, inputPath, outputPath string) error {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg not found in PATH — install with: sudo emerge media-video/ffmpeg")
	}
	cmd := exec.CommandContext(ctx, ffmpeg,
		"-i", inputPath,
		"-vn",          // strip video
		"-ar", "16000", // 16 kHz required by whisper
		"-ac", "1",     // mono
		"-f", "f32le",  // raw 32-bit float little-endian PCM
		"-y",           // overwrite output
		outputPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg: %w\n%s", err, string(out))
	}
	return nil
}
