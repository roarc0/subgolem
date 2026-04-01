package translate_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"github.com/roarc0/subgolem/internal/segment"
	"github.com/roarc0/subgolem/internal/translate"
)

func TestOpenAITranslator_Translate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openai.ChatCompletionResponse{
			Choices: []openai.ChatCompletionChoice{
				{Message: openai.ChatCompletionMessage{Content: "Hello world\nGoodbye world"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tr := translate.NewOpenAITranslator(translate.OpenAIConfig{
		BaseURL: srv.URL + "/v1",
		APIKey:  "test",
		Model:   "gpt-4o-mini",
	})

	input := []segment.Segment{
		{Start: 0, End: time.Second, Text: "שלום עולם"},
		{Start: time.Second, End: 2 * time.Second, Text: "להתראות עולם"},
	}
	got, err := tr.Translate(context.Background(), input, "he")
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d segments, want 2", len(got))
	}
	if got[0].Text != "Hello world" {
		t.Errorf("seg 0 text: got %q, want %q", got[0].Text, "Hello world")
	}
	if got[1].Text != "Goodbye world" {
		t.Errorf("seg 1 text: got %q, want %q", got[1].Text, "Goodbye world")
	}
	if got[0].Start != input[0].Start || got[0].End != input[0].End {
		t.Error("timing not preserved in segment 0")
	}
}

func TestOpenAITranslator_Empty(t *testing.T) {
	tr := translate.NewOpenAITranslator(translate.OpenAIConfig{
		BaseURL: "http://unused",
		APIKey:  "test",
		Model:   "gpt-4o-mini",
	})
	got, err := tr.Translate(context.Background(), nil, "he")
	if err != nil {
		t.Fatalf("Translate(nil): %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty result, got %d segments", len(got))
	}
}
