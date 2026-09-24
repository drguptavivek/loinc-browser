package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"loinc-browser/internal/agent"
)

// Settings persisted in the shared KV file, alongside the official-API credentials
// (official_credentials.go). The API key is encrypted at rest with the same app key and AEAD
// helpers as OfficialCredentialVault; it is never returned to the browser (see
// agentSettingsResponse).
const (
	agentLLMSettingsKey = "agent_llm.settings"
	agentLLMAPIKeyKey   = "agent_llm.api_key"
	agentLLMAPIKeyAlg   = "AES-256-GCM"
)

// agentLLMSettings is the non-secret part of the agent's saved settings. Once any field has been
// saved via PUT, the whole record supersedes the env-seeded defaults for baseUrl/model/thinking
// (see (*app).agentResolvedSettings); a PUT always writes back the full merged record so a
// partial update never loses the other saved fields.
type agentLLMSettings struct {
	BaseURL  string `json:"baseUrl"`
	Model    string `json:"model"`
	Thinking bool   `json:"thinking"`
}

type agentAPIKeyRecord struct {
	Algorithm  string `json:"algorithm"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

// agentSettingsStore persists agentLLMSettings and the encrypted API key in the same KV file and
// app-key file the official credential vault uses. nil when the app wasn't given an app-key path
// and KV path (AppKeyPath/KVPath), in which case settings live in env only for the process
// lifetime and PUT reports the store as unconfigured.
type agentSettingsStore struct {
	keyPath string
	kvPath  string
}

func newAgentSettingsStore(keyPath, kvPath string) *agentSettingsStore {
	keyPath, kvPath = strings.TrimSpace(keyPath), strings.TrimSpace(kvPath)
	if keyPath == "" || kvPath == "" {
		return nil
	}
	return &agentSettingsStore{keyPath: keyPath, kvPath: kvPath}
}

func (s *agentSettingsStore) load() (agentLLMSettings, error) {
	kvFileMu.Lock()
	defer kvFileMu.Unlock()
	store, err := readKVFile(s.kvPath)
	if err != nil {
		return agentLLMSettings{}, err
	}
	raw := store[agentLLMSettingsKey]
	if len(raw) == 0 {
		return agentLLMSettings{}, nil
	}
	var settings agentLLMSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		return agentLLMSettings{}, sanitizedSettingsError(err)
	}
	return settings, nil
}

func (s *agentSettingsStore) save(settings agentLLMSettings) error {
	kvFileMu.Lock()
	defer kvFileMu.Unlock()
	store, err := readKVFile(s.kvPath)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal agent settings: %w", err)
	}
	store[agentLLMSettingsKey] = raw
	return writeKVFile(s.kvPath, store)
}

func (s *agentSettingsStore) saveAPIKey(apiKey string) error {
	kvFileMu.Lock()
	defer kvFileMu.Unlock()
	key, err := loadOrCreateAppKey(s.keyPath)
	if err != nil {
		return err
	}
	aead, err := aesGCM(key)
	if err != nil {
		return err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("generate agent API key nonce: %w", err)
	}
	ciphertext := aead.Seal(nil, nonce, []byte(apiKey), []byte(agentLLMAPIKeyKey))
	store, err := readKVFile(s.kvPath)
	if err != nil {
		return err
	}
	record := agentAPIKeyRecord{
		Algorithm:  agentLLMAPIKeyAlg,
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal agent API key: %w", err)
	}
	store[agentLLMAPIKeyKey] = raw
	return writeKVFile(s.kvPath, store)
}

func (s *agentSettingsStore) clearAPIKey() error {
	kvFileMu.Lock()
	defer kvFileMu.Unlock()
	store, err := readKVFile(s.kvPath)
	if err != nil {
		return err
	}
	delete(store, agentLLMAPIKeyKey)
	return writeKVFile(s.kvPath, store)
}

func (s *agentSettingsStore) loadAPIKey() (string, bool, error) {
	kvFileMu.Lock()
	defer kvFileMu.Unlock()
	store, err := readKVFile(s.kvPath)
	if err != nil {
		return "", false, err
	}
	raw := store[agentLLMAPIKeyKey]
	if len(raw) == 0 {
		return "", false, nil
	}
	var record agentAPIKeyRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return "", false, sanitizedSettingsError(err)
	}
	if record.Algorithm != agentLLMAPIKeyAlg {
		return "", false, errors.New("saved agent API key uses an unsupported encryption algorithm")
	}
	key, err := loadOrCreateAppKey(s.keyPath)
	if err != nil {
		return "", false, err
	}
	aead, err := aesGCM(key)
	if err != nil {
		return "", false, err
	}
	nonce, err := base64.StdEncoding.DecodeString(record.Nonce)
	if err != nil {
		return "", false, errors.New("saved agent API key is unavailable")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(record.Ciphertext)
	if err != nil {
		return "", false, errors.New("saved agent API key is unavailable")
	}
	plain, err := aead.Open(nil, nonce, ciphertext, []byte(agentLLMAPIKeyKey))
	if err != nil {
		return "", false, errors.New("saved agent API key is unavailable")
	}
	return string(plain), true, nil
}

// agentEffectiveSettings is the env defaults merged with any saved override, resolved fresh on
// every request (so a saved change takes effect without a restart).
type agentEffectiveSettings struct {
	BaseURL   string
	Model     string
	APIKey    string
	APIKeySet bool
	Thinking  bool
	LocalOnly bool // env-only; not UI-settable (see agentSettingsUpdateRequest)
}

func (a *app) agentResolvedSettings() (agentEffectiveSettings, error) {
	out := agentEffectiveSettings{
		BaseURL:   a.agentEnvBaseURL,
		Model:     a.agentEnvModel,
		APIKey:    a.agentEnvAPIKey,
		Thinking:  a.agentEnvThinking,
		LocalOnly: a.agentEnvLocalOnly,
	}
	if a.agentSettings != nil {
		saved, err := a.agentSettings.load()
		if err != nil {
			return out, err
		}
		out.BaseURL, out.Model, out.Thinking = saved.BaseURL, saved.Model, saved.Thinking
		if out.BaseURL == "" {
			out.BaseURL = a.agentEnvBaseURL
		}
		if out.Model == "" {
			out.Model = a.agentEnvModel
		}
		if apiKey, set, err := a.agentSettings.loadAPIKey(); err != nil {
			return out, err
		} else if set {
			out.APIKey = apiKey
		}
	}
	out.APIKeySet = strings.TrimSpace(out.APIKey) != ""
	return out, nil
}

func agentSettingsResponse(effective agentEffectiveSettings) map[string]any {
	return map[string]any{
		"baseUrl":    effective.BaseURL,
		"model":      effective.Model,
		"thinking":   effective.Thinking,
		"apiKeySet":  effective.APIKeySet,
		"localOnly":  effective.LocalOnly,
		"configured": strings.TrimSpace(effective.BaseURL) != "" && strings.TrimSpace(effective.Model) != "",
	}
}

var errAgentDisabled = errors.New("the LOINC search agent is disabled on this server")

// agentDisabledGuard rejects every agent route (even read-only ones) when LOINC_AGENT_DISABLED
// is set, without requiring the passphrase agentGuard also checks.
func (a *app) agentDisabledGuard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.agentDisabled {
			writeError(w, http.StatusForbidden, errAgentDisabled)
			return
		}
		next(w, r)
	}
}

// agentGuard additionally requires the official-API passphrase (the same
// X-Loinc-Passphrase header and LOINC_OFFICIAL_PASSPHRASE env var) on mutating or
// network-reaching agent routes. The agent has its own disabled flag (LOINC_AGENT_DISABLED),
// independent of officialDisabled, so it does not reuse app.officialGuard directly.
func (a *app) agentGuard(next http.HandlerFunc) http.HandlerFunc {
	return a.agentDisabledGuard(func(w http.ResponseWriter, r *http.Request) {
		if a.officialPassphrase != "" && subtle.ConstantTimeCompare([]byte(r.Header.Get(officialPassphraseHeader)), []byte(a.officialPassphrase)) != 1 {
			writeError(w, http.StatusUnauthorized, errors.New("official API passphrase is missing or incorrect"))
			return
		}
		next(w, r)
	})
}

func (a *app) getAgentSettings(w http.ResponseWriter, r *http.Request) {
	effective, err := a.agentResolvedSettings()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	writeJSON(w, http.StatusOK, agentSettingsResponse(effective))
}

type agentSettingsUpdateRequest struct {
	BaseURL     *string `json:"baseUrl"`
	Model       *string `json:"model"`
	Thinking    *bool   `json:"thinking"`
	APIKey      *string `json:"apiKey"`
	ClearAPIKey bool    `json:"clearApiKey"`
}

func (a *app) putAgentSettings(w http.ResponseWriter, r *http.Request) {
	if a.agentSettings == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("agent settings storage is not configured"))
		return
	}
	var req agentSettingsUpdateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid agent settings request"))
		return
	}
	effective, err := a.agentResolvedSettings()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	merged := agentLLMSettings{BaseURL: effective.BaseURL, Model: effective.Model, Thinking: effective.Thinking}
	if req.BaseURL != nil {
		merged.BaseURL = strings.TrimSpace(*req.BaseURL)
	}
	if req.Model != nil {
		merged.Model = strings.TrimSpace(*req.Model)
	}
	if req.Thinking != nil {
		merged.Thinking = *req.Thinking
	}
	if merged.BaseURL != "" {
		if err := agent.CheckBaseURL(merged.BaseURL, effective.LocalOnly); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	if err := a.agentSettings.save(merged); err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	if req.ClearAPIKey {
		if err := a.agentSettings.clearAPIKey(); err != nil {
			writeError(w, http.StatusServiceUnavailable, err)
			return
		}
	} else if req.APIKey != nil && strings.TrimSpace(*req.APIKey) != "" {
		if err := a.agentSettings.saveAPIKey(*req.APIKey); err != nil {
			writeError(w, http.StatusServiceUnavailable, err)
			return
		}
	}
	effective, err = a.agentResolvedSettings()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	writeJSON(w, http.StatusOK, agentSettingsResponse(effective))
}

const agentNetworkTimeout = 30 * time.Second

// normalizeBaseURL makes two base URL spellings comparable: trim whitespace and any trailing
// slash. Used only to decide whether an ad-hoc ?baseUrl= probe matches the saved endpoint (and so
// may carry its API key) — not a security boundary by itself, since CheckBaseURL/NewHTTPClient
// still validate and dial-guard whatever URL is actually used.
func normalizeBaseURL(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), "/")
}

func (a *app) agentModels(w http.ResponseWriter, r *http.Request) {
	effective, err := a.agentResolvedSettings()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	baseURL := strings.TrimSpace(r.URL.Query().Get("baseUrl"))
	// The saved/env API key is a secret for the saved endpoint; an ad-hoc ?baseUrl= probe (used
	// by the settings UI to test a URL before saving it) must never carry it to some other host.
	apiKey := effective.APIKey
	if baseURL == "" {
		baseURL = effective.BaseURL
	} else if normalizeBaseURL(baseURL) != normalizeBaseURL(effective.BaseURL) {
		apiKey = ""
	}
	if err := agent.CheckBaseURL(baseURL, effective.LocalOnly); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), agentNetworkTimeout)
	defer cancel()
	ids, err := agent.NewClient(baseURL, apiKey, effective.LocalOnly).ListModels(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	models := make([]map[string]string, 0, len(ids))
	for _, id := range ids {
		models = append(models, map[string]string{"id": id})
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

func (a *app) agentTest(w http.ResponseWriter, r *http.Request) {
	effective, err := a.agentResolvedSettings()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	respond := func(ok bool, latencyMs int64, toolCalls bool, testErr error) {
		body := map[string]any{"ok": ok, "model": effective.Model, "latencyMs": latencyMs, "toolCalls": toolCalls}
		if testErr != nil {
			body["error"] = testErr.Error()
		}
		writeJSON(w, http.StatusOK, body)
	}
	if effective.BaseURL == "" || effective.Model == "" {
		respond(false, 0, false, errors.New("agent is not configured: set a base URL and model"))
		return
	}
	if err := agent.CheckBaseURL(effective.BaseURL, effective.LocalOnly); err != nil {
		respond(false, 0, false, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), agentNetworkTimeout)
	defer cancel()
	probe := agent.ToolDef{
		Name:        "ping",
		Description: "Call this tool with no arguments to confirm tool-calling support.",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
	}
	started := time.Now()
	final, finishReason, err := agent.NewClient(effective.BaseURL, effective.APIKey, effective.LocalOnly).ChatStream(ctx, agent.ChatRequest{
		Model:    effective.Model,
		Messages: []agent.Message{{Role: "user", Content: "Call the ping tool now, with no arguments."}},
		Tools:    []agent.ToolDef{probe},
		Thinking: effective.Thinking,
		// Generous: a small model that reasons before calling a tool (e.g. Gemma) can spend
		// most of a tight budget on reasoning_content and never reach the tool call, which
		// reported as a flat "doesn't support tools" even though it does with room to think.
		MaxTokens: agentTestProbeMaxTokens,
	}, nil)
	latency := time.Since(started).Milliseconds()
	if err != nil {
		respond(false, latency, false, err)
		return
	}
	toolCalls := len(final.ToolCalls) > 0
	if !toolCalls && finishReason == "length" {
		respond(true, latency, false, fmt.Errorf("the model ran out of tokens (reasoning first) before it could call the tool; it may still support tools with more headroom, or may not support them at all"))
		return
	}
	respond(true, latency, toolCalls, nil)
}

// agentTestProbeMaxTokens is generous enough that a reasoning model (Gemma, Qwen3, …) has room to
// think before calling the probe tool; 64 was routinely exhausted by reasoning_content alone.
const agentTestProbeMaxTokens = 512

const (
	maxAgentChatMessages     = 20
	maxAgentChatMessageChars = 4000
)

type agentChatMessageRequest struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type agentChatRequest struct {
	Messages []agentChatMessageRequest `json:"messages"`
	Thinking *bool                     `json:"thinking"`
}

// agentChat streams the agent loop's events as text/event-stream: "event: <type>\ndata:
// <json>\n\n" per event, matching web/src/lib/sse.ts's parser. r.Context() is passed straight
// into agent.Run, so a client disconnect cancels the in-flight LLM request.
func (a *app) agentChat(w http.ResponseWriter, r *http.Request) {
	effective, err := a.agentResolvedSettings()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	if effective.BaseURL == "" || effective.Model == "" {
		writeError(w, http.StatusConflict, errors.New("agent is not configured: set a base URL and model in agent settings"))
		return
	}
	if err := agent.CheckBaseURL(effective.BaseURL, effective.LocalOnly); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var req agentChatRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid agent chat request"))
		return
	}
	if len(req.Messages) == 0 || len(req.Messages) > maxAgentChatMessages {
		writeError(w, http.StatusBadRequest, fmt.Errorf("messages must have 1-%d entries", maxAgentChatMessages))
		return
	}
	messages := make([]agent.Message, 0, len(req.Messages))
	for _, m := range req.Messages {
		role := strings.TrimSpace(m.Role)
		if role != "user" && role != "assistant" {
			writeError(w, http.StatusBadRequest, errors.New("message role must be user or assistant"))
			return
		}
		if len(m.Content) > maxAgentChatMessageChars {
			writeError(w, http.StatusBadRequest, fmt.Errorf("message content exceeds %d characters", maxAgentChatMessageChars))
			return
		}
		messages = append(messages, agent.Message{Role: role, Content: m.Content})
	}
	thinking := effective.Thinking
	if req.Thinking != nil {
		thinking = *req.Thinking
	}
	if a.agentMCPServer == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("agent tools are not available"))
		return
	}
	bridge, err := agent.NewToolBridge(r.Context(), a.agentMCPServer)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	defer bridge.Close()

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, errors.New("streaming is not supported by this response writer"))
		return
	}
	w.Header().Set("content-type", "text/event-stream")
	w.Header().Set("cache-control", "no-cache")
	w.Header().Set("connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	client := agent.NewClient(effective.BaseURL, effective.APIKey, effective.LocalOnly)
	termLookup := func(ctx context.Context, loincNum string) (string, string, bool) {
		store, err := a.currentStore()
		if err != nil {
			return "", "", false
		}
		term, err := store.Term(ctx, loincNum)
		if err != nil {
			return "", "", false
		}
		return term.LongCommonName, term.Status, true
	}
	agent.Run(r.Context(), agent.Config{
		Client:     client,
		Model:      effective.Model,
		Tools:      bridge,
		TermLookup: termLookup,
	}, messages, thinking, func(evt agent.Event) {
		writeAgentSSE(w, flusher, evt)
	})
}

func writeAgentSSE(w http.ResponseWriter, flusher http.Flusher, evt agent.Event) {
	payload, err := json.Marshal(evt.Data)
	if err != nil {
		payload = []byte("{}")
	}
	fmt.Fprintf(w, "event: %s\n", evt.Type)
	fmt.Fprintf(w, "data: %s\n\n", payload)
	flusher.Flush()
}
