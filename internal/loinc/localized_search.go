package loinc

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
)

// variantFTSTable is the derived per-language full-text index built lazily over the linguistic
// variant raw tables (AccessoryFiles/LinguisticVariants), so a lang=de-DE word search also matches
// that language's names ("Natrium" -> 2951-2), not English words only. It is not part of ingest's
// schema: Ingest always removes and recreates the whole database file, so this table (and any rows
// in it) never survive a re-ingest -- the first Store opened against a fresh database just rebuilds
// it, no separate release/version check needed.
const variantFTSTable = "loinc_variant_fts"

// variantFTSCandidateLimit caps how many linguistic-variant matches searchVariantLOINCNums fetches
// before the normal filter machinery narrows them down, same order of magnitude as a generous
// results page.
const variantFTSCandidateLimit = 200

// variantFTSBuild gates loinc_variant_fts's build: kicked off once in the background at store open
// (startVariantFTSBuild), since indexing every linguistic variant file can take a while on a large
// release. Until ready, lang search falls back to English-only word matching; LocalizedNames
// (display only) is unaffected.
type variantFTSBuild struct {
	once  sync.Once
	ready atomic.Bool
}

// startVariantFTSBuild kicks off loinc_variant_fts's build in the background, once per Store. It
// is a no-op read-only (CREATE VIRTUAL TABLE needs a write connection, same restriction as
// ensureRawIndex) or on an empty database (nothing to index yet, and RawTable lookups would fail).
func (s *Store) startVariantFTSBuild(hasTerms bool) {
	if s.readOnly || !hasTerms {
		return
	}
	go func() {
		if err := s.buildVariantFTS(context.Background()); err != nil {
			log.Printf("loinc: build linguistic variant search index: %v", err)
		}
	}()
}

func (s *Store) buildVariantFTS(ctx context.Context) error {
	var buildErr error
	s.variantFTS.once.Do(func() {
		buildErr = s.doBuildVariantFTS(ctx)
	})
	return buildErr
}

// doBuildVariantFTS creates and populates variantFTSTable, only setting variantFTS.ready when the
// table actually exists and can be queried -- a release with no linguistic variants loaded leaves
// ready false forever, so searchVariantLOINCNums never queries a table that was never created.
func (s *Store) doBuildVariantFTS(ctx context.Context) error {
	var existing int
	if err := s.db.QueryRowContext(ctx,
		`select count(*) from sqlite_master where type = 'table' and name = ?`, variantFTSTable,
	).Scan(&existing); err != nil {
		return fmt.Errorf("check %s: %w", variantFTSTable, err)
	}
	if existing > 0 {
		// Already built by an earlier Store against this same database file: nothing to do.
		s.variantFTS.ready.Store(true)
		return nil
	}

	union, err := s.buildLinguisticVariantUnionOnce(ctx)
	if err != nil {
		return err
	}
	if len(union.tables) == 0 {
		return nil // no linguistic variants loaded in this release
	}

	if _, err := s.db.ExecContext(ctx, `create virtual table `+variantFTSTable+` using fts5(
		loinc_num unindexed, lang unindexed, name, tokenize = 'unicode61 remove_diacritics 2')`); err != nil {
		return fmt.Errorf("create %s: %w", variantFTSTable, err)
	}
	// Columns come from RawTable resolution against the allowlisted linguistic-variant relative
	// paths (buildLinguisticVariantUnion), never from user input; union.languages is the matching
	// "iso-COUNTRY" code for each table, same order.
	for i, table := range union.tables {
		stmt := `insert into ` + variantFTSTable + `(loinc_num, lang, name)
			select "LOINC_NUM", ?,
				coalesce("LONG_COMMON_NAME",'') || ' ' || coalesce("COMPONENT",'') || ' ' ||
				coalesce("SHORTNAME",'') || ' ' || coalesce("RELATEDNAMES2",'')
			from ` + quoteIdentifier(table)
		if _, err := s.db.ExecContext(ctx, stmt, union.languages[i]); err != nil {
			return fmt.Errorf("index linguistic variant names for %s: %w", union.languages[i], err)
		}
	}
	s.variantFTS.ready.Store(true)
	return nil
}

