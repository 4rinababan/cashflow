package receipts

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// maxCompletionTokens is generous on purpose: a reasoning-style model spends
// part of its token budget "thinking" before it emits any visible content, so
// a low cap truncates the answer mid-JSON rather than the reasoning. This is
// not theoretical — live-testing against Gemini (gemini-flash-latest) with
// 1500 reliably cut the JSON off after ~4 fields with finish_reason=length;
// raising it to 6000 (real usage on a 3-line-item receipt: ~3250 tokens of
// reasoning + answer combined) fixed it. Mirrors the same lesson already
// learned the hard way in this workspace's other OpenAI-compatible
// integration (Task Management's ChatAiAssistant).
const maxCompletionTokens = 6000

// extractionTemperature is low: reading a receipt correctly rewards
// consistency over creativity.
const extractionTemperature = 0.2

// OpenAIScanner calls an OpenAI-compatible chat-completions API (currently
// configured for Biznet Gio, model openai/gpt-oss-20b — see
// docs/adr/0002-ai-receipt-scanning.md for why, and the caveat about that
// model's vision support) with the receipt image and asks for a plain JSON
// reply, rather than relying on the API's tool/function-calling feature.
// That mirrors this same workspace's other OpenAI-compatible integration
// (TaskManagement.Infrastructure.Ai.ChatAiAssistant), which settled on plain
// prompted-JSON specifically because it is the more universally-supported
// convention across OpenAI-compatible providers — tool-calling support is
// inconsistent, prompted JSON always works.
type OpenAIScanner struct {
	endpoint   string // base URL, e.g. "https://api.biznetgio.ai/v1"; "/chat/completions" is appended
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewOpenAIScanner builds a scanner against endpoint (trailing slash
// optional) for model, using httpClient if non-nil (a sensible default with
// a timeout is used otherwise). apiKey may be empty in an environment where
// AI scanning hasn't been provisioned yet — Scan then returns
// ErrNotConfigured without making a network call.
func NewOpenAIScanner(endpoint, apiKey, model string, httpClient *http.Client) *OpenAIScanner {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 45 * time.Second}
	}
	return &OpenAIScanner{
		endpoint:   strings.TrimSuffix(endpoint, "/"),
		apiKey:     apiKey,
		model:      model,
		httpClient: httpClient,
	}
}

func (c *OpenAIScanner) Scan(ctx context.Context, image []byte, mimeType string) (Draft, error) {
	if c.apiKey == "" {
		return Draft{}, ErrNotConfigured
	}

	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(image))
	reqBody := chatCompletionsRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: receiptSystemPrompt},
			{Role: "user", Content: []chatContentPart{
				{Type: "text", Text: "Extract this receipt's fields as instructed."},
				{Type: "image_url", ImageURL: &chatImageURL{URL: dataURL}},
			}},
		},
		MaxTokens:   maxCompletionTokens,
		Temperature: extractionTemperature,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return Draft{}, fmt.Errorf("marshal chat completions request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return Draft{}, fmt.Errorf("build chat completions request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Draft{}, fmt.Errorf("call AI endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Draft{}, fmt.Errorf("read AI endpoint response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return Draft{}, fmt.Errorf("AI endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	var parsed chatCompletionsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Draft{}, fmt.Errorf("parse AI endpoint response: %w", err)
	}

	choice := firstChoice(parsed)
	if choice == nil {
		return Draft{}, fmt.Errorf("AI endpoint response had no choices")
	}
	if strings.TrimSpace(choice.Message.Content) == "" {
		// A reasoning model can spend its whole token budget "thinking" and
		// leave nothing for the actual answer — a real, reproducible outcome,
		// not a bug, so it gets a specific message rather than a parse error.
		if choice.FinishReason == "length" {
			return Draft{}, fmt.Errorf("AI endpoint ran out of tokens before answering (finish_reason: length)")
		}
		return Draft{}, fmt.Errorf("AI endpoint returned no content (finish_reason: %s)", choice.FinishReason)
	}

	var extracted receiptExtraction
	if err := json.Unmarshal([]byte(extractJSON(choice.Message.Content)), &extracted); err != nil {
		return Draft{}, fmt.Errorf("parse extracted JSON: %w", err)
	}
	return extracted.toDraft(), nil
}

func firstChoice(resp chatCompletionsResponse) *chatChoice {
	if len(resp.Choices) == 0 {
		return nil
	}
	return &resp.Choices[0]
}

// extractJSON pulls out the outermost {...} object. Models are told not to
// fence the JSON or add prose, and usually comply — but "usually" is not a
// contract, so this doesn't trust the whole string to parse as-is (same
// approach as ChatAiAssistant.ExtractJson in this workspace's other
// OpenAI-compatible integration).
func extractJSON(content string) string {
	start := strings.IndexByte(content, '{')
	end := strings.LastIndexByte(content, '}')
	if start >= 0 && end > start {
		return content[start : end+1]
	}
	return strings.TrimSpace(content)
}

// ----- OpenAI-compatible chat-completions request/response shapes (only the fields we use) -----

type chatCompletionsRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
}

