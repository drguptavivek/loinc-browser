// Package semantic adds meaning-based search over LOINC terms: each term's name and synonyms are
// embedded once with an OpenAI-compatible embeddings endpoint (LM Studio, Ollama, vLLM, or a
// hosted API), stored in a SQLite file beside the database, and searched in memory by cosine
// similarity. Queries are embedded with the same endpoint at search time.
package semantic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

// Client calls an OpenAI-compatible POST {BaseURL}/embeddings endpoint.
type Client struct {
	BaseURL string // e.g. http://127.0.0.1:1234/v1
	Model   string
	APIKey  string
	HTTP    *http.Client
}

func NewClient(baseURL, model, apiKey string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		Model:   strings.TrimSpace(model),
		APIKey:  strings.TrimSpace(apiKey),
		// A cold model load in LM Studio can take tens of seconds on the first batch.
		HTTP: &http.Client{Timeout: 5 * time.Minute},
	}
}

// Embed returns one unit-length vector per input, in input order.
func (c *Client) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	body, err := json.Marshal(map[string]any{"model": c.Model, "input": inputs})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding endpoint %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return nil, fmt.Errorf("embedding endpoint %s: HTTP %d: %s", c.BaseURL, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	var parsed struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode embeddings: %w", err)
	}
	if len(parsed.Data) != len(inputs) {
		return nil, fmt.Errorf("embedding endpoint returned %d vectors for %d inputs", len(parsed.Data), len(inputs))
	}
	out := make([][]float32, len(inputs))
	for i, item := range parsed.Data {
		slot := item.Index
		if slot < 0 || slot >= len(out) || out[slot] != nil {
			slot = i
		}
		out[slot] = normalize(item.Embedding)
	}
	return out, nil
}

func normalize(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return v
	}
	scale := float32(1 / math.Sqrt(sum))
	for i := range v {
		v[i] *= scale
	}
	return v
}

// prefixes returns the query and document instructions an embedding model family expects;
// retrieval quality drops noticeably without them. Unknown models get none.
func prefixes(model string) (query, document string) {
	name := strings.ToLower(model)
	switch {
	case strings.Contains(name, "qwen3-embedding"):
		return "Instruct: Given a clinical lab or imaging test request, retrieve the matching LOINC term\nQuery: ", ""
	case strings.Contains(name, "embeddinggemma"):
		return "task: search result | query: ", "title: none | text: "
	case strings.Contains(name, "nomic-embed"):
		return "search_query: ", "search_document: "
	default:
		return "", ""
	}
}