// searchVariantLOINCNums runs word search against lang's linguistic variant names, returning up to
// limit matching LOINC numbers ordered by common_test_rank (ranked terms first, 0 last) then bm25
// -- the same "usage beats raw relevance" preference the English pipeline applies. query is plain
// words (searchTerms/mergeLocalizedTerms never call this for an exact LOINC number); each word
// becomes a required prefix match via makeFTSQuery, same as the English FTS query. Returns nil,nil
// when the index isn't built yet or lang has no linguistic variant loaded.
func (s *Store) searchVariantLOINCNums(ctx context.Context, lang, query string, limit int) ([]string, error) {
	if !s.variantFTS.ready.Load() {
		return nil, nil
	}
	ftsQuery := makeFTSQuery(query)
	if ftsQuery == "" {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `select v.loinc_num
		from `+variantFTSTable+` v
		left join loinc_terms t on t.loinc_num = v.loinc_num
		where v.lang = ? and v.name match ?
		order by case when coalesce(t.common_test_rank, 0) > 0 then 0 else 1 end,
			coalesce(t.common_test_rank, 0), bm25(`+variantFTSTable+`)
		limit ?`, lang, ftsQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("search linguistic variant names for %s: %w", lang, err)
	}
	defer rows.Close()
	var codes []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, fmt.Errorf("scan linguistic variant match: %w", err)
		}
		codes = append(codes, code)
	}
	return codes, rows.Err()
}

// mergeLocalizedTerms adds lang's linguistic-variant word matches to response, the English-only
// result of englishSearchTerms: if English matched something, variant-only hits are appended after
// them, up to the page limit; if English matched nothing (query was typed entirely in lang), the
// variant hits become the whole result. Every normal filter (class, status, clci, ...) still
// applies to variant hits, via the same s.search/filterClauses machinery, using LOINCNums to
// restrict the candidate set to what the variant index matched.
func (s *Store) mergeLocalizedTerms(ctx context.Context, params SearchParams, response SearchResponse) (SearchResponse, error) {
	params = NormalizeTermListParams(params)
	lang := strings.TrimSpace(params.Lang)
	query := strings.TrimSpace(params.Query)
	if lang == "" || query == "" || loincNumberRegexp.MatchString(query) {
		return response, nil
	}
	variantCodes, err := s.searchVariantLOINCNums(ctx, lang, query, variantFTSCandidateLimit)
	if err != nil || len(variantCodes) == 0 {
		return response, err
	}

	seen := make(map[string]bool, len(response.Results))
	for _, result := range response.Results {
		seen[result.LOINCNum] = true
	}
	newCodes := make([]string, 0, len(variantCodes))
	for _, code := range variantCodes {
		if !seen[code] {
			newCodes = append(newCodes, code)
		}
	}
	if len(newCodes) == 0 {
		return response, nil
	}

	probe := params
	probe.Query, probe.Offset = "", 0
	probe.LOINCNums = newCodes
	probe.Limit = len(newCodes)
	variantHits, err := s.search(ctx, probe, "")
	if err != nil {
		return response, err
	}
	byCode := make(map[string]SearchResult, len(variantHits.Results))
	for _, result := range variantHits.Results {
		byCode[result.LOINCNum] = result
	}
	// Filtering (status, class, ...) may have dropped some codes; walk newCodes to keep
	// searchVariantLOINCNums' rank order among what is left.
	ordered := make([]SearchResult, 0, len(newCodes))
	for _, code := range newCodes {
		if result, ok := byCode[code]; ok {
			ordered = append(ordered, result)
		}
	}
	if len(ordered) == 0 {
		return response, nil
	}

	if response.Total > 0 {
		room := params.Limit - len(response.Results)
		if room <= 0 {
			return response, nil
		}
		if room > len(ordered) {
			room = len(ordered)
		}
		response.Results = append(response.Results, ordered[:room]...)
		// ponytail: Total counts only what this page actually appended, not every remaining
		// variant match -- an exact merged total would need a second count query per page.
		response.Total += room
	} else {
		// English matched nothing, even after the relaxed drop-a-word retry: the query is a
		// localized name search. ordered is already ranked and ready to paginate like an
		// English-only result.
		end := params.Offset + params.Limit
		if end > len(ordered) {
			end = len(ordered)
		}
		if params.Offset < len(ordered) {
			response.Results = ordered[params.Offset:end]
		} else {
			response.Results = nil
		}
		// ponytail: Total is the FTS candidate count (capped at variantFTSCandidateLimit), not an
		// exact match count -- exact would need a separate count query against the variant index.
		response.Total = len(newCodes)
		response.Relaxed = false
		response.Notice = ""
	}
	response.HasMore = response.Offset+len(response.Results) < response.Total
	response.Links = termListPageLinks("/api/v1/terms/search", params, response.Total)
	return response, nil
}
