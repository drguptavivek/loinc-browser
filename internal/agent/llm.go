// Package agent runs an agentic LOINC-search loop against an OpenAI-compatible chat/completions
// endpoint (LM Studio, Ollama, vLLM, or a hosted API), using the existing MCP tools as its only
// way to read LOINC data.
package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
)

// Message is one OpenAI-style chat message. ToolCalls is set on an assistant message that
// requested tool calls; ToolCallID/Name identify which call a "tool" role message answers.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	Name       string     `json:"name,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall is one function call, either requested by the model (Arguments is a JSON object
// string) or, in a streamed delta, a fragment of one being accumulated by index.
type ToolCall struct {
	ID        string `json:"-"`
	Name      string `json:"-"`
	Arguments string `json:"-"`
}

// MarshalJSON writes the OpenAI wire shape for a tool call inside an assistant message.
func (tc ToolCall) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}{
		ID:   tc.ID,
		Type: "function",
		Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Name: tc.Name, Arguments: tc.Arguments},
	})
}

// ToolDef is one tool offered to the model, in the OpenAI "tools" array shape.
type ToolDef struct {
	Name        string
	Description string
	Parameters  any // JSON Schema object (e.g. an MCP tool's InputSchema)
}

func (t ToolDef) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type     string `json:"type"`
		Function struct {
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
			Parameters  any    `json:"parameters,omitempty"`
		} `json:"function"`
	}{
		Type: "function",
		Function: struct {
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
			Parameters  any    `json:"parameters,omitempty"`
		}{Name: t.Name, Description: t.Description, Parameters: t.Parameters},
	})
}

// ChatRequest is one turn of the agent loop sent to the chat/completions endpoint.
type ChatRequest struct {
	Model       string
	Messages    []Message
	Tools       []ToolDef
	Thinking    bool
	Temperature float64 // 0 uses the default (0.2)
	MaxTokens   int     // 0 uses the default (2048)
}

// FinalMessage is the assistant message assembled from a completed stream.
type FinalMessage struct {
	Content   string
	ToolCalls []ToolCall
}

// Client calls an OpenAI-compatible POST {BaseURL}/chat/completions and GET {BaseURL}/models.
type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client

	mu                   sync.Mutex
	noChatTemplateKwargs bool // set once a server 400s on that field; skipped on later calls
}

// NewClient builds a Client whose HTTP transport is NewHTTPClient(localOnly) — every dial is
// checked against localOnly the same way regardless of which agent route constructed the client.
func NewClient(baseURL, apiKey string, localOnly bool) *Client {
	return &Client{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		APIKey:  strings.TrimSpace(apiKey),
		HTTP:    NewHTTPClient(localOnly),
	}
}

func (c *Client) setAuth(req *http.Request) {
	req.Header.Set("content-type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("authorization", "Bearer "+c.APIKey)
	}
}

// embeddingIDHint matches model ids that are almost certainly embedding models, not chat models.
const embeddingIDHint = "embed"

// maxModelsBodyBytes bounds the GET /models response body; a misbehaving or hostile endpoint
// should not be able to exhaust memory decoding it.
const maxModelsBodyBytes = 2 << 20 // 2 MB

// maxChatStreamBytes bounds the total bytes read from a chat/completions SSE stream across all
// lines, closing off an endpoint that never sends [DONE] and just keeps streaming.
const maxChatStreamBytes = 8 << 20 // 8 MB

// ListModels returns the chat-capable model ids from GET {base}/models, filtering out ids that
// look like embedding models unless doing so would leave nothing.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(req)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list models at %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2000))
		return nil, fmt.Errorf("list models at %s: %s", c.BaseURL, upstreamErrorMessage(resp.StatusCode, body))
	}
	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxModelsBodyBytes)).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode model list: %w", err)
	}
	all := make([]string, 0, len(parsed.Data))
	chatOnly := make([]string, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		if m.ID == "" {
			continue
		}
		all = append(all, m.ID)
		if !strings.Contains(strings.ToLower(m.ID), embeddingIDHint) {
			chatOnly = append(chatOnly, m.ID)
		}
	}
	if len(chatOnly) > 0 {
		return chatOnly, nil
	}
	return all, nil
}

