package receipts

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestScanner(t *testing.T, handler http.HandlerFunc) *OpenAIScanner {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return NewOpenAIScanner(server.URL, "test-api-key", "openai/gpt-oss-20b", server.Client())
}

func chatResponseWithContent(content string) chatCompletionsResponse {
	return chatCompletionsResponse{Choices: []chatChoice{{
		Message:      chatResponseMessage{Content: content},
		FinishReason: "stop",
	}}}
}

func TestOpenAIScanner_Scan_ReturnsErrNotConfiguredWithoutAnApiKey(t *testing.T) {
	scanner := NewOpenAIScanner("https://example.invalid", "", "model", nil)
	if _, err := scanner.Scan(context.Background(), someBytes(), validJPEG); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("error = %v, want ErrNotConfigured", err)
	}
}

func TestOpenAIScanner_Scan_SendsTheExpectedRequestAndParsesTheJSONReply(t *testing.T) {
	var gotPath, gotAuth string
	var gotBody chatCompletionsRequest

	scanner := newTestScanner(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		content := `{"merchant":"Warung Bu Tini","occurred_at":"2026-09-01","currency":"idr","total_minor":5000000,"category_guess":"Food","confidence":0.87,"line_items":[{"description":"Nasi goreng","amount_minor":3000000},{"description":"Es teh","amount_minor":2000000}]}`
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(chatResponseWithContent(content))
	})

	draft, err := scanner.Scan(context.Background(), someBytes(), validJPEG)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if gotPath != "/chat/completions" {
		t.Errorf("path = %q, want /chat/completions", gotPath)
	}
	if gotAuth != "Bearer test-api-key" {
		t.Errorf("Authorization header = %q, want Bearer test-api-key", gotAuth)
	}
	if gotBody.Model != "openai/gpt-oss-20b" {
		t.Errorf("request model = %q", gotBody.Model)
	}
	if gotBody.MaxTokens <= 0 {
		t.Errorf("MaxTokens = %d, want a generous positive budget (reasoning models think before answering)", gotBody.MaxTokens)
	}
	if len(gotBody.Messages) != 2 || gotBody.Messages[0].Role != "system" || gotBody.Messages[1].Role != "user" {
		t.Fatalf("messages = %+v, want [system, user]", gotBody.Messages)
	}

	if draft.Merchant != "Warung Bu Tini" {
		t.Errorf("Merchant = %q", draft.Merchant)
	}
	if draft.Currency != "IDR" {
		t.Errorf("Currency = %q, want upper-cased IDR", draft.Currency)
	}
	if draft.TotalMinor != 5000000 {
		t.Errorf("TotalMinor = %d", draft.TotalMinor)
	}
	if draft.Confidence != 0.87 {
		t.Errorf("Confidence = %v", draft.Confidence)
	}
	if draft.OccurredAt == nil || !draft.OccurredAt.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("OccurredAt = %v, want 2026-09-01", draft.OccurredAt)
	}
	if len(draft.LineItems) != 2 || draft.LineItems[0].Description != "Nasi goreng" {
		t.Errorf("LineItems = %+v", draft.LineItems)
	}
}

func TestOpenAIScanner_Scan_ExtractsJSONEvenWithSurroundingProseOrAFence(t *testing.T) {
	scanner := newTestScanner(t, func(w http.ResponseWriter, r *http.Request) {
		content := "Sure, here you go:\n```json\n{\"merchant\":\"Toko Aman\",\"occurred_at\":\"2026-01-05\",\"currency\":\"IDR\",\"total_minor\":10000,\"confidence\":0.5,\"line_items\":[]}\n```"
		_ = json.NewEncoder(w).Encode(chatResponseWithContent(content))
	})

	draft, err := scanner.Scan(context.Background(), someBytes(), validJPEG)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if draft.Merchant != "Toko Aman" {
		t.Errorf("Merchant = %q, want the model's reply parsed despite the surrounding fence/prose", draft.Merchant)
	}
}

func TestOpenAIScanner_Scan_HandlesAnUnparseableDateGracefully(t *testing.T) {
	scanner := newTestScanner(t, func(w http.ResponseWriter, r *http.Request) {
		content := `{"merchant":"?","occurred_at":"","total_minor":0,"confidence":0.1,"line_items":[]}`
		_ = json.NewEncoder(w).Encode(chatResponseWithContent(content))
	})

	draft, err := scanner.Scan(context.Background(), someBytes(), validJPEG)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if draft.OccurredAt != nil {
		t.Errorf("OccurredAt = %v, want nil for an unparseable/blank date", draft.OccurredAt)
	}
}

func TestOpenAIScanner_Scan_RejectsANonOKStatus(t *testing.T) {
	scanner := newTestScanner(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	})

	if _, err := scanner.Scan(context.Background(), someBytes(), validJPEG); err == nil {
		t.Fatal("Scan() error = nil, want an error for a non-200 response")
	}
}

func TestOpenAIScanner_Scan_ReportsWhenTheModelRanOutOfTokensBeforeAnswering(t *testing.T) {
	scanner := newTestScanner(t, func(w http.ResponseWriter, r *http.Request) {
		resp := chatCompletionsResponse{Choices: []chatChoice{{
			Message:      chatResponseMessage{Content: ""},
			FinishReason: "length",
		}}}
		_ = json.NewEncoder(w).Encode(resp)
	})

	_, err := scanner.Scan(context.Background(), someBytes(), validJPEG)
	if err == nil {
		t.Fatal("Scan() error = nil, want an error when the model returns empty content at finish_reason=length")
	}
}

func TestOpenAIScanner_Scan_RejectsAResponseWithNoChoices(t *testing.T) {
	scanner := newTestScanner(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(chatCompletionsResponse{})
	})

	if _, err := scanner.Scan(context.Background(), someBytes(), validJPEG); err == nil {
		t.Fatal("Scan() error = nil, want an error when there are no choices")
	}
}
