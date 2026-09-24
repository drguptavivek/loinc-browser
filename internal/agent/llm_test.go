package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func sseServer(t *testing.T, lines []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		for _, line := range lines {
			_, _ = io.WriteString(w, "data: "+line+"\n\n")
			flusher.Flush()
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
}

func TestChatStreamAccumulatesSplitToolCallArguments(t *testing.T) {
	srv := sseServer(t, []string{
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"loinc_search_terms","arguments":"{\"que"}}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ry\":\"sodium\"}"}}]}}]}`,
		`{"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
	})
	defer srv.Close()

	client := NewClient(srv.URL, "", false)
	final, finishReason, err := client.ChatStream(context.Background(), ChatRequest{Model: "m"}, nil)
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if finishReason != "tool_calls" {
		t.Fatalf("finish reason = %q, want tool_calls", finishReason)
	}
	if len(final.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(final.ToolCalls))
	}
	call := final.ToolCalls[0]
	if call.ID != "call_1" || call.Name != "loinc_search_terms" {
		t.Fatalf("tool call = %+v", call)
	}
	if call.Arguments != `{"query":"sodium"}` {
		t.Fatalf("arguments = %q", call.Arguments)
	}
}

func TestChatStreamRoutesReasoningContentSeparately(t *testing.T) {
	srv := sseServer(t, []string{
		`{"choices":[{"delta":{"reasoning_content":"thinking about it"}}]}`,
		`{"choices":[{"delta":{"content":"the answer"}}]}`,
		`{"choices":[{"delta":{},"finish_reason":"stop"}]}`,
	})
	defer srv.Close()

	var thinking, text string
	client := NewClient(srv.URL, "", false)
	final, _, err := client.ChatStream(context.Background(), ChatRequest{Model: "m"}, func(kind, s string) {
		switch kind {
		case "thinking":
			thinking += s
		case "text":
			text += s
		}
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if thinking != "thinking about it" {
		t.Fatalf("thinking = %q", thinking)
	}
	if text != "the answer" || final.Content != "the answer" {
		t.Fatalf("text = %q, final.Content = %q", text, final.Content)
	}
}

func TestChatStreamRoutesInlineThinkTagSpanningChunks(t *testing.T) {
	srv := sseServer(t, []string{
		`{"choices":[{"delta":{"content":"<thi"}}]}`,
		`{"choices":[{"delta":{"content":"nk>reasoning here</thi"}}]}`,
		`{"choices":[{"delta":{"content":"nk>answer here"}}]}`,
		`{"choices":[{"delta":{},"finish_reason":"stop"}]}`,
	})
	defer srv.Close()

	var thinking, text string
	client := NewClient(srv.URL, "", false)
	if _, _, err := client.ChatStream(context.Background(), ChatRequest{Model: "m"}, func(kind, s string) {
		switch kind {
		case "thinking":
			thinking += s
		case "text":
			text += s
		}
	}); err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if thinking != "reasoning here" {
		t.Fatalf("thinking = %q", thinking)
	}
	if text != "answer here" {
		t.Fatalf("text = %q", text)
	}
}

func TestChatStreamRetriesOnceWithoutChatTemplateKwargsOn400(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		body, _ := io.ReadAll(r.Body)
		hasKwargs := strings.Contains(string(body), "chat_template_kwargs")
		if n == 1 {
			if !hasKwargs {
				t.Errorf("first attempt should include chat_template_kwargs")
			}
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"unknown field chat_template_kwargs"}`))
			return
		}
		if hasKwargs {
			t.Errorf("retry should omit chat_template_kwargs")
		}
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		_, _ = io.WriteString(w, `data: {"choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}]}`+"\n\n")
		flusher.Flush()
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "", false)
	final, _, err := client.ChatStream(context.Background(), ChatRequest{Model: "m"}, nil)
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if final.Content != "ok" {
		t.Fatalf("content = %q", final.Content)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
}

func TestListModelsFiltersEmbeddingModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{
			{"id": "qwen3-8b"},
			{"id": "text-embedding-qwen3-embedding-0.6b"},
		}})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "", false)
	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 1 || models[0] != "qwen3-8b" {
		t.Fatalf("models = %v", models)
	}
}

func TestCheckBaseURLTable(t *testing.T) {
	cases := []struct {
		name      string
		url       string
		localOnly bool
		wantErr   bool
	}{
		{"localhost allowed", "http://localhost:1234/v1", true, false},
		{"loopback ip allowed", "http://127.0.0.1:1234/v1", true, false},
		{"non-http scheme rejected", "ftp://127.0.0.1/v1", true, true},
		{"public host rejected when local-only", "https://api.openai.com/v1", true, true},
		{"public host allowed when not local-only", "https://api.openai.com/v1", false, false},
		{"empty url rejected", "", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckBaseURL(tc.url, tc.localOnly)
			if (err != nil) != tc.wantErr {
				t.Fatalf("CheckBaseURL(%q, %v) err = %v, wantErr %v", tc.url, tc.localOnly, err, tc.wantErr)
			}
		})
	}
}

// TestListModelsSanitizesUpstreamErrorBody confirms a failed /models call surfaces only the
// OpenAI-style error message, not the endpoint's raw response body (which could contain anything
// the upstream server chose to send).
func TestListModelsSanitizesUpstreamErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":{"message":"Invalid model identifier: bogus-model"},"secret_debug_trace":"internal stack dump"}`)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "", false).ListModels(context.Background())
	if err == nil {
		t.Fatal("expected an error from a 404 response")
	}
	if !strings.Contains(err.Error(), "Invalid model identifier: bogus-model") {
		t.Fatalf("expected the sanitized error message, got %q", err.Error())
	}
	if strings.Contains(err.Error(), "secret_debug_trace") || strings.Contains(err.Error(), "internal stack dump") {
		t.Fatalf("raw upstream body leaked into error: %q", err.Error())
	}
}

// TestParseChatStreamAbortsPastByteLimit confirms a stream that never sends [DONE] is bounded
// rather than read forever.
func TestParseChatStreamAbortsPastByteLimit(t *testing.T) {
	line := `{"choices":[{"delta":{"content":"` + strings.Repeat("x", 1<<20) + `"}}]}` + "\n"
	body := strings.NewReader(strings.Repeat("data: "+line+"\n", 10)) // ~10 MB, over maxChatStreamBytes
	_, _, err := parseChatStream(body, nil)
	if err == nil {
		t.Fatal("expected parseChatStream to abort once the total byte limit is exceeded")
	}
}