// qwenThinkSuffix appends /think or /no_think to a Qwen3 model's system prompt, which is that
// family's own switch for its chain-of-thought; other model families ignore an unknown suffix
// word so this is harmless to send always for a qwen3 id.
func qwenThinkSuffix(messages []Message, model string, thinking bool) []Message {
	if !strings.Contains(strings.ToLower(model), "qwen3") {
		return messages
	}
	suffix := " /no_think"
	if thinking {
		suffix = " /think"
	}
	out := make([]Message, len(messages))
	copy(out, messages)
	for i := range out {
		if out[i].Role == "system" {
			out[i].Content += suffix
			return out
		}
	}
	return append([]Message{{Role: "system", Content: strings.TrimSpace(suffix)}}, out...)
}

// buildBody assembles the request payload. includeChatTemplateKwargs is false on the retry after
// a server rejected that field.
func buildBody(req ChatRequest, includeChatTemplateKwargs bool) map[string]any {
	temperature := req.Temperature
	if temperature == 0 {
		temperature = 0.2
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 2048
	}
	body := map[string]any{
		"model":       req.Model,
		"messages":    qwenThinkSuffix(req.Messages, req.Model, req.Thinking),
		"stream":      true,
		"temperature": temperature,
		"max_tokens":  maxTokens,
	}
	if len(req.Tools) > 0 {
		body["tools"] = req.Tools
		body["tool_choice"] = "auto"
	}
	if includeChatTemplateKwargs {
		body["chat_template_kwargs"] = map[string]any{"enable_thinking": req.Thinking}
	}
	return body
}

// ChatStream streams one chat/completions turn, invoking onDelta(kind, text) with kind "text" for
// answer content and "thinking" for reasoning content (from delta.reasoning_content,
// delta.reasoning, or an inline <think>...</think> span in delta.content). It returns the
// assembled final assistant message and the finish reason.
func (c *Client) ChatStream(ctx context.Context, req ChatRequest, onDelta func(kind, text string)) (FinalMessage, string, error) {
	c.mu.Lock()
	includeKwargs := !c.noChatTemplateKwargs
	c.mu.Unlock()

	final, finishReason, status, retryable, err := c.chatStreamOnce(ctx, req, includeKwargs, onDelta)
	if err != nil && retryable && includeKwargs && status == http.StatusBadRequest {
		c.mu.Lock()
		c.noChatTemplateKwargs = true
		c.mu.Unlock()
		final, finishReason, _, _, err = c.chatStreamOnce(ctx, req, false, onDelta)
	}
	return final, finishReason, err
}

func (c *Client) chatStreamOnce(ctx context.Context, req ChatRequest, includeChatTemplateKwargs bool, onDelta func(kind, text string)) (FinalMessage, string, int, bool, error) {
	payload, err := json.Marshal(buildBody(req, includeChatTemplateKwargs))
	if err != nil {
		return FinalMessage{}, "", 0, false, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return FinalMessage{}, "", 0, false, err
	}
	c.setAuth(httpReq)
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return FinalMessage{}, "", 0, false, fmt.Errorf("chat endpoint %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2000))
		msg := strings.TrimSpace(string(body))
		retryable := includeChatTemplateKwargs && resp.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(msg), "chat_template_kwargs")
		return FinalMessage{}, "", resp.StatusCode, retryable, fmt.Errorf("chat endpoint %s: %s", c.BaseURL, upstreamErrorMessage(resp.StatusCode, body))
	}

	final, finishReason, err := parseChatStream(resp.Body, onDelta)
	return final, finishReason, resp.StatusCode, false, err
}

type toolCallAccum struct {
	id   string
	name string
	args strings.Builder
}

