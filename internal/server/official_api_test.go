package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"loinc-browser/internal/loinc"
)

func TestOfficialCredentialVaultEncryptsCredentialsInFileKV(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "loinc-browser-app.key")
	kvPath := filepath.Join(dir, "loinc-browser-kv.json")

	vault, err := NewOfficialCredentialVault(keyPath, kvPath)
	if err != nil {
		t.Fatalf("create vault: %v", err)
	}
	if err := vault.Save(t.Context(), OfficialCredentials{Username: "alice@example.org", Password: "top-secret"}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}

	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read key file: %v", err)
	}
	decodedKey, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(keyBytes)))
	if err != nil {
		t.Fatalf("decode key: %v", err)
	}
	if len(decodedKey) != 32 {
		t.Fatalf("expected 32-byte app key, got %d", len(decodedKey))
	}
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat key file: %v", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("expected restrictive key permissions, got %v", info.Mode().Perm())
	}

	kvBytes, err := os.ReadFile(kvPath)
	if err != nil {
		t.Fatalf("read kv file: %v", err)
	}
	kvText := string(kvBytes)
	for _, secret := range []string{"alice@example.org", "top-secret"} {
		if strings.Contains(kvText, secret) {
			t.Fatalf("kv file leaked secret %q: %s", secret, kvText)
		}
	}
	if !strings.Contains(kvText, "official_api.credentials") {
		t.Fatalf("expected official credential key in kv file, got %s", kvText)
	}

	reopened, err := NewOfficialCredentialVault(keyPath, kvPath)
	if err != nil {
		t.Fatalf("reopen vault: %v", err)
	}
	credentials, err := reopened.Load(t.Context())
	if err != nil {
		t.Fatalf("load credentials: %v", err)
	}
	if credentials.Username != "alice@example.org" || credentials.Password != "top-secret" {
		t.Fatalf("unexpected credentials round trip: %#v", credentials)
	}

	status, err := reopened.Status(t.Context())
	if err != nil {
		t.Fatalf("credential status: %v", err)
	}
	if !status.Saved || !status.Usable || status.MaskedUsername == "" {
		t.Fatalf("expected usable saved credential status, got %#v", status)
	}
}

func TestOfficialCredentialVaultReportsMalformedKVAsUnavailable(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "loinc-browser-app.key")
	kvPath := filepath.Join(dir, "loinc-browser-kv.json")
	vault, err := NewOfficialCredentialVault(keyPath, kvPath)
	if err != nil {
		t.Fatalf("create vault: %v", err)
	}
	if err := os.WriteFile(kvPath, []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("write malformed kv: %v", err)
	}

	status, err := vault.Status(t.Context())
	if err == nil {
		t.Fatal("expected malformed kv status error")
	}
	if status.Usable || !strings.Contains(err.Error(), "settings unavailable") {
		t.Fatalf("expected sanitized unavailable status and error, got status=%#v err=%v", status, err)
	}
}

func TestOfficialSearchEndpointForwardsParametersAndBasicAuth(t *testing.T) {
	var upstreamAuth string
	var upstreamPath string
	var upstreamQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamAuth = r.Header.Get("Authorization")
		upstreamPath = r.URL.Path
		upstreamQuery = r.URL.RawQuery
		writeJSON(w, http.StatusOK, map[string]any{
			"results": []map[string]any{{"LOINC_NUM": "2339-0", "LONG_COMMON_NAME": "Glucose [Mass/volume] in Blood"}},
			"total":   1,
		})
	}))
	defer upstream.Close()

	handler := New(Options{
		OfficialAPIBaseURL: upstream.URL,
		AppKeyPath:         filepath.Join(t.TempDir(), "app.key"),
		KVPath:             filepath.Join(t.TempDir(), "kv.json"),
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	body := `{
		"scope": "loincs",
		"query": "Component:glucose System:blood",
		"rows": 5,
		"offset": 10,
		"sortorder": "loinc_num desc",
		"language": 1,
		"includefiltercounts": true,
		"username": "demo-user",
		"password": "demo-pass"
	}`
	var result OfficialSearchResponse
	postJSON(t, server.URL+"/api/v1/official/search", body, &result)

	if upstreamPath != "/loincs" {
		t.Fatalf("expected loincs upstream path, got %q", upstreamPath)
	}
	for _, expected := range []string{
		"query=Component%3Aglucose+System%3Ablood",
		"rows=5",
		"offset=10",
		"sortorder=loinc_num+desc",
		"language=1",
		"includefiltercounts=true",
	} {
		if !strings.Contains(upstreamQuery, expected) {
			t.Fatalf("expected upstream query %q to contain %q", upstreamQuery, expected)
		}
	}
	if upstreamAuth != "Basic "+base64.StdEncoding.EncodeToString([]byte("demo-user:demo-pass")) {
		t.Fatalf("unexpected upstream auth header %q", upstreamAuth)
	}
	if result.Scope != "loincs" || result.UpstreamStatus != http.StatusOK {
		t.Fatalf("unexpected official response envelope: %#v", result)
	}
}

func TestOfficialSearchEndpointAddsLocalMatches(t *testing.T) {
	ctx := context.Background()
	releaseDir := writeServerTestRelease(t)
	dbPath := filepath.Join(t.TempDir(), "loinc.sqlite")
	if _, err := loinc.Ingest(ctx, loinc.IngestOptions{ReleaseDir: releaseDir, DBPath: dbPath}); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	store, err := loinc.OpenStore(dbPath, loinc.StoreOptions{CacheEntries: 4})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"Results": []map[string]any{
				{"LOINC_NUM": "2000-1", "LONG_COMMON_NAME": "Official cholesterol"},
				{"LOINC_NUM": "9999-9", "LONG_COMMON_NAME": "Not in local fixture"},
			},
		})
	}))
	defer upstream.Close()

	server := httptest.NewServer(New(Options{
		Store:              store,
		OfficialAPIBaseURL: upstream.URL,
		AppKeyPath:         filepath.Join(t.TempDir(), "app.key"),
		KVPath:             filepath.Join(t.TempDir(), "kv.json"),
	}))
	defer server.Close()

	var result OfficialSearchResponse
	postJSON(t, server.URL+"/api/v1/official/search", `{"scope":"loincs","query":"cholesterol","username":"demo-user","password":"demo-pass"}`, &result)

	if result.Local == nil || !result.Local.Available {
		t.Fatalf("expected available local integration, got %#v", result.Local)
	}
	if result.Local.Matched != 1 || result.Local.Missing != 1 {
		t.Fatalf("expected one matched and one missing LOINC, got %#v", result.Local)
	}
	matched := result.Local.Matches["2000-1"]
	if !matched.Found || matched.Term == nil || matched.Term.LongCommonName != "Cholesterol [Mass/volume] in Serum" {
		t.Fatalf("expected 2000-1 local term match, got %#v", matched)
	}
	if matched.LocalURL != "/api/v1/terms/2000-1" {
		t.Fatalf("expected local URL for 2000-1, got %q", matched.LocalURL)
	}
	if missing := result.Local.Matches["9999-9"]; missing.Found {
		t.Fatalf("expected 9999-9 to be marked missing, got %#v", missing)
	}
}

