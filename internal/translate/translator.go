package translate

import (
	"context"

	"github.com/roarc0/subgolem/internal/segment"
)

// Translator translates subtitle segments to English.
// The whisper backend is a no-op: whisper.cpp handles translation during transcription.
type Translator interface {
	Translate(ctx context.Context, segments []segment.Segment, sourceLang string) ([]segment.Segment, error)
}
