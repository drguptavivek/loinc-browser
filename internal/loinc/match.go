package loinc

import (
	"context"
	"strings"
)

// matchCandidateLimit caps candidates per name to a short pick list, not a full results page.
const matchCandidateLimit = 5

// matchConfidentMargin is how much better the top result's Rank (bm25-like: lower is better) must
// be than the second result's, absent a LOINC-number or CLCI-name hit, to call a name confident
// rather than needing review.
// ponytail: hand-tuned against the CLCI General Names in TestMatchNamesAgainstCLCI
// (match_test.go, gated on LOINC_TEST_DB); retune there if the wrong-confident share drifts.
const matchConfidentMargin = 4.0

// NameMatch is one name's match result when mapping a batch of lab test master names to LOINC.
// Bucket is "confident" (safe to auto-map), "review" (needs a human), or "none" (nothing found).
type NameMatch struct {
	Name       string         `json:"name"`
	Bucket     string         `json:"bucket"`
	Candidates []SearchResult `json:"candidates"`
	// Relaxed/Synonyms/CLCIMatches mirror the underlying SearchResponse, for a UI that wants to
	// explain why a name landed in its bucket.
	Relaxed     bool     `json:"relaxed,omitempty"`
	Synonyms    []string `json:"synonyms,omitempty"`
	CLCIMatches []string `json:"clciMatches,omitempty"`
}

// MatchNames looks up each name as a term search, for mapping a whole lab test master in one
// request. params carries the shared list filters (class, status, clci, ...); Query, paging, and
// sort are overridden per name. Order of the result matches names. An empty or whitespace-only name is
// bucketed "none" without a lookup.
func (s *Store) MatchNames(ctx context.Context, names []string, params SearchParams) ([]NameMatch, error) {
	matches := make([]NameMatch, len(names))
	for i, name := range names {
		matches[i].Name = name
		matches[i].Candidates = []SearchResult{}
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			matches[i].Bucket = "none"
			continue
		}
		p := params
		p.Query = trimmed
		p.Limit, p.Offset, p.Sort = matchCandidateLimit, 0, "relevance"
		resp, err := s.Search(ctx, p)
		if err != nil {
			return nil, err
		}
		matches[i].Bucket = matchBucket(resp, trimmed)
		if resp.Results != nil {
			matches[i].Candidates = resp.Results
		}
		matches[i].Relaxed = resp.Relaxed
		matches[i].Synonyms = resp.Synonyms
		matches[i].CLCIMatches = resp.CLCIMatches
	}
	return matches, nil
}

// matchBucket buckets one name's search response: "none" with no results; "confident" when the
// name is itself the top LOINC number, the top result is pinned by a CLCI General Name match, or
// (absent a relaxed/dropped-word retry) there is exactly one result or the top result clearly
// outranks the runner-up; else "review".
func matchBucket(resp SearchResponse, name string) string {
	if len(resp.Results) == 0 {
		return "none"
	}
	top := resp.Results[0]
	if loincNumberRegexp.MatchString(name) && strings.EqualFold(name, top.LOINCNum) {
		return "confident"
	}
	if containsValue(resp.CLCIMatches, top.LOINCNum) {
		return "confident"
	}
	if resp.Relaxed {
		return "review"
	}
	if len(resp.Results) == 1 {
		return "confident"
	}
	if resp.Results[1].Rank-top.Rank >= matchConfidentMargin {
		return "confident"
	}
	return "review"
}