func TestOfficialSearchCanSaveAndReuseCredentials(t *testing.T) {
	var seenAuth []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = append(seenAuth, r.Header.Get("Authorization"))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}))
	defer upstream.Close()

	dir := t.TempDir()
	server := httptest.NewServer(New(Options{
		OfficialAPIBaseURL: upstream.URL,
		AppKeyPath:         filepath.Join(dir, "app.key"),
		KVPath:             filepath.Join(dir, "kv.json"),
	}))
	defer server.Close()

	postJSON(t, server.URL+"/api/v1/official/search", `{"scope":"parts","query":"Part:glucose","username":"saved-user","password":"saved-pass","remember":true}`, &OfficialSearchResponse{})
	postJSON(t, server.URL+"/api/v1/official/search", `{"scope":"parts","query":"Part:glucose","useSavedCredentials":true}`, &OfficialSearchResponse{})
	if len(seenAuth) != 2 || seenAuth[0] == "" || seenAuth[0] != seenAuth[1] {
		t.Fatalf("expected direct and saved credentials to use same auth, got %#v", seenAuth)
	}

	var status OfficialCredentialStatus
	getJSON(t, server.URL+"/api/v1/official/credentials/status", &status)
	if !status.Saved || !status.Usable || status.MaskedUsername == "" {
		t.Fatalf("expected saved credential status, got %#v", status)
	}

	req, err := http.NewRequest(http.MethodDelete, server.URL+"/api/v1/official/credentials", nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete saved credentials: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected delete 200, got %d: %s", resp.StatusCode, body)
	}
	getJSON(t, server.URL+"/api/v1/official/credentials/status", &status)
	if status.Saved || status.Usable {
		t.Fatalf("expected deleted credential status, got %#v", status)
	}
}

func TestOfficialSearchEndpointRejectsMissingCredentials(t *testing.T) {
	server := httptest.NewServer(New(Options{
		OfficialAPIBaseURL: "http://127.0.0.1:1",
		AppKeyPath:         filepath.Join(t.TempDir(), "app.key"),
		KVPath:             filepath.Join(t.TempDir(), "kv.json"),
	}))
	defer server.Close()

	resp, err := http.Post(server.URL+"/api/v1/official/search", "application/json", strings.NewReader(`{"scope":"loincs","query":"glucose"}`))
	if err != nil {
		t.Fatalf("post official search: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 401 for missing credentials, got %d: %s", resp.StatusCode, body)
	}
}

func postJSON(t *testing.T, url string, body string, target any) {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post json: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 2xx, got %d: %s", resp.StatusCode, responseBody)
	}
	if target == nil {
		return
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func TestOfficialSearchRequestDoesNotAcceptCredentialQueryParams(t *testing.T) {
	server := httptest.NewServer(New(Options{OfficialAPIBaseURL: "http://127.0.0.1:1"}))
	defer server.Close()
	resp, err := http.Get(server.URL + "/api/v1/official/search?username=leak&password=leak")
	if err != nil {
		t.Fatalf("get official search: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed && resp.StatusCode != http.StatusNotFound {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, resp.Body)
		t.Fatalf("expected GET official search to be unavailable, got %d: %s", resp.StatusCode, buf.String())
	}
}
