package translate

import (
	"context"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"

	"github.com/roarc0/subgolem/internal/segment"
)

// OpenAIConfig holds settings for an OpenAI-compatible endpoint.
// BaseURL accepts OpenAI, LM Studio (http://localhost:1234/v1), or any compatible server.
type OpenAIConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

// OpenAITranslator translates segments via a single /v1/chat/completions call.
type OpenAITranslator struct {
	client *openai.Client
	model  string
}

func NewOpenAITranslator(cfg OpenAIConfig) *OpenAITranslator {
	c := openai.DefaultConfig(cfg.APIKey)
	c.BaseURL = cfg.BaseURL
	return &OpenAITranslator{
		client: openai.NewClientWithConfig(c),
		model:  cfg.Model,
	}
}

// Translate sends all segments in one API call and maps translated lines back.
// Timing is always preserved from input.
func (t *OpenAITranslator) Translate(ctx context.Context, segs []segment.Segment, sourceLang string) ([]segment.Segment, error) {
	if len(segs) == 0 {
		return segs, nil
	}

	lines := make([]string, len(segs))
	for i, s := range segs {
		lines[i] = s.Text
	}

	system := fmt.Sprintf(
		"You are a professional subtitle translator. Translate the following subtitle lines from %s to English. "+
			"Return ONLY the translated lines, one per line, same order. No numbering, no extra text.",
		sourceLang,
	)

	resp, err := t.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: t.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: system},
			{Role: openai.ChatMessageRoleUser, Content: strings.Join(lines, "\n")},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("openai translate: %w", err)
	}

	translated := strings.Split(strings.TrimSpace(resp.Choices[0].Message.Content), "\n")
	if len(translated) != len(segs) {
		return nil, fmt.Errorf("translation returned %d lines, expected %d", len(translated), len(segs))
	}

	result := make([]segment.Segment, len(segs))
	for i, s := range segs {
		result[i] = segment.Segment{
			Start: s.Start,
			End:   s.End,
			Text:  strings.TrimSpace(translated[i]),
		}
	}
	return result, nil
}