// chatMessage.Content is `any` because the system message is a plain string
// while the user message (image + text) is an array of content parts — both
// are valid per the OpenAI chat-completions wire format.
type chatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type chatContentPart struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *chatImageURL `json:"image_url,omitempty"`
}

type chatImageURL struct {
	URL string `json:"url"`
}

type chatCompletionsResponse struct {
	Choices []chatChoice `json:"choices"`
}

type chatChoice struct {
	Message      chatResponseMessage `json:"message"`
	FinishReason string              `json:"finish_reason"`
}

type chatResponseMessage struct {
	Content string `json:"content"`
}

// ----- the extraction prompt and its parsed JSON shape -----

const receiptSystemPrompt = `You read photos of purchase receipts for a personal expense-tracking app. Reply with ONLY a JSON object — no prose, no explanation, no markdown code fence.

Keys:
  merchant        string, the store/merchant name
  occurred_at     string, ISO date YYYY-MM-DD (infer the year if only day/month is printed); empty string if illegible
  currency        string, ISO 4217 currency code; empty string if you cannot tell
  total_minor     integer, the total amount in the currency's MINOR units — always the printed amount multiplied by exactly 100, regardless of currency, even one like IDR that has no coin smaller than 1 unit in real circulation. This is a fixed-point storage convention, not a statement about real coins. Examples: a printed "12.50" (USD) becomes 1250. A printed "Rp 35.000" or "35000" (IDR — where the dots are thousands separators, not decimals) becomes 3500000, NOT 35000. Always multiply by 100 no matter how the amount is formatted. 0 if illegible.
  category_guess  string, one of: Food, Transport, Bills, Shopping, Health, Entertainment, Other
  line_items      array of {"description": string, "amount_minor": integer} — amount_minor follows the exact same "always times 100" rule as total_minor above; empty array if none are legible
  confidence      number from 0.0 to 1.0: how confident you are the fields above are correct

Never fabricate a value you cannot actually read — use an empty string, 0, or an empty list instead, and lower confidence accordingly.`

type receiptExtraction struct {
	Merchant      string               `json:"merchant"`
	OccurredAt    string               `json:"occurred_at"`
	Currency      string               `json:"currency"`
	TotalMinor    int64                `json:"total_minor"`
	CategoryGuess string               `json:"category_guess"`
	LineItems     []receiptLineItemDTO `json:"line_items"`
	Confidence    float64              `json:"confidence"`
}

type receiptLineItemDTO struct {
	Description string `json:"description"`
	AmountMinor int64  `json:"amount_minor"`
}

func (e receiptExtraction) toDraft() Draft {
	d := Draft{
		Merchant:      e.Merchant,
		Currency:      strings.ToUpper(strings.TrimSpace(e.Currency)),
		TotalMinor:    e.TotalMinor,
		CategoryGuess: e.CategoryGuess,
		Confidence:    e.Confidence,
	}
	if parsed, err := time.Parse("2006-01-02", e.OccurredAt); err == nil {
		d.OccurredAt = &parsed
	}
	for _, li := range e.LineItems {
		d.LineItems = append(d.LineItems, LineItem{Description: li.Description, AmountMinor: li.AmountMinor})
	}
	return d
}
