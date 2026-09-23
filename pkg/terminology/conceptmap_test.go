package terminology

import (
	"context"
	"path/filepath"
	"testing"

	"loinc-browser/internal/loinc"
)

func newConceptMapTestService(t *testing.T) *Service {
	t.Helper()
	ctx := context.Background()
	releaseDir := writeConceptMapQuestionnaireFixture(t)
	dbPath := filepath.Join(t.TempDir(), "loinc.sqlite")
	if _, err := loinc.Ingest(ctx, loinc.IngestOptions{ReleaseDir: releaseDir, DBPath: dbPath}); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	store, err := loinc.OpenStore(dbPath, loinc.StoreOptions{CacheEntries: 4})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return NewService(func() (*loinc.Store, error) { return store, nil })
}

func matchConcept(t *testing.T, match Parameter) Coding {
	t.Helper()
	concept, ok := partValue(match, "concept")
	if !ok || concept.ValueCoding == nil {
		t.Fatalf("match has no concept part: %+v", match)
	}
	return *concept.ValueCoding
}

func matchEquivalence(t *testing.T, match Parameter) string {
	t.Helper()
	eq, ok := partValue(match, "equivalence")
	if !ok || eq.ValueCode == nil {
		t.Fatalf("match has no equivalence part: %+v", match)
	}
	return *eq.ValueCode
}

func TestTranslateIEEEForwardAndReverse(t *testing.T) {
	svc := newConceptMapTestService(t)
	ctx := context.Background()

	result, err := svc.Translate(ctx, TranslateParams{URL: loincSystem + "/cm/loinc-to-ieee-11073-10101", Code: "10000-1"})
	if err != nil {
		t.Fatalf("forward translate: %v", err)
	}
	assertResultTrue(t, result)
	match, _ := findParam(result.Parameter, "match")
	if got := matchConcept(t, match); got.Code != "999001" || got.System != "urn:iso:std:iso:11073:10101" {
		t.Fatalf("forward concept = %+v", got)
	}
	if matchEquivalence(t, match) != "equivalent" {
		t.Fatalf("equivalence = %q", matchEquivalence(t, match))
	}

	reverse, err := svc.Translate(ctx, TranslateParams{URL: loincSystem + "/cm/ieee-11073-10101-to-loinc", Code: "999001"})
	if err != nil {
		t.Fatalf("reverse translate: %v", err)
	}
	assertResultTrue(t, reverse)
	rmatch, _ := findParam(reverse.Parameter, "match")
	if got := matchConcept(t, rmatch); got.Code != "10000-1" || got.System != loincSystem {
		t.Fatalf("reverse concept = %+v", got)
	}
}

func TestTranslateReverseFlagEquivalentToReverseMap(t *testing.T) {
	svc := newConceptMapTestService(t)
	ctx := context.Background()

	viaFlag, err := svc.Translate(ctx, TranslateParams{URL: loincSystem + "/cm/loinc-to-ieee-11073-10101", Code: "999001", Reverse: true})
	if err != nil {
		t.Fatalf("translate with reverse flag: %v", err)
	}
	assertResultTrue(t, viaFlag)
	match, _ := findParam(viaFlag.Parameter, "match")
	if got := matchConcept(t, match); got.Code != "10000-1" {
		t.Fatalf("reverse-flag concept = %+v", got)
	}
}

func TestTranslateNoURLSearchesBySourceSystem(t *testing.T) {
	svc := newConceptMapTestService(t)
	ctx := context.Background()

	// 10000-1 has both an IEEE row and a term-level playbook (RadLex) row; catalogue order puts
	// IEEE first.
	result, err := svc.Translate(ctx, TranslateParams{System: loincSystem, Code: "10000-1"})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	assertResultTrue(t, result)
	matches := 0
	for _, p := range result.Parameter {
		if p.Name == "match" {
			matches++
		}
	}
	if matches != 2 {
		t.Fatalf("matches = %d, want 2 (IEEE + RadLex)", matches)
	}
	first, _ := findParamAt(result.Parameter, "match", 0)
	if got := matchConcept(t, first).System; got != "urn:iso:std:iso:11073:10101" {
		t.Fatalf("first match system = %q, want IEEE (catalogue order)", got)
	}
	second, _ := findParamAt(result.Parameter, "match", 1)
	if got := matchConcept(t, second); got.System != "http://radlex.org" || got.Code != "RPID9001" {
		t.Fatalf("second match = %+v", got)
	}
	if matchEquivalence(t, second) != "relatedto" {
		t.Fatalf("playbook equivalence = %q, want relatedto (no CSV column)", matchEquivalence(t, second))
	}
}

func TestTranslatePartRelatedSnomed(t *testing.T) {
	svc := newConceptMapTestService(t)
	ctx := context.Background()
	result, err := svc.Translate(ctx, TranslateParams{URL: loincSystem + "/cm/loinc-parts-to-snomed-ct", Code: "LP1000-1"})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	assertResultTrue(t, result)
	match, _ := findParam(result.Parameter, "match")
	if got := matchConcept(t, match); got.Code != "123456" || got.System != "http://snomed.info/sct" {
		t.Fatalf("concept = %+v", got)
	}
}

