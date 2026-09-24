package semantic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"

	"loinc-browser/internal/loinc"
)

// ErrNotReady means meaning-based search can't run yet: no endpoint configured, or the index is
// missing, building, or from another model. Callers answer 503.
var ErrNotReady = errors.New("meaning-based search is not ready")

const (
	buildBatchSize = 128
	// candidatePool is how many nearest terms are fetched before filters (status, classType, ...)
	// and fusion; filters can discard most of them, so it is well above any page size.
	candidatePool = 300
	// rrfK is the reciprocal-rank-fusion constant: fused score = sum 1/(rrfK + rank).
	rrfK = 60
	// popularityWeight/Scale add up to 12 points (cosine x100) for the most-used terms, decaying
	// with common test rank. Tuned on a 20-query natural-language probe set: top-1 went 4 -> 10.
	popularityWeight = 12.0
	popularityScale  = 200.0
)

// Status reports the meaning index for GET /api/v1/semantic/status.
type Status struct {
	State    string `json:"state"` // missing, building, incomplete, stale, ready, error
	Model    string `json:"model"`
	Endpoint string `json:"endpoint"`
	Count    int    `json:"count"`
	Done     int64  `json:"done,omitempty"`
	Total    int64  `json:"total,omitempty"`
	BuiltAt  string `json:"builtAt,omitempty"`
	Building bool   `json:"building,omitempty"`
	Message  string `json:"message,omitempty"`
}

// Service owns the embedding client, the vector file, and the loaded index.
type Service struct {
	client *Client
	path   string

	mu    sync.Mutex
	index *Index

	building atomic.Bool
	done     atomic.Int64
	total    atomic.Int64
	lastErr  atomic.Value // string
}

func NewService(client *Client, path string) *Service {
	return &Service{client: client, path: path}
}

