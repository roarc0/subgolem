package transcribe

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"

	"github.com/roarc0/subgolem/internal/segment"
)

// WhisperTranscriber transcribes audio using a local whisper.cpp model.
type WhisperTranscriber struct {
	model whisper.Model
}

// NewWhisperTranscriber loads the model at modelPath.
// modelPath should point to a ggml .bin file (e.g. data/models/ggml-large-v3.bin).
func NewWhisperTranscriber(modelPath string) (*WhisperTranscriber, error) {
	m, err := whisper.New(modelPath)
	if err != nil {
		return nil, fmt.Errorf("load whisper model %s: %w", modelPath, err)
	}
	return &WhisperTranscriber{model: m}, nil
}

// Close releases model resources.
func (w *WhisperTranscriber) Close() error {
	return w.model.Close()
}

// Transcribe processes the f32le PCM file at pcmPath.
// lang: BCP-47 language code ("he", "en", …) or "auto" for detection.
// translate: when true, whisper natively outputs English regardless of source language.
func (w *WhisperTranscriber) Transcribe(_ context.Context, pcmPath string, lang string, translate bool) ([]segment.Segment, error) {
	samples, err := readF32LE(pcmPath)
	if err != nil {
		return nil, fmt.Errorf("read PCM: %w", err)
	}

	ctx, err := w.model.NewContext()
	if err != nil {
		return nil, fmt.Errorf("new whisper context: %w", err)
	}

	if lang != "" && lang != "auto" {
		if err := ctx.SetLanguage(lang); err != nil {
			return nil, fmt.Errorf("set language %q: %w", lang, err)
		}
	}
	ctx.SetTranslate(translate)

	if err := ctx.Process(samples, nil, nil); err != nil {
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
