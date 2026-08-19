package convert

import (
	"Argus/schema"
	"encoding/json"
	"fmt"
)

type Converter interface {
	Name() string

	// ToNative translates a unified request into the provider's native
	// request shape, ready to be marshaled and sent upstream.
	ToNative(req schema.ChatCompletionRequest) (any, error)

	// FromNative parses a provider's raw native response body and
	// translates it into Argus's unified response shape.
	FromNative(body []byte) (schema.ChatCompletionResponse, error)
}

type AnthropicConverter struct{}

func (AnthropicConverter) Name() string { return "anthropic" }

func (AnthropicConverter) ToNative(req schema.ChatCompletionRequest) (any, error) {
	native := schema.AnthropicRequest{
		Model:     req.Model,
		MaxTokens: 1024, // Anthropic requires this field; wire up a real default/config value
	}

	for _, m := range req.Messages {
		if m.Role == "system" {
			native.System = m.Content // Anthropic has no "system" role — it's a top-level field
			continue
		}
		native.Messages = append(native.Messages, schema.AnthropicMessage{
			Role:    m.Role, // "user" / "assistant" pass through unchanged
			Content: []schema.AnthropicContentBlock{{Type: "text", Text: m.Content}},
		})
	}
	return native, nil
}

func (AnthropicConverter) FromNative(body []byte) (schema.ChatCompletionResponse, error) {
	var native schema.AnthropicResponse
	if err := json.Unmarshal(body, &native); err != nil {
		return schema.ChatCompletionResponse{}, fmt.Errorf("anthropic: decode response: %w", err)
	}

	text := ""
	for _, block := range native.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}

	return schema.ChatCompletionResponse{
		ID:    native.ID,
		Model: native.Model,
		Choices: []schema.Choice{{
			Index:        0,
			Message:      schema.Message{Role: "assistant", Content: text},
			FinishReason: mapAnthropicFinishReason(native.StopReason),
		}},
		Usage: schema.Usage{
			PromptTokens:     native.Usage.InputTokens,
			CompletionTokens: native.Usage.OutputTokens,
			TotalTokens:      native.Usage.InputTokens + native.Usage.OutputTokens,
		},
	}, nil
}

func mapAnthropicFinishReason(r string) string {
	switch r {
	case "end_turn", "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	default:
		return r
	}
}

// ---------- Gemini ----------

type GeminiConverter struct{}

func (GeminiConverter) Name() string { return "gemini" }

func (GeminiConverter) ToNative(req schema.ChatCompletionRequest) (any, error) {
	native := schema.GeminiRequest{}

	for _, m := range req.Messages {
		if m.Role == "system" {
			native.SystemInstruction = &schema.GeminiContent{
				Parts: []schema.GeminiPart{{Text: m.Content}},
			}
			continue
		}
		role := m.Role
		if role == "assistant" {
			role = "model" // Gemini calls the assistant turn "model"
		}
		native.Contents = append(native.Contents, schema.GeminiContent{
			Role:  role,
			Parts: []schema.GeminiPart{{Text: m.Content}},
		})
	}
	return native, nil
}

func (GeminiConverter) FromNative(body []byte) (schema.ChatCompletionResponse, error) {
	var native schema.GeminiResponse
	if err := json.Unmarshal(body, &native); err != nil {
		return schema.ChatCompletionResponse{}, fmt.Errorf("gemini: decode response: %w", err)
	}
	if len(native.Candidates) == 0 {
		return schema.ChatCompletionResponse{}, fmt.Errorf("gemini: response had no candidates")
	}

	text := ""
	for _, part := range native.Candidates[0].Content.Parts {
		text += part.Text
	}

	return schema.ChatCompletionResponse{
		Choices: []schema.Choice{{
			Index:        native.Candidates[0].Index,
			Message:      schema.Message{Role: "assistant", Content: text},
			FinishReason: mapGeminiFinishReason(native.Candidates[0].FinishReason),
		}},
		Usage: schema.Usage{
			PromptTokens:     native.UsageMetadata.PromptTokenCount,
			CompletionTokens: native.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      native.UsageMetadata.PromptTokenCount + native.UsageMetadata.CandidatesTokenCount,
		},
	}, nil
}

func mapGeminiFinishReason(r string) string {
	switch r {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	default:
		return r
	}
}

var Registry = map[string]Converter{
	"anthropic": AnthropicConverter{},
	"gemini":    GeminiConverter{},
}
