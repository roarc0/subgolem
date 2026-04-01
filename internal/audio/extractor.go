package audio

import (
	"context"
	"fmt"
	"os/exec"
)

// Extractor extracts audio from video/audio files using FFmpeg.
type Extractor struct {
	audioFilter bool
}

// NewExtractor creates an extractor. When audioFilter is true, FFmpeg applies
// loudness normalisation and a speech bandpass filter (200–3000 Hz) before
// writing the PCM file — improves whisper accuracy on noisy or quiet sources.
func NewExtractor(audioFilter bool) *Extractor { return &Extractor{audioFilter: audioFilter} }

// Extract extracts 16kHz mono float32 PCM from inputPath to outputPath.
// outputPath is a raw f32le file (no header) — pass directly to whisper.cpp.
func (e *Extractor) Extract(ctx context.Context, inputPath, outputPath string) error {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg not found in PATH — install with: sudo emerge media-video/ffmpeg")
	}

	args := []string{"-i", inputPath, "-vn", "-ar", "16000", "-ac", "1"}
	if e.audioFilter {
		args = append(args, "-af", "loudnorm,highpass=f=200,lowpass=f=3000")
	}
	args = append(args, "-f", "f32le", "-y", outputPath)

	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg: %w\n%s", err, string(out))
	}
	return nil
}
