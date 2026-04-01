package transcribe

import (
	"context"

	"github.com/roarc0/subgolem/internal/segment"
)

// Transcriber transcribes a raw f32le PCM file into timed segments.
// When translate=true, whisper.cpp translates to English natively (no extra API call needed).
// When translate=false, segments are returned in the source language.
type Transcriber interface {
	Transcribe(ctx context.Context, pcmPath string, lang string, translate bool) ([]segment.Segment, error)
}
