package translate_test

import (
	"context"
	"testing"
	"time"

	"github.com/roarc0/subgolem/internal/segment"
	"github.com/roarc0/subgolem/internal/translate"
)

func TestWhisperTranslator_Passthrough(t *testing.T) {
	tr := translate.NewWhisperTranslator()
	input := []segment.Segment{
		{Start: 0, End: time.Second, Text: "שלום עולם"},
		{Start: time.Second, End: 2 * time.Second, Text: "Hello World"},
	}
	got, err := tr.Translate(context.Background(), input, "he")
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if len(got) != len(input) {
		t.Fatalf("got %d segments, want %d", len(got), len(input))
	}
	for i := range got {
		if got[i] != input[i] {
			t.Errorf("segment %d changed: got %+v, want %+v", i, got[i], input[i])
		}
	}
}
