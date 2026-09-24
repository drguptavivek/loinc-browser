package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"hash/fnv"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"loinc-browser/internal/loinc"
	"loinc-browser/internal/semantic"
)

// embeddedTexts counts the texts fakeEmbeddings has embedded.
var embeddedTexts atomic.Int64

// fakeEmbeddings is an OpenAI-compatible /embeddings endpoint that hashes each word into one of
// 64 dimensions, so texts sharing words are close and results are deterministic.
func fakeEmbeddings(t *testing.T) *httptest.Server {
	t.Helper()
	word := regexp.MustCompile(`[a-z0-9]+`)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		type item struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		}
		out := struct {
			Data []item `json:"data"`
		}{}
		embeddedTexts.Add(int64(len(req.Input)))
		for i, text := range req.Input {
			vec := make([]float32, 64)
			for _, token := range word.FindAllString(strings.ToLower(text), -1) {
				h := fnv.New32a()
				h.Write([]byte(token))
				vec[h.Sum32()%64]++
			}
			out.Data = append(out.Data, item{Index: i, Embedding: vec})
		}
		writeJSON(w, http.StatusOK, out)
	}))
}

func TestSemanticSearchBuildStatusAndModes(t *testing.T) {
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

	embeddings := fakeEmbeddings(t)
	defer embeddings.Close()
	embeddingsPath := filepath.Join(t.TempDir(), "embeddings.sqlite")
	server := httptest.NewServer(New(Options{
		Store:          store,
		EmbeddingURL:   embeddings.URL,
		EmbeddingModel: "fake-embed",
		EmbeddingsPath: embeddingsPath,
	}))
	defer server.Close()

	var status semantic.Status
	getJSON(t, server.URL+"/api/v1/semantic/status", &status)
	if status.State != "missing" {
		t.Fatalf("expected missing meaning index, got %#v", status)
	}
	// Meaning search before the index exists is a 503, not a 500.
	if resp, err := http.Get(server.URL + "/api/v1/terms/search?q=glucose&mode=semantic"); err != nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 before build, got %v %v", resp.StatusCode, err)
	}

	rebuild := func() {
		t.Helper()
		resp, err := http.Post(server.URL+"/api/v1/semantic/rebuild", "application/json", nil)
		if err != nil || resp.StatusCode != http.StatusAccepted {
			t.Fatalf("expected 202 from rebuild, got %v %v", resp.StatusCode, err)
		}
		deadline := time.Now().Add(10 * time.Second)
		for status.State != "ready" && time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
			getJSON(t, server.URL+"/api/v1/semantic/status", &status)
		}
		if status.State != "ready" || status.Count == 0 {
			t.Fatalf("expected ready meaning index, got %#v", status)
		}
	}
	rebuild()
	count := status.Count

	// Changed lay phrases make the index stale, and a rebuild re-embeds only the lay-phrase terms.
	db, err := sql.Open("sqlite", embeddingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`update meta set value = 'old' where key = 'lay_phrases'`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	getJSON(t, server.URL+"/api/v1/semantic/status", &status)
	if status.State != "stale" {
		t.Fatalf("expected stale after lay phrases changed, got %#v", status)
	}
	embeddedTexts.Store(0)
	rebuild()
	if status.Count != count || embeddedTexts.Load() >= int64(count) {
		t.Fatalf("expected a partial re-embed keeping %d vectors, got count %d after embedding %d texts", count, status.Count, embeddedTexts.Load())
	}

	var semanticResult loinc.SearchResponse
	getJSON(t, server.URL+"/api/v1/terms/search?q=platelets%20in%20blood&mode=semantic", &semanticResult)
	if semanticResult.Mode != "semantic" || len(semanticResult.Results) == 0 || semanticResult.Results[0].LOINCNum != "2001-9" {
		t.Fatalf("expected the platelet term first by meaning, got %#v", semanticResult)
	}
	for _, result := range semanticResult.Results {
		if result.Status == "DEPRECATED" {
			t.Fatalf("meaning search must keep the default deprecated filter, got %#v", result)
		}
	}

	var hybrid loinc.SearchResponse
	getJSON(t, server.URL+"/api/v1/terms/search?q=cholesterol&mode=hybrid", &hybrid)
	if hybrid.Mode != "hybrid" || hybrid.Total == 0 {
		t.Fatalf("expected hybrid results, got %#v", hybrid)
	}

	if resp, err := http.Get(server.URL + "/api/v1/terms/search?q=glucose&mode=bogus"); err != nil || resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown mode, got %v %v", resp.StatusCode, err)
	}
}

func TestSemanticSearchDisabledWithoutEndpoint(t *testing.T) {
	server := httptest.NewServer(New(Options{}))
	defer server.Close()
	var status semantic.Status
	getJSON(t, server.URL+"/api/v1/semantic/status", &status)
	if status.State != "disabled" {
		t.Fatalf("expected disabled status, got %#v", status)
	}
}
