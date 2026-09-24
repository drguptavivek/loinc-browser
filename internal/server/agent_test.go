package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestAgentSettingsGetNeverLeaksAPIKey(t *testing.T) {
	handler := New(Options{
		AppKeyPath:      filepath.Join(t.TempDir(), "app.key"),
		KVPath:          filepath.Join(t.TempDir(), "kv.json"),
		AgentLLMBaseURL: "http://127.0.0.1:1234/v1",
		AgentLLMModel:   "qwen3-8b",
		AgentLLMAPIKey:  "super-secret-key",
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/agent/settings")
	if err != nil {
		t.Fatalf("GET settings: %v", err)
	}
	defer resp.Body.Close()
	body := readAll(t, resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	if strings.Contains(body, "super-secret-key") {
		t.Fatalf("settings response leaked the API key: %s", body)
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(body), &settings); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	if settings["apiKeySet"] != true {
		t.Fatalf("expected apiKeySet=true, got %#v", settings)
	}
	if settings["configured"] != true {
		t.Fatalf("expected configured=true, got %#v", settings)
	}
	if _, hasKey := settings["apiKey"]; hasKey {
		t.Fatalf("settings response must not include an apiKey field: %#v", settings)
	}
}

func TestAgentSettingsPutRejectsPublicURLWhenLocalOnly(t *testing.T) {
	handler := New(Options{
		AppKeyPath:        filepath.Join(t.TempDir(), "app.key"),
		KVPath:            filepath.Join(t.TempDir(), "kv.json"),
		AgentLLMLocalOnly: true,
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodPut, server.URL+"/api/v1/agent/settings", strings.NewReader(`{"baseUrl":"https://api.openai.com/v1"}`))
	req.Header.Set("content-type", "application/json")
	putResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT settings: %v", err)
	}
	defer putResp.Body.Close()
	body := readAll(t, putResp.Body)
	if putResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", putResp.StatusCode, body)
	}
}

func TestAgentSettingsPutAllowsLocalURLAndPersists(t *testing.T) {
	handler := New(Options{
		AppKeyPath:        filepath.Join(t.TempDir(), "app.key"),
		KVPath:            filepath.Join(t.TempDir(), "kv.json"),
		AgentLLMLocalOnly: true,
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodPut, server.URL+"/api/v1/agent/settings", strings.NewReader(`{"baseUrl":"http://127.0.0.1:1234/v1","model":"qwen3-8b","thinking":true}`))
	req.Header.Set("content-type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT settings: %v", err)
	}
	defer resp.Body.Close()
	body := readAll(t, resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(body), &settings); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if settings["baseUrl"] != "http://127.0.0.1:1234/v1" || settings["model"] != "qwen3-8b" || settings["thinking"] != true {
		t.Fatalf("unexpected saved settings: %#v", settings)
	}

	var reGet map[string]any
	getJSON(t, server.URL+"/api/v1/agent/settings", &reGet)
	if reGet["baseUrl"] != "http://127.0.0.1:1234/v1" || reGet["model"] != "qwen3-8b" {
		t.Fatalf("settings did not persist across requests: %#v", reGet)
	}
}

func TestAgentRoutesReturn403WhenDisabled(t *testing.T) {
	handler := New(Options{
		AppKeyPath:    filepath.Join(t.TempDir(), "app.key"),
		KVPath:        filepath.Join(t.TempDir(), "kv.json"),
		AgentDisabled: true,
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/agent/settings")
	if err != nil {
		t.Fatalf("GET settings: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestAgentChatReturns409WhenNotConfigured(t *testing.T) {
	handler := New(Options{
		AppKeyPath: filepath.Join(t.TempDir(), "app.key"),
		KVPath:     filepath.Join(t.TempDir(), "kv.json"),
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Post(server.URL+"/api/v1/agent/chat", "application/json", strings.NewReader(`{"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatalf("POST chat: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
}

// TestAgentTestReportsTruncatedReasoningRatherThanFlatToolCallsFalse reproduces what a small
// reasoning model (e.g. Gemma) does against LM Studio: it spends its token budget on
// reasoning_content and hits finish_reason "length" before ever emitting a tool call. Before the
// fix, /api/v1/agent/test reported a flat toolCalls:false, indistinguishable from "this model
// cannot call tools at all". It should instead say the probe was truncated.
func TestAgentTestReportsTruncatedReasoningRatherThanFlatToolCallsFalse(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		for _, line := range []string{
			`{"choices":[{"delta":{"reasoning_content":"thinking a lot before deciding"}}]}`,
			`{"choices":[{"delta":{},"finish_reason":"length"}]}`,
		} {
			_, _ = w.Write([]byte("data: " + line + "\n\n"))
			flusher.Flush()
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	handler := New(Options{
		AppKeyPath:      filepath.Join(t.TempDir(), "app.key"),
		KVPath:          filepath.Join(t.TempDir(), "kv.json"),
		AgentLLMBaseURL: upstream.URL,
		AgentLLMModel:   "small-reasoning-model",
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Post(server.URL+"/api/v1/agent/test", "application/json", nil)
	if err != nil {
		t.Fatalf("POST test: %v", err)
	}
	defer resp.Body.Close()
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["ok"] != true {
		t.Fatalf("expected ok=true (the call itself succeeded), got %#v", result)
	}
	if result["toolCalls"] != false {
		t.Fatalf("expected toolCalls=false, got %#v", result)
	}
	errMsg, _ := result["error"].(string)
	if !strings.Contains(errMsg, "ran out of tokens") {
		t.Fatalf("expected a truncation explanation, got %#v", result)
	}
}

func readAll(t *testing.T, r interface{ Read([]byte) (int, error) }) string {
	t.Helper()
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	for {
		n, err := r.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
	}
	return string(buf)
}

// TestAgentModelsSendsAPIKeyOnlyToSavedBaseURL is the regression test for the key-exfiltration
// fix: an ad-hoc ?baseUrl= probe (the settings UI uses this to test a URL before saving it) must
// never carry the saved/env API key to that other host, only the saved base URL should.
func TestAgentModelsSendsAPIKeyOnlyToSavedBaseURL(t *testing.T) {
	var savedAuth, otherAuth string
	saved := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		savedAuth = r.Header.Get("Authorization")
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"saved-model"}]}`))
	}))
	defer saved.Close()
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		otherAuth = r.Header.Get("Authorization")
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"other-model"}]}`))
	}))
	defer other.Close()

	handler := New(Options{
		AppKeyPath:      filepath.Join(t.TempDir(), "app.key"),
		KVPath:          filepath.Join(t.TempDir(), "kv.json"),
		AgentLLMBaseURL: saved.URL,
		AgentLLMModel:   "saved-model",
		AgentLLMAPIKey:  "super-secret-key",
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	probeResp, err := http.Get(server.URL + "/api/v1/agent/models?baseUrl=" + url.QueryEscape(other.URL))
	if err != nil {
		t.Fatalf("GET models (probe): %v", err)
	}
	defer probeResp.Body.Close()
	if probeResp.StatusCode != http.StatusOK {
		t.Fatalf("probe status = %d, body = %s", probeResp.StatusCode, readAll(t, probeResp.Body))
	}
	if otherAuth != "" {
		t.Fatalf("ad-hoc ?baseUrl= probe leaked the API key: Authorization=%q", otherAuth)
	}

	savedResp, err := http.Get(server.URL + "/api/v1/agent/models")
	if err != nil {
		t.Fatalf("GET models (saved): %v", err)
	}
	defer savedResp.Body.Close()
	if savedResp.StatusCode != http.StatusOK {
		t.Fatalf("saved status = %d, body = %s", savedResp.StatusCode, readAll(t, savedResp.Body))
	}
	if savedAuth != "Bearer super-secret-key" {
		t.Fatalf("saved base URL did not receive the API key: Authorization=%q", savedAuth)
	}
}

// TestAgentSettingsCSRFCrossOriginRefused and its same-origin counterpart cover the CSRF guard on
// /api/v1/agent/* routes.
func TestAgentSettingsCSRFCrossOriginRefused(t *testing.T) {
	handler := New(Options{
		AppKeyPath: filepath.Join(t.TempDir(), "app.key"),
		KVPath:     filepath.Join(t.TempDir(), "kv.json"),
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/agent/settings", nil)
	req.Header.Set("Origin", "http://evil.example")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET settings: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body = %s", resp.StatusCode, readAll(t, resp.Body))
	}
}

func TestAgentSettingsCSRFSameOriginAllowed(t *testing.T) {
	handler := New(Options{
		AppKeyPath: filepath.Join(t.TempDir(), "app.key"),
		KVPath:     filepath.Join(t.TempDir(), "kv.json"),
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/agent/settings", nil)
	req.Header.Set("Origin", server.URL)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET settings: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", resp.StatusCode, readAll(t, resp.Body))
	}
}

// TestOfficialSearchRejectsNonJSONContentType covers the CSRF guard's content-type check on a
// mutating official route, which sits outside /api/v1/agent/*.
func TestOfficialSearchRejectsNonJSONContentType(t *testing.T) {
	handler := New(Options{
		AppKeyPath: filepath.Join(t.TempDir(), "app.key"),
		KVPath:     filepath.Join(t.TempDir(), "kv.json"),
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Post(server.URL+"/api/v1/official/search", "text/plain", strings.NewReader("not json"))
	if err != nil {
		t.Fatalf("POST official/search: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415, body = %s", resp.StatusCode, readAll(t, resp.Body))
	}
}

// TestConcurrentSavesToBothKVStoresKeepBothEntries is the regression test for the KV-file race:
// agentSettingsStore and OfficialCredentialVault share one KV file, so a save on one must not
// clobber a concurrent save on the other via a stale read-modify-write.
func TestConcurrentSavesToBothKVStoresKeepBothEntries(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "app.key")
	kvPath := filepath.Join(dir, "kv.json")

	agentStore := newAgentSettingsStore(keyPath, kvPath)
	vault, err := NewOfficialCredentialVault(keyPath, kvPath)
	if err != nil {
		t.Fatalf("NewOfficialCredentialVault: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			_ = agentStore.save(agentLLMSettings{BaseURL: "http://127.0.0.1:1234/v1", Model: fmt.Sprintf("m%d", i)})
		}(i)
		go func(i int) {
			defer wg.Done()
			_ = vault.Save(context.Background(), OfficialCredentials{Username: "user", Password: fmt.Sprintf("pw%d", i)})
		}(i)
	}
	wg.Wait()

	settings, err := agentStore.load()
	if err != nil {
		t.Fatalf("agentStore.load: %v", err)
	}
	if settings.Model == "" {
		t.Fatalf("agent settings entry lost after concurrent saves: %+v", settings)
	}

	status, err := vault.Status(context.Background())
	if err != nil {
		t.Fatalf("vault.Status: %v", err)
	}
	if !status.Saved || !status.Usable {
		t.Fatalf("official credentials entry lost after concurrent saves: %+v", status)
	}
}
