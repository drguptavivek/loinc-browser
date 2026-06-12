package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"loinc-browser/internal/loinc"
)

type Options struct {
	Store        *loinc.Store
	Assets       http.FileSystem
	DBPath       string
	UploadDir    string
	CacheEntries int
}

func New(options Options) http.Handler {
	app := &app{
		store:        options.Store,
		assets:       options.Assets,
		dbPath:       options.DBPath,
		uploadDir:    options.UploadDir,
		cacheEntries: options.CacheEntries,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", app.health)
	mux.HandleFunc("GET /api/search", app.search)
	mux.HandleFunc("GET /api/terms/{loincNum}", app.term)
	mux.HandleFunc("GET /api/terms/{loincNum}/relationships", app.termRelationships)
	mux.HandleFunc("GET /api/facets", app.facets)
	mux.HandleFunc("GET /api/source-organizations", app.sourceOrganizations)
	mux.HandleFunc("GET /api/accessories", app.accessories)
	mux.HandleFunc("GET /api/cache", app.cacheStats)
	mux.HandleFunc("POST /api/import/upload", app.uploadImport)
	mux.HandleFunc("GET /openapi.json", app.openapi)
	mux.HandleFunc("/", app.frontend)
	return mux
}

type app struct {
	mu           sync.RWMutex
	store        *loinc.Store
	assets       http.FileSystem
	dbPath       string
	uploadDir    string
	cacheEntries int
}

func (a *app) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *app) search(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	query := r.URL.Query()
	params := loinc.SearchParams{
		Query:          query.Get("q"),
		Class:          query.Get("class"),
		Statuses:       queryValues(query, "status"),
		System:         query.Get("system"),
		TimeAspects:    queryValues(query, "timeAspect"),
		Scales:         queryValues(query, "scale"),
		Methods:        queryValues(query, "method"),
		Property:       query.Get("property"),
		OrderObsValues: queryValues(query, "orderObs"),
		Limit:          parseInt(query.Get("limit"), 25),
		Offset:         parseInt(query.Get("offset"), 0),
	}
	response, err := store.Search(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *app) term(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	includeRelationships := r.URL.Query().Get("include") == "relationships"
	var term loinc.Term
	if includeRelationships {
		term, err = store.TermWithAccessories(r.Context(), r.PathValue("loincNum"))
	} else {
		term, err = store.Term(r.Context(), r.PathValue("loincNum"))
	}
	if err != nil {
		if errors.Is(err, loinc.ErrNotFound) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, term)
}

func (a *app) termRelationships(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	graph, err := store.TermRelationships(r.Context(), r.PathValue("loincNum"))
	if err != nil {
		if errors.Is(err, loinc.ErrNotFound) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, graph)
}

func (a *app) facets(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	facets, err := store.Facets(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, facets)
}

func (a *app) cacheStats(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	writeJSON(w, http.StatusOK, store.CacheStats())
}

func (a *app) sourceOrganizations(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	items, err := store.SourceOrganizations(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *app) accessories(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	query := r.URL.Query()
	response, err := store.BrowseAccessories(r.Context(), loinc.AccessoryBrowseParams{
		Kind:   query.Get("kind"),
		Query:  query.Get("q"),
		Limit:  parseInt(query.Get("limit"), 50),
		Offset: parseInt(query.Get("offset"), 0),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *app) openapi(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, openAPISpec)
}

func (a *app) currentStore() (*loinc.Store, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.store == nil {
		return nil, errors.New("LOINC database is not loaded")
	}
	return a.store, nil
}

func (a *app) swapStore(store *loinc.Store) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	old := a.store
	a.store = store
	if old != nil {
		return old.Close()
	}
	return nil
}

func (a *app) frontend(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	if a.assets == nil {
		w.Header().Set("content-type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<html><body><h1>LOINC Browser</h1><p>Frontend assets are not built yet.</p></body></html>`)
		return
	}
	http.FileServer(a.assets).ServeHTTP(w, r)
}

func parseInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func queryValues(values map[string][]string, key string) []string {
	raw := values[key]
	filtered := make([]string, 0, len(raw))
	for _, value := range raw {
		value = strings.TrimSpace(value)
		if value != "" {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{
		"error": err.Error(),
	})
}
