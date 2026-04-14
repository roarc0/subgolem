package refine

import (
	"context"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/roarc0/subgolem/internal/segment"
)

// RefineConfig holds the parameters for connecting to the OpenAI/Ollama refiner API.
type RefineConfig struct {
	BaseURL string
	APIKey  string
	Model   string
	Prompt  string
	Chunk   int
}

// LlmRefiner formats Whisper segment objects into raw SRT chunks and passes them
// to an LLM for formatting, spelling, and phrase-merging improvements.
type LlmRefiner struct {
	client *openai.Client
	cfg    RefineConfig
}

// NewLlmRefiner initializes a new instance for subtitle refinement.
func NewLlmRefiner(cfg RefineConfig) *LlmRefiner {
	c := openai.DefaultConfig(cfg.APIKey)
	c.BaseURL = cfg.BaseURL
	return &LlmRefiner{
		client: openai.NewClientWithConfig(c),
		cfg:    cfg,
	}
}

// Refine batches segments into SRT text chunks, sends them to the LLM, and
// aggregates the fully refined text back together into a single string.
func (r *LlmRefiner) Refine(ctx context.Context, segs []segment.Segment) (string, error) {
	if len(segs) == 0 {
		return "", nil
	}

	var finalOutput strings.Builder
	chunkSize := r.cfg.Chunk
	if chunkSize <= 0 {
		chunkSize = 40 // Default safe limit
	}

	for i := 0; i < len(segs); i += chunkSize {
		end := i + chunkSize
		if end > len(segs) {
			end = len(segs)
		}
		
		// 1. Serialize physical chunk into raw SRT format
		chunkStr := buildSrtChunk(segs[i:end], i+1)

		// 2. Call completion API
		resp, err := r.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model: r.cfg.Model,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: r.cfg.Prompt},
				{Role: openai.ChatMessageRoleUser, Content: chunkStr},
			},
		})
		if err != nil {
			return "", fmt.Errorf("llm refinement failed at chunk %d-%d: %w", i, end, err)
		}

		// 3. Append to output block
		block := strings.TrimSpace(resp.Choices[0].Message.Content)
		finalOutput.WriteString(block)
		finalOutput.WriteString("\n\n")
	}

	return strings.TrimSpace(finalOutput.String()) + "\n", nil
}

// buildSrtChunk generates strict SRT formatted text for a specific subset to pass explicitly to the LLM.
func buildSrtChunk(segs []segment.Segment, startIdx int) string {
	var sb strings.Builder
	for i, s := range segs {
		sb.WriteString(fmt.Sprintf("%d\n", startIdx+i))
		sb.WriteString(fmt.Sprintf("%s --> %s\n", formatSRTTime(s.Start.Seconds()), formatSRTTime(s.End.Seconds())))
		sb.WriteString(s.Text)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// formatSRTTime maps seconds directly into typical SRT presentation.
func formatSRTTime(seconds float64) string {
	h := int(seconds / 3600)
	m := int(seconds/60) % 60
	s := int(seconds) % 60
	ms := int((seconds - float64(int(seconds))) * 1000)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}
