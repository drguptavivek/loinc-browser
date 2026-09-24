package agent

import (
	"context"
	_ "embed"
	"fmt"
	"regexp"
	"time"
)

//go:embed prompts/system.md
var systemPrompt string

const (
	defaultTimeout   = 120 * time.Second
	defaultMaxRounds = 8
)

// loincCodeRE matches a bare LOINC number such as 2951-2, both in tool results (to build the
// hallucination-guard allowlist) and in the model's final answer (to check it against that list).
var loincCodeRE = regexp.MustCompile(`\b\d{1,7}-\d\b`)

// llmClient is the subset of *Client the loop needs; tests supply a fake.
type llmClient interface {
	ChatStream(ctx context.Context, req ChatRequest, onDelta func(kind, text string)) (FinalMessage, string, error)
}

// toolCaller is the subset of *ToolBridge the loop needs; tests supply a fake.
type toolCaller interface {
	Tools() []ToolDef
	CallTool(ctx context.Context, name string, argsJSON string) (string, error)
}

// TermLookupFunc resolves a LOINC number to its long common name and status for the
// hallucination guard's "codes" event; ok is false when the code doesn't exist in the loaded
// database (which itself is a strong signal the model invented it).
type TermLookupFunc func(ctx context.Context, loincNum string) (longCommonName, status string, ok bool)

// VerifiedCode is one code the guard confirmed appeared in a tool result and resolves in the
// local database.
type VerifiedCode struct {
	LOINCNum       string `json:"loincNum"`
	LongCommonName string `json:"longCommonName"`
	Status         string `json:"status"`
}

// Event is one streamed step of the agent loop; Data marshals to the SSE "data:" payload.
type Event struct {
	Type string
	Data any
}

// Config bundles everything one Run call needs.
type Config struct {
	Client     llmClient
	Model      string
	Tools      toolCaller
	TermLookup TermLookupFunc
	Timeout    time.Duration // 0 -> defaultTimeout
	MaxRounds  int           // 0 -> defaultMaxRounds
}

// Run drives the agentic search loop: it lets the model call tools for up to MaxRounds rounds,
// then verifies every LOINC code in the final answer against codes actually seen in tool results
// this run (the hallucination guard) before emitting them.
func Run(ctx context.Context, cfg Config, messages []Message, thinking bool, emit func(Event)) {
	if emit == nil {
		emit = func(Event) {}
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	maxRounds := cfg.MaxRounds
	if maxRounds <= 0 {
		maxRounds = defaultMaxRounds
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	started := time.Now()
	full := append([]Message{{Role: "system", Content: systemPrompt}}, messages...)
	toolResultCodes := map[string]bool{}
	var finalAnswer string
	rounds := 0

	for rounds < maxRounds {
		rounds++
		onDelta := func(kind, text string) {
			if text == "" {
				return
			}
			emit(Event{Type: kind, Data: map[string]string{"text": text}})
		}
		result, _, err := cfg.Client.ChatStream(ctx, ChatRequest{
			Model:    cfg.Model,
			Messages: full,
			Tools:    cfg.Tools.Tools(),
			Thinking: thinking,
		}, onDelta)
		if err != nil {
			emit(Event{Type: "error", Data: map[string]string{"message": err.Error()}})
			return
		}
		finalAnswer = result.Content
		if len(result.ToolCalls) == 0 {
			full = append(full, Message{Role: "assistant", Content: result.Content})
			break
		}
		full = append(full, Message{Role: "assistant", Content: result.Content, ToolCalls: result.ToolCalls})
		for _, call := range result.ToolCalls {
			emit(Event{Type: "tool-start", Data: map[string]any{"id": call.ID, "name": call.Name, "args": call.Arguments}})
			resultText, callErr := cfg.Tools.CallTool(ctx, call.Name, call.Arguments)
			ok := callErr == nil
			if !ok {
				resultText = fmt.Sprintf("error: %v", callErr)
			}
			emit(Event{Type: "tool-end", Data: map[string]any{"id": call.ID, "name": call.Name, "ok": ok, "chars": len(resultText)}})
			for _, code := range loincCodeRE.FindAllString(resultText, -1) {
				toolResultCodes[code] = true
			}
			full = append(full, Message{Role: "tool", Content: resultText, ToolCallID: call.ID, Name: call.Name})
		}
	}

	verified, unverified := verifyCodes(ctx, finalAnswer, toolResultCodes, cfg.TermLookup)
	emit(Event{Type: "codes", Data: map[string]any{"codes": verified}})
	if len(unverified) > 0 {
		emit(Event{Type: "unverified", Data: map[string]any{"codes": unverified}})
	}
	emit(Event{Type: "done", Data: map[string]any{"rounds": rounds, "model": cfg.Model, "elapsedMs": time.Since(started).Milliseconds()}})
}

// verifyCodes keeps only codes that appear in both the final answer and the tool results seen
// this run, re-verified against the local database; every other code mentioned in the answer is
// reported as unverified (likely hallucinated) rather than shown to the user as a suggestion.
func verifyCodes(ctx context.Context, answer string, toolResultCodes map[string]bool, lookup TermLookupFunc) (verified []VerifiedCode, unverified []string) {
	seen := map[string]bool{}
	for _, code := range loincCodeRE.FindAllString(answer, -1) {
		if seen[code] {
			continue
		}
		seen[code] = true
		if !toolResultCodes[code] {
			unverified = append(unverified, code)
			continue
		}
		if lookup == nil {
			unverified = append(unverified, code)
			continue
		}
		longCommonName, status, ok := lookup(ctx, code)
		if !ok {
			unverified = append(unverified, code)
			continue
		}
		verified = append(verified, VerifiedCode{LOINCNum: code, LongCommonName: longCommonName, Status: status})
	}
	return verified, unverified
}