func parseChatStream(body io.Reader, onDelta func(kind, text string)) (FinalMessage, string, error) {
	if onDelta == nil {
		onDelta = func(string, string) {}
	}
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	var content strings.Builder
	var finishReason string
	router := &thinkRouter{}
	calls := map[int]*toolCallAccum{}
	var order []int
	var totalBytes int

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		totalBytes += len(line)
		if totalBytes > maxChatStreamBytes {
			return FinalMessage{}, finishReason, fmt.Errorf("chat stream exceeded %d bytes", maxChatStreamBytes)
		}
		data, ok := strings.CutPrefix(line, "data:")
		if !ok {
			continue
		}
		data = strings.TrimSpace(data)
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					Reasoning        string `json:"reasoning"`
					ReasoningContent string `json:"reasoning_content"`
					ToolCalls        []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // skip a malformed line rather than aborting the whole stream
		}
		for _, choice := range chunk.Choices {
			if choice.FinishReason != "" {
				finishReason = choice.FinishReason
			}
			if reasoning := firstNonEmpty(choice.Delta.ReasoningContent, choice.Delta.Reasoning); reasoning != "" {
				onDelta("thinking", reasoning)
			}
			if choice.Delta.Content != "" {
				content.WriteString(choice.Delta.Content)
				answer, thinking := router.feed(choice.Delta.Content)
				if answer != "" {
					onDelta("text", answer)
				}
				if thinking != "" {
					onDelta("thinking", thinking)
				}
			}
			for _, tc := range choice.Delta.ToolCalls {
				acc, exists := calls[tc.Index]
				if !exists {
					acc = &toolCallAccum{}
					calls[tc.Index] = acc
					order = append(order, tc.Index)
				}
				if tc.ID != "" {
					acc.id = tc.ID
				}
				if tc.Function.Name != "" {
					acc.name = tc.Function.Name
				}
				acc.args.WriteString(tc.Function.Arguments)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return FinalMessage{}, finishReason, fmt.Errorf("read chat stream: %w", err)
	}

	sort.Ints(order)
	toolCalls := make([]ToolCall, 0, len(order))
	for _, idx := range order {
		acc := calls[idx]
		toolCalls = append(toolCalls, ToolCall{ID: acc.id, Name: acc.name, Arguments: acc.args.String()})
	}
	return FinalMessage{Content: content.String(), ToolCalls: toolCalls}, finishReason, nil
}

// upstreamErrorMessage turns a failed upstream HTTP response into a message safe to show a
// browser client: a generic "endpoint returned <status>", plus, when the body is an
// OpenAI-style {"error":{"message":...}} payload, that message alone (truncated to 300 chars) —
// often the one actionable detail (e.g. LM Studio's "Invalid model identifier ..."). The raw body
// is otherwise discarded rather than echoed back.
func upstreamErrorMessage(status int, body []byte) string {
	msg := fmt.Sprintf("LLM endpoint returned %d", status)
	var parsed struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &parsed) == nil && parsed.Error.Message != "" {
		detail := parsed.Error.Message
		if len(detail) > 300 {
			detail = detail[:300]
		}
		msg += ": " + detail
	}
	return msg
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// thinkRouter splits a content stream into answer text and <think>...</think> reasoning text,
// holding back the tail of the buffer that could still be an incomplete tag.
type thinkRouter struct {
	buf     strings.Builder
	inThink bool
}

const (
	openThinkTag  = "<think>"
	closeThinkTag = "</think>"
)

func (t *thinkRouter) feed(chunk string) (answer, thinking string) {
	t.buf.WriteString(chunk)
	var answerOut, thinkOut strings.Builder
	for {
		s := t.buf.String()
		tag := openThinkTag
		if t.inThink {
			tag = closeThinkTag
		}
		i := strings.Index(s, tag)
		if i < 0 {
			break
		}
		pre := s[:i]
		if t.inThink {
			thinkOut.WriteString(pre)
		} else {
			answerOut.WriteString(pre)
		}
		t.inThink = !t.inThink
		t.buf.Reset()
		t.buf.WriteString(s[i+len(tag):])
	}
	s := t.buf.String()
	holdBack := 0
	maxTagLen := len(closeThinkTag)
	for k := 1; k < maxTagLen && k <= len(s); k++ {
		suffix := s[len(s)-k:]
		if strings.HasPrefix(openThinkTag, suffix) || strings.HasPrefix(closeThinkTag, suffix) {
			holdBack = k
		}
	}
	flush := s[:len(s)-holdBack]
	if t.inThink {
		thinkOut.WriteString(flush)
	} else {
		answerOut.WriteString(flush)
	}
	t.buf.Reset()
	t.buf.WriteString(s[len(s)-holdBack:])
	return answerOut.String(), thinkOut.String()
}