func TestTranslateLoincMapToComment(t *testing.T) {
	svc := newConceptMapTestService(t)
	ctx := context.Background()
	result, err := svc.Translate(ctx, TranslateParams{URL: loincSystem + "/cm/loinc-map-to", Code: "20000-8"})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	assertResultTrue(t, result)
	match, _ := findParam(result.Parameter, "match")
	if matchEquivalence(t, match) != "equivalent" {
		t.Fatalf("equivalence = %q", matchEquivalence(t, match))
	}
	comment, ok := partValue(match, "comment")
	if !ok || comment.ValueString == nil || *comment.ValueString != "See replacement term 10000-1" {
		t.Fatalf("comment part = %+v, ok=%v", comment, ok)
	}
	if got := matchConcept(t, match); got.Code != "10000-1" {
		t.Fatalf("concept = %+v", got)
	}
}

func TestTranslateNoMatch(t *testing.T) {
	svc := newConceptMapTestService(t)
	ctx := context.Background()
	result, err := svc.Translate(ctx, TranslateParams{System: loincSystem, Code: "60000-1"})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	res, _ := findParam(result.Parameter, "result")
	if res.ValueBoolean == nil || *res.ValueBoolean {
		t.Fatalf("result = %+v, want false", res)
	}
	msg, ok := findParam(result.Parameter, "message")
	if !ok || msg.ValueString == nil || *msg.ValueString != "No mapping found matching specified criteria" {
		t.Fatalf("message = %+v", msg)
	}
}

func TestTranslateUnknownSystem404(t *testing.T) {
	svc := newConceptMapTestService(t)
	ctx := context.Background()
	_, err := svc.Translate(ctx, TranslateParams{System: "http://example.org/nope", Code: "1"})
	if err == nil || err.Status != 404 || err.Code != "not-found" {
		t.Fatalf("err = %+v, want 404 not-found", err)
	}
}

func TestReadConceptMapEmbedsGroup(t *testing.T) {
	svc := newConceptMapTestService(t)
	ctx := context.Background()
	cm, err := svc.ReadConceptMap(ctx, "loinc-to-ieee-11073-10101")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if cm.ResourceType != "ConceptMap" || cm.SourceUri != loincSystem || cm.TargetUri != "urn:iso:std:iso:11073:10101" {
		t.Fatalf("resource = %+v", cm)
	}
	if len(cm.Group) != 1 || len(cm.Group[0].Element) != 1 || cm.Group[0].Element[0].Code != "10000-1" {
		t.Fatalf("group = %+v", cm.Group)
	}
	if len(cm.Extension) != 0 {
		t.Fatalf("extension = %+v, want none under the cap", cm.Extension)
	}
}

func TestReadConceptMapUnknownID(t *testing.T) {
	svc := newConceptMapTestService(t)
	_, err := svc.ReadConceptMap(context.Background(), "not-a-real-map")
	if err == nil || err.Status != 404 {
		t.Fatalf("err = %+v, want 404", err)
	}
}

func TestSearchConceptMapsFiltersAndPages(t *testing.T) {
	svc := newConceptMapTestService(t)
	ctx := context.Background()

	all, err := svc.SearchConceptMaps(ctx, ConceptMapSearchParams{})
	if err != nil {
		t.Fatalf("search all: %v", err)
	}
	if all.Total != len(allConceptMapDescriptors()) {
		t.Fatalf("total = %d, want %d", all.Total, len(allConceptMapDescriptors()))
	}

	filtered, err := svc.SearchConceptMaps(ctx, ConceptMapSearchParams{TargetSystem: "urn:iso:std:iso:11073:10101"})
	if err != nil {
		t.Fatalf("search by target-system: %v", err)
	}
	if filtered.Total != 1 || len(filtered.Entry) != 1 {
		t.Fatalf("filtered = %+v", filtered)
	}

	bySourceCode, err := svc.SearchConceptMaps(ctx, ConceptMapSearchParams{SourceSystem: loincSystem, SourceCode: "10000-1"})
	if err != nil {
		t.Fatalf("search by source-code: %v", err)
	}
	if bySourceCode.Total != 2 { // IEEE + term-level RadLex
		t.Fatalf("bySourceCode.Total = %d, want 2", bySourceCode.Total)
	}

	paged, err := svc.SearchConceptMaps(ctx, ConceptMapSearchParams{Count: 1, Offset: 1})
	if err != nil {
		t.Fatalf("search paged: %v", err)
	}
	if len(paged.Entry) != 1 || paged.Total != all.Total {
		t.Fatalf("paged = %+v", paged)
	}
}

func assertResultTrue(t *testing.T, result *Parameters) {
	t.Helper()
	res, ok := findParam(result.Parameter, "result")
	if !ok || res.ValueBoolean == nil || !*res.ValueBoolean {
		t.Fatalf("result = %+v, want true", res)
	}
}
