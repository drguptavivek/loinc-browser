package server

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"loinc-browser/internal/loinc"
)

const (
	searchAPIDefaultRows  = 20
	searchAPIMaxRows      = 500
	searchAPIFacetScanCap = 20000
	searchAPIMissingIndex = "local search index not built; POST /api/v1/local-search/rebuild"
)

type searchAPIResponseSummary struct {
	RecordsFound       int     `json:"RecordsFound"`
	StartingOffset     int     `json:"StartingOffset"`
	RowsReturned       int     `json:"RowsReturned"`
	LoincVersion       string  `json:"LoincVersion"`
	Copyright          string  `json:"Copyright"`
	QueryUrl           string  `json:"QueryUrl"`
	Next               string  `json:"Next,omitempty"`
	Previous           string  `json:"Previous,omitempty"`
	QueryExecutionTime string  `json:"QueryExecutionTime"`
	QueryDuration      float64 `json:"QueryDuration"`
}

type searchAPIResponse struct {
	ResponseSummary searchAPIResponseSummary                `json:"ResponseSummary"`
	Results         []any                                   `json:"Results"`
	FilterCounts    map[string][]loinc.SearchAPIFilterCount `json:"FilterCounts,omitempty"`
}

// searchAPI serves GET /searchapi/{loincs|parts|answerlists|groups}: a local,
// wire-compatible clone of https://loinc.regenstrief.org/searchapi (plan
// §5). It answers entirely from the local database and the existing Bleve
// local-search index (local_search.go); it never calls the network. Basic
// auth headers are accepted and ignored, matching upstream client requests
// that send them.
//
// Documented gaps versus the captured exemplars (docs/exemplars/searchapi/),
// all because the underlying data is not in the LOINC release:
//   - loincs rows: CodeSystems and Tags are always `[]`; LHCForms is always
//     "false"; TermDescriptions reproduces only DefinitionDescription (no
//     Url/Copyright, which upstream sources from elsewhere).
//   - answerlists rows: the list-level Description is always null — the
//     release CSV has one Description column, used at the per-answer level.
//   - FilterCounts (loincs only): System, Method, Property, Timing, Scale,
//     Class, Status, OrderObs, ClassType, VersionFirstReleased,
//     VersionLastChanged, PanelType, and HL7AttachmentStructure are computed
//     locally; Tags and CodeSystems facets are omitted (same reason as
//     above). FilterCounts is only ever emitted for the loincs scope.
func (a *app) searchAPI(w http.ResponseWriter, r *http.Request) {
	scope, ok := normalizeLocalSearchScope(r.PathValue("scope"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"Message": fmt.Sprintf("unsupported search scope %q", r.PathValue("scope"))})
		return
	}
	store, err := a.currentStore()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"Message": "local LOINC database is not loaded"})
		return
	}

	start := time.Now()
	query := r.URL.Query()
	rows := parseSearchAPIRows(query.Get("rows"))
	offset := parseSearchAPIOffset(query.Get("offset"))
	includeFilterCounts := parseBool(query.Get("includefiltercounts"))
	language := strings.TrimSpace(query.Get("language"))
	sortBy, _ := searchAPISortField(scope, query.Get("sortorder"))

	response, status, err := a.localSearch.query(r.Context(), store, LocalSearchRequest{
		Scope:    scope,
		Query:    query.Get("query"),
		Limit:    rows,
		Offset:   offset,
		MaxLimit: searchAPIMaxRows,
		SortBy:   sortBy,
	})
	if err != nil {
		if status == http.StatusServiceUnavailable {
			writeJSON(w, status, map[string]string{"Message": searchAPIMissingIndex})
			return
		}
		writeJSON(w, status, map[string]string{"Message": err.Error()})
		return
	}

	results := make([]any, 0, len(response.Results))
	for _, hit := range response.Results {
		row, ok, err := searchAPIRow(r.Context(), store, scope, hit.Key, language)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"Message": err.Error()})
			return
		}
		if !ok {
			continue
		}
		results = append(results, row)
	}

	total := int(response.Total)
	summary := searchAPIResponseSummary{
		RecordsFound:       total,
		StartingOffset:     offset,
		RowsReturned:       len(results),
		Copyright:          fmt.Sprintf("Copyright © %d Regenstrief Institute, Inc. All Rights Reserved.", time.Now().Year()),
		QueryUrl:           searchAPIAbsoluteURL(r),
		QueryExecutionTime: time.Now().Format(time.RFC3339),
		QueryDuration:      time.Since(start).Seconds(),
	}
	if version, err := store.ReleaseVersion(r.Context()); err == nil {
		summary.LoincVersion = version
	}
	if offset+rows < total {
		summary.Next = searchAPIPageURL(r, offset+rows)
	}
	if offset > 0 {
		previousOffset := offset - rows
		if previousOffset < 0 {
			previousOffset = 0
		}
		summary.Previous = searchAPIPageURL(r, previousOffset)
	}

	out := searchAPIResponse{ResponseSummary: summary, Results: results}
	if includeFilterCounts && scope == "loincs" {
		keys, _, err := a.localSearch.matchingKeys(r.Context(), scope, query.Get("query"), searchAPIFacetScanCap)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"Message": err.Error()})
			return
		}
		counts, err := store.SearchAPILoincFilterCounts(r.Context(), keys)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"Message": err.Error()})
			return
		}
		out.FilterCounts = counts
	}
	writeJSON(w, http.StatusOK, out)
}

func searchAPIRow(ctx context.Context, store *loinc.Store, scope, key, language string) (any, bool, error) {
	switch scope {
	case "loincs":
		return store.SearchAPILoincRow(ctx, key, language)
	case "parts":
		return store.SearchAPIPartRow(ctx, key)
	case "answerlists":
		return store.SearchAPIAnswerListRow(ctx, key)
	case "groups":
		return store.SearchAPIGroupRow(ctx, key)
	default:
		return nil, false, nil
	}
}

func parseSearchAPIRows(raw string) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		return searchAPIDefaultRows
	}
	if n > searchAPIMaxRows {
		return searchAPIMaxRows
	}
	return n
}

func parseSearchAPIOffset(raw string) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func searchAPIScheme(r *http.Request) string {
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func searchAPIHost(r *http.Request) string {
	if host := r.Header.Get("X-Forwarded-Host"); host != "" {
		return host
	}
	return r.Host
}

func searchAPIAbsoluteURL(r *http.Request) string {
	u := *r.URL
	u.Scheme = searchAPIScheme(r)
	u.Host = searchAPIHost(r)
	return u.String()
}

func searchAPIPageURL(r *http.Request, offset int) string {
	u := *r.URL
	u.Scheme = searchAPIScheme(r)
	u.Host = searchAPIHost(r)
	values := u.Query()
	values.Set("offset", strconv.Itoa(offset))
	u.RawQuery = values.Encode()
	return u.String()
}
