package audio

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Extractor extracts audio from video/audio files using FFmpeg.
type Extractor struct {
	audioFilter bool
}

// NewExtractor creates an extractor. When audioFilter is true, FFmpeg applies
// loudness normalisation and a speech bandpass filter (200–3000 Hz).
func NewExtractor(audioFilter bool) *Extractor { return &Extractor{audioFilter: audioFilter} }

// Extract extracts 16kHz mono float32 PCM from inputPath to outputPath.
// onProgress is called with (done, total) duration during extraction; pass nil to silence it.
func (e *Extractor) Extract(ctx context.Context, inputPath, outputPath string, onProgress func(done, total time.Duration)) error {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg not found in PATH — install with: sudo emerge media-video/ffmpeg")
	}

	total := probeDuration(ctx, inputPath)

	args := []string{"-i", inputPath, "-vn", "-ar", "16000", "-ac", "1"}
	if e.audioFilter {
		args = append(args, "-af", "loudnorm,highpass=f=200,lowpass=f=3000")
	}
	if onProgress != nil {
		args = append(args, "-progress", "pipe:1", "-nostats")
	}
	args = append(args, "-f", "f32le", "-y", outputPath)

	cmd := exec.CommandContext(ctx, ffmpeg, args...)

	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if onProgress != nil {
		pr, pw := io.Pipe()
		cmd.Stdout = pw
		go func() {
			scanner := bufio.NewScanner(pr)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "out_time_us=") {
					us, err := strconv.ParseInt(strings.TrimPrefix(line, "out_time_us="), 10, 64)
					if err == nil && us > 0 {
						onProgress(time.Duration(us)*time.Microsecond, total)
					}
				}
			}
		}()
		err = cmd.Run()
		pw.Close()
	} else {
		err = cmd.Run()
	}

	if err != nil {
		return fmt.Errorf("ffmpeg: %w\n%s", err, errBuf.String())
	}
	return nil
}

// probeDuration returns the media duration via ffprobe, or 0 if unavailable.
func probeDuration(ctx context.Context, inputPath string) time.Duration {
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		return 0
	}
	out, err := exec.CommandContext(ctx, ffprobe,
		"-v", "quiet",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		inputPath,
	).Output()
	if err != nil {
		return 0
	}
	secs, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0
	}
	return time.Duration(secs * float64(time.Second))
}