func (s *Service) open() (*sql.DB, error) {
	db, err := sql.Open("sqlite", s.path)
	if err != nil {
		return nil, err
	}
	for _, stmt := range []string{
		`pragma journal_mode = wal`,
		`pragma busy_timeout = 5000`,
		`create table if not exists meta (key text primary key, value text not null)`,
		`create table if not exists vectors (loinc_num text primary key, vec blob not null)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return nil, fmt.Errorf("open meaning index %s: %w", s.path, err)
		}
	}
	return db, nil
}

func metaGet(db *sql.DB, key string) string {
	var value string
	_ = db.QueryRow(`select value from meta where key = ?`, key).Scan(&value)
	return value
}

func metaSet(db *sql.DB, key, value string) error {
	_, err := db.Exec(`insert into meta(key, value) values(?, ?) on conflict(key) do update set value = excluded.value`, key, value)
	return err
}

func importStamp(ctx context.Context, store *loinc.Store) string {
	importedAt, err := store.ImportedAt(ctx)
	if err != nil {
		return ""
	}
	return importedAt.UTC().Format(time.RFC3339)
}

// Status never blocks on a running build.
func (s *Service) Status(ctx context.Context, store *loinc.Store) Status {
	status := Status{Model: s.client.Model, Endpoint: s.client.BaseURL}
	if s.building.Load() {
		status.State, status.Building = "building", true
		status.Done, status.Total = s.done.Load(), s.total.Load()
		status.Message = fmt.Sprintf("building: %d of %d terms embedded", status.Done, status.Total)
		return status
	}
	db, err := s.open()
	if err != nil {
		status.State, status.Message = "error", err.Error()
		return status
	}
	defer db.Close()
	_ = db.QueryRow(`select count(*) from vectors`).Scan(&status.Count)
	status.BuiltAt = metaGet(db, "built_at")
	lastErr, _ := s.lastErr.Load().(string)
	switch {
	case status.Count == 0:
		status.State, status.Message = "missing", "meaning index has not been built; POST /api/v1/semantic/rebuild"
	case metaGet(db, "model") != s.client.Model:
		status.State = "missing"
		status.Message = fmt.Sprintf("meaning index was built with %q, not %q; rebuild it", metaGet(db, "model"), s.client.Model)
	case metaGet(db, "complete") != "1":
		status.State = "incomplete"
		status.Message = "meaning index build did not finish; rebuild resumes where it stopped"
	case store != nil && metaGet(db, "source_imported_at") != importStamp(ctx, store):
		status.State = "stale"
		status.Message = "meaning index predates the current LOINC import; results may be outdated; rebuild it"
	case metaGet(db, "lay_phrases") != loinc.LayPhrasesVersion():
		status.State = "stale"
		status.Message = "meaning index predates the current lay phrases; rebuild re-embeds only those terms"
	default:
		status.State, status.Message = "ready", "meaning index is ready"
	}
	if lastErr != "" && status.State != "ready" {
		status.Message += " (last build error: " + lastErr + ")"
	}
	return status
}

// Rebuild starts a background build and returns at once. A build that stopped part-way for the
// same import and model resumes; otherwise every term is re-embedded.
func (s *Service) Rebuild(store *loinc.Store) error {
	if store == nil {
		return errors.New("LOINC database is not loaded")
	}
	if !s.building.CompareAndSwap(false, true) {
		return errors.New("a meaning index build is already running")
	}
	s.lastErr.Store("")
	go func() {
		defer s.building.Store(false)
		if err := s.build(context.Background(), store); err != nil {
			s.lastErr.Store(err.Error())
			log.Printf("meaning index build: %v", err)
		}
	}()
	return nil
}

func (s *Service) build(ctx context.Context, store *loinc.Store) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()
	stamp := importStamp(ctx, store)
	layVersion := loinc.LayPhrasesVersion()
	switch {
	case metaGet(db, "complete") == "1" && metaGet(db, "model") == s.client.Model &&
		metaGet(db, "source_imported_at") == stamp && metaGet(db, "lay_phrases") != layVersion:
		// Only the lay phrases changed: re-embed just those terms.
		for _, num := range loinc.LayPhraseTerms() {
			if _, err := db.Exec(`delete from vectors where loinc_num = ?`, num); err != nil {
				return err
			}
		}
	case metaGet(db, "model") != s.client.Model || metaGet(db, "building_for") != stamp || metaGet(db, "complete") == "1":
		if _, err := db.Exec(`delete from vectors`); err != nil {
			return err
		}
	}
	for key, value := range map[string]string{"model": s.client.Model, "building_for": stamp, "complete": "0"} {
		if err := metaSet(db, key, value); err != nil {
			return err
		}
	}

	existing := map[string]bool{}
	rows, err := db.Query(`select loinc_num from vectors`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var num string
		if err := rows.Scan(&num); err != nil {
			rows.Close()
			return err
		}
		existing[num] = true
	}
	rows.Close()

	type doc struct{ num, text string }
	var pending []doc
	total := 0
	if err := store.VisitEmbeddingDocuments(ctx, func(num, text string) error {
		total++
		if !existing[num] {
			pending = append(pending, doc{num, text})
		}
		return nil
	}); err != nil {
		return err
	}
	s.total.Store(int64(total))
	s.done.Store(int64(total - len(pending)))

	_, docPrefix := prefixes(s.client.Model)
	dims := 0
	for start := 0; start < len(pending); start += buildBatchSize {
		batch := pending[start:min(start+buildBatchSize, len(pending))]
		inputs := make([]string, len(batch))
		for i, d := range batch {
			inputs[i] = docPrefix + d.text
		}
		vectors, err := s.client.Embed(ctx, inputs)
		if err != nil {
			return err
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		for i, d := range batch {
			dims = len(vectors[i])
			q := quantize(vectors[i])
			blob := make([]byte, len(q))
			for j, x := range q {
				blob[j] = byte(x)
			}
			if _, err := tx.Exec(`insert or replace into vectors(loinc_num, vec) values(?, ?)`, d.num, blob); err != nil {
				tx.Rollback()
				return err
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		s.done.Add(int64(len(batch)))
	}
	if dims > 0 {
		if err := metaSet(db, "dims", fmt.Sprint(dims)); err != nil {
			return err
		}
	}
	for key, value := range map[string]string{"source_imported_at": stamp, "lay_phrases": layVersion, "built_at": time.Now().UTC().Format(time.RFC3339), "complete": "1"} {
		if err := metaSet(db, key, value); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.index = nil // reload on next search
	s.mu.Unlock()
	return nil
}

func (s *Service) loadIndex() (*Index, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.index != nil {
		return s.index, nil
	}
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var dims, count int
	fmt.Sscan(metaGet(db, "dims"), &dims)
	if err := db.QueryRow(`select count(*) from vectors`).Scan(&count); err != nil {
		return nil, err
	}
	idx := &Index{dims: dims, ids: make([]string, 0, count), data: make([]int8, 0, count*dims)}
	rows, err := db.Query(`select loinc_num, vec from vectors`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var num string
		var blob []byte
		if err := rows.Scan(&num, &blob); err != nil {
			return nil, err
		}
		if len(blob) != dims {
			continue
		}
		idx.ids = append(idx.ids, num)
		for _, b := range blob {
			idx.data = append(idx.data, int8(b))
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.index = idx
	return idx, nil
}

// Search answers mode "semantic" (nearest terms by meaning) or "hybrid" (meaning and word search
// merged by reciprocal rank fusion). Every filter in params applies, including the default that
// hides deprecated terms. Scores are only comparable within one response.
func (s *Service) Search(ctx context.Context, store *loinc.Store, params loinc.SearchParams, mode string) (loinc.SearchResponse, error) {
	params = loinc.NormalizeTermListParams(params)
	query := strings.TrimSpace(params.Query)
	if query == "" {
		return loinc.SearchResponse{}, fmt.Errorf("%w: mode=%s needs a q", loinc.ErrInvalidParam, mode)
	}
	status := s.Status(ctx, store)
	if status.State != "ready" && status.State != "stale" {
		return loinc.SearchResponse{}, fmt.Errorf("%w: %s", ErrNotReady, status.Message)
	}
	idx, err := s.loadIndex()
	if err != nil {
		return loinc.SearchResponse{}, err
	}
	queryPrefix, _ := prefixes(s.client.Model)
	vectors, err := s.client.Embed(ctx, []string{queryPrefix + query})
	if err != nil {
		return loinc.SearchResponse{}, fmt.Errorf("%w: %v", ErrNotReady, err)
	}
	hits := idx.TopK(vectors[0], candidatePool)

	// Apply the caller's filters to the nearest terms and fetch their rows.
	candidateParams := params
	candidateParams.Query, candidateParams.Sort, candidateParams.Offset = "", "alpha", 0
	candidateParams.LOINCNums = make([]string, len(hits))
	for i, hit := range hits {
		candidateParams.LOINCNums[i] = hit.LOINCNum
	}
	candidateParams.Limit, candidateParams.MaxLimit = len(hits), len(hits)
	candidates, err := store.Search(ctx, candidateParams)
	if err != nil {
		return loinc.SearchResponse{}, err
	}
	rows := map[string]loinc.SearchResult{}
	for _, result := range candidates.Results {
		rows[result.LOINCNum] = result
	}

	// Cosine alone favours obscure near-synonyms ("platelets" -> a platelet-volume term), so add
	// the same kind of popularity prior word search uses before ranking.
	meaning := make([]Hit, 0, len(hits))
	for _, hit := range hits {
		row, ok := rows[hit.LOINCNum]
		if !ok {
			continue
		}
		score := float64(hit.Score) * 100
		if row.CommonTestRank > 0 {
			score += popularityWeight / (1 + float64(row.CommonTestRank)/popularityScale)
		}
		meaning = append(meaning, Hit{LOINCNum: hit.LOINCNum, Score: float32(score)})
	}
	sort.SliceStable(meaning, func(a, b int) bool { return meaning[a].Score > meaning[b].Score })
	scores := map[string]float64{}
	for i, hit := range meaning {
		if mode == "semantic" {
			scores[hit.LOINCNum] = float64(hit.Score)
		} else {
			scores[hit.LOINCNum] = 1000.0 / float64(rrfK+i+1)
		}
	}
	response := loinc.SearchResponse{Query: query, Mode: mode}
	if mode == "hybrid" {
		wordParams := params
		wordParams.Offset, wordParams.Limit, wordParams.MaxLimit = 0, candidatePool, candidatePool
		words, err := store.Search(ctx, wordParams)
		if err != nil {
			return loinc.SearchResponse{}, err
		}
		for i, result := range words.Results {
			rows[result.LOINCNum] = result
			scores[result.LOINCNum] += 1000.0 / float64(rrfK+i+1)
		}
		response.Relaxed, response.DroppedWords = words.Relaxed, words.DroppedWords
	}

	order := make([]string, 0, len(scores))
	for num := range scores {
		order = append(order, num)
	}
	sort.Slice(order, func(a, b int) bool {
		if scores[order[a]] != scores[order[b]] {
			return scores[order[a]] > scores[order[b]]
		}
		return order[a] < order[b]
	})
	response.Total = len(order)
	response.Limit, response.Offset = params.Limit, params.Offset
	for i := params.Offset; i < len(order) && i < params.Offset+params.Limit; i++ {
		result := rows[order[i]]
		result.Rank = -scores[order[i]]
		response.Results = append(response.Results, result)
	}
	response.HasMore = params.Offset+len(response.Results) < len(order)
	if response.Results == nil {
		response.Results = []loinc.SearchResult{}
	}
	if status.State == "stale" {
		response.Notice = "The meaning index predates the current LOINC import; rebuild it for current results."
	}
	return response, nil
}
