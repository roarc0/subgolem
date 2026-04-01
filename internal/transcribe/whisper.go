package transcribe

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
	"strings"
	"syscall"

	"github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"

	"github.com/roarc0/subgolem/internal/segment"
)

// WhisperTranscriber transcribes audio using a local whisper.cpp model.
type WhisperTranscriber struct {
	model    whisper.Model
	beamSize int
	vad      bool
	prompt   string
}

// muteStderr redirects fd 2 to /dev/null and returns a restore function.
// This silences C-library output (whisper_model_load etc.) that bypasses Go's os.Stderr.
func muteStderr() func() {
	savedFd, err := syscall.Dup(2)
	if err != nil {
		return func() {}
	}
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		syscall.Close(savedFd)
		return func() {}
	}
	if err := syscall.Dup2(int(devNull.Fd()), 2); err != nil {
		syscall.Close(savedFd)
		devNull.Close()
		return func() {}
	}
	return func() {
		syscall.Dup2(savedFd, 2) //nolint:errcheck
		syscall.Close(savedFd)
		devNull.Close()
	}
}

// NewWhisperTranscriber loads the model at modelPath.
// beamSize controls beam search width (5 = default, 10 = better accuracy/slower).
// vad enables voice activity detection to strip silence before processing.
func NewWhisperTranscriber(modelPath string, beamSize int, vad bool, prompt string) (*WhisperTranscriber, error) {
	restore := muteStderr()
	m, err := whisper.New(modelPath)
	restore()
	if err != nil {
		return nil, fmt.Errorf("load whisper model %s: %w", modelPath, err)
	}
	return &WhisperTranscriber{model: m, beamSize: beamSize, vad: vad, prompt: prompt}, nil
}

// Close releases model resources.
func (w *WhisperTranscriber) Close() error {
	return w.model.Close()
}

// Transcribe processes the f32le PCM file at pcmPath.
// lang: BCP-47 language code ("he", "en", …) or "auto" for detection.
// translate: when true, whisper natively outputs English regardless of source language.
// onProgress is called with a value in [0,1] as transcription advances; pass nil to silence it.
func (w *WhisperTranscriber) Transcribe(_ context.Context, pcmPath string, lang string, translate bool, onProgress func(float32)) ([]segment.Segment, error) {
	samples, err := readF32LE(pcmPath)
	if err != nil {
		return nil, fmt.Errorf("read PCM: %w", err)
	}

	ctx, err := w.model.NewContext()
	if err != nil {
		return nil, fmt.Errorf("new whisper context: %w", err)
	}

	ctx.SetThreads(uint(runtime.NumCPU()))
	ctx.SetTokenTimestamps(true)
	ctx.SetSplitOnWord(true)
	ctx.SetTemperature(0)
	ctx.SetTemperatureFallback(0)
	if w.beamSize > 0 {
		ctx.SetBeamSize(w.beamSize)
	}
	if w.vad {
		ctx.SetVAD(true)
	}
	if w.prompt != "" {
		ctx.SetInitialPrompt(w.prompt)
	}

	if lang != "" && lang != "auto" {
		if err := ctx.SetLanguage(lang); err != nil {
			return nil, fmt.Errorf("set language %q: %w", lang, err)
		}
	}
	ctx.SetTranslate(translate)

	restore := muteStderr()
	var progressCb whisper.ProgressCallback
	if onProgress != nil {
		progressCb = func(pct int) {
			onProgress(float32(pct) / 100)
		}
	}
	err = ctx.Process(samples, nil, nil, progressCb)
	restore()
	if err != nil {
		return nil, fmt.Errorf("whisper process: %w", err)
	}

	var segs []segment.Segment
	for {
		s, err := ctx.NextSegment()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("next segment: %w", err)
		}
		segs = append(segs, segment.Segment{
			Start: s.Start,
			End:   s.End,
			Text:  strings.TrimSpace(s.Text),
		})
	}
	return segs, nil
}

// readF32LE reads a raw 32-bit float little-endian PCM file into float32 samples.
// This is the format FFmpeg produces with -f f32le.
func readF32LE(path string) ([]float32, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data)%4 != 0 {
		return nil, fmt.Errorf("invalid PCM data: length %d is not a multiple of 4", len(data))
	}
	samples := make([]float32, len(data)/4)
	for i := range samples {
		bits := binary.LittleEndian.Uint32(data[i*4 : i*4+4])
		samples[i] = math.Float32frombits(bits)
	}
	return samples, nil
}
