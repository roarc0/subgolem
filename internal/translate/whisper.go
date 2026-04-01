package translate

import (
	"context"

	"github.com/roarc0/subgolem/internal/segment"
)

// WhisperTranslator is a no-op: whisper.cpp already translated during transcription.
type WhisperTranslator struct{}

func NewWhisperTranslator() *WhisperTranslator { return &WhisperTranslator{} }

func (t *WhisperTranslator) Translate(_ context.Context, segs []segment.Segment, _ string) ([]segment.Segment, error) {
	return segs, nil
}
