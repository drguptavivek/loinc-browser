package terminology

import (
	"context"
	"os"
	"testing"

	"loinc-browser/internal/loinc"
)

// openBenchStore opens the real local DB named by LOINC_TEST_DB for in-process latency
// benchmarks, skipping when it is not set (§8 P4, bug #1: warm $lookup must be ~100µs
// in-process / ≤5ms over HTTP; these benchmarks are the in-process half of that budget).
func openBenchStore(b *testing.B) *loinc.Store {
	b.Helper()
	dbPath := os.Getenv("LOINC_TEST_DB")
	if dbPath == "" {
		b.Skip("LOINC_TEST_DB is not set; skipping in-process lookup benchmark")
	}
	store, err := loinc.OpenStore(dbPath, loinc.StoreOptions{CacheEntries: 4})
	if err != nil {
		b.Fatalf("open %s: %v", dbPath, err)
	}
	b.Cleanup(func() { store.Close() })
	return store
}

// BenchmarkLookupTerm measures warm $lookup for a LOINC term (718-7), which exercises the full
// term path: part links, hierarchy parents, groups, MAP_TO, the raw Loinc.csv row, and every
// LinguisticVariants language table.
func BenchmarkLookupTerm(b *testing.B) {
	store := openBenchStore(b)
	svc := NewService(func() (*loinc.Store, error) { return store, nil })
	ctx := context.Background()
	if _, outcomeErr := svc.Lookup(ctx, LookupParams{Code: "718-7"}); outcomeErr != nil {
		b.Fatalf("warm-up lookup: %v", outcomeErr)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, outcomeErr := svc.Lookup(ctx, LookupParams{Code: "718-7"}); outcomeErr != nil {
			b.Fatalf("lookup: %v", outcomeErr)
		}
	}
}

// BenchmarkLookupPart measures warm $lookup for an LP part (LP14542-2), the hierarchy-heavy path
// (parent/child properties, no linguistic variants or raw Loinc.csv row).
func BenchmarkLookupPart(b *testing.B) {
	store := openBenchStore(b)
	svc := NewService(func() (*loinc.Store, error) { return store, nil })
	ctx := context.Background()
	if _, outcomeErr := svc.Lookup(ctx, LookupParams{Code: "LP14542-2"}); outcomeErr != nil {
		b.Fatalf("warm-up lookup: %v", outcomeErr)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, outcomeErr := svc.Lookup(ctx, LookupParams{Code: "LP14542-2"}); outcomeErr != nil {
			b.Fatalf("lookup: %v", outcomeErr)
		}
	}
}

// BenchmarkValidateCode measures warm CodeSystem $validate-code (§4.4), which internally reuses
// $lookup, so its budget should track BenchmarkLookupTerm plus a small constant overhead.
func BenchmarkValidateCode(b *testing.B) {
	store := openBenchStore(b)
	svc := NewService(func() (*loinc.Store, error) { return store, nil })
	ctx := context.Background()
	params := ValidateCodeParams{Code: "718-7"}
	if _, outcomeErr := svc.ValidateCode(ctx, params); outcomeErr != nil {
		b.Fatalf("warm-up validate-code: %v", outcomeErr)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, outcomeErr := svc.ValidateCode(ctx, params); outcomeErr != nil {
			b.Fatalf("validate-code: %v", outcomeErr)
		}
	}
}

// BenchmarkSubsumes measures warm CodeSystem $subsumes (§4.5) walking the Component Hierarchy by
// System from an LP part down to a term. Uses §11's own LP384441-4/30064-0 pair (the upstream
// 2.83 exemplar pair): LP384441-4 is a hierarchy-only node in the local 2.82 release (no Part.csv
// row; see lookupHierarchyOnlyPart), which assertCodeExists/codeResolves now resolves via the
// same hierarchy-only fallback $lookup uses (previously a 400 "invalid" -- see
// TestSubsumesHierarchyOnlyPart).
func BenchmarkSubsumes(b *testing.B) {
	store := openBenchStore(b)
	svc := NewService(func() (*loinc.Store, error) { return store, nil })
	ctx := context.Background()
	params := SubsumesParams{CodeA: "LP384441-4", CodeB: "30064-0"}
	if _, outcomeErr := svc.Subsumes(ctx, params); outcomeErr != nil {
		b.Fatalf("warm-up subsumes: %v", outcomeErr)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, outcomeErr := svc.Subsumes(ctx, params); outcomeErr != nil {
			b.Fatalf("subsumes: %v", outcomeErr)
		}
	}
}

// BenchmarkTranslateWithURL measures warm ConceptMap $translate (§4.10) against one named map
// (loinc-to-ieee-11073-10101, per §11), the cheaper path since it skips catalogue search.
func BenchmarkTranslateWithURL(b *testing.B) {
	store := openBenchStore(b)
	svc := NewService(func() (*loinc.Store, error) { return store, nil })
	ctx := context.Background()
	params := TranslateParams{URL: loincSystem + "/cm/loinc-to-ieee-11073-10101", Code: "11556-8"}
	if _, outcomeErr := svc.Translate(ctx, params); outcomeErr != nil {
		b.Fatalf("warm-up translate: %v", outcomeErr)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, outcomeErr := svc.Translate(ctx, params); outcomeErr != nil {
			b.Fatalf("translate: %v", outcomeErr)
		}
	}
}

// BenchmarkTranslateWithoutURL measures warm $translate without a url/id: it searches every
// served map whose source system is http://loinc.org (§4.10), the more expensive path.
func BenchmarkTranslateWithoutURL(b *testing.B) {
	store := openBenchStore(b)
	svc := NewService(func() (*loinc.Store, error) { return store, nil })
	ctx := context.Background()
	params := TranslateParams{System: loincSystem, Code: "11556-8"}
	if _, outcomeErr := svc.Translate(ctx, params); outcomeErr != nil {
		b.Fatalf("warm-up translate: %v", outcomeErr)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, outcomeErr := svc.Translate(ctx, params); outcomeErr != nil {
			b.Fatalf("translate: %v", outcomeErr)
		}
	}
}

// BenchmarkExpandAnswerList measures warm ValueSet $expand of a small, fully intensional answer
// list (LL1162-8).
func BenchmarkExpandAnswerList(b *testing.B) {
	benchmarkExpand(b, ExpandParams{URL: loincSystem + "/vs/LL1162-8"})
}

// BenchmarkExpandAllCount100 measures $expand of the full "all LOINC codes" value set at the
// default page size (§11: count=100 p95 budget).
func BenchmarkExpandAllCount100(b *testing.B) {
	count := 100
	benchmarkExpand(b, ExpandParams{URL: loincSystem + "/vs", Count: &count})
}

// BenchmarkExpandAllCount1000 measures $expand of the full value set at the maximum page size
// (§10.1's DuckDB-mirror trigger: p95 > 50ms here would revisit that decision).
func BenchmarkExpandAllCount1000(b *testing.B) {
	count := 1000
	benchmarkExpand(b, ExpandParams{URL: loincSystem + "/vs", Count: &count})
}

// BenchmarkExpandAllCount1000Offset50000 measures the same page size deep into the result set,
// where a naive OFFSET scan would show up as a cost that grows with offset.
func BenchmarkExpandAllCount1000Offset50000(b *testing.B) {
	count, offset := 1000, 50000
	benchmarkExpand(b, ExpandParams{URL: loincSystem + "/vs", Count: &count, Offset: &offset})
}

// BenchmarkExpandFilterGlucose measures $expand with a text filter, which drives a different
// query plan (LIKE/property match) than the plain paged scan.
func BenchmarkExpandFilterGlucose(b *testing.B) {
	benchmarkExpand(b, ExpandParams{URL: loincSystem + "/vs", Filter: "glucose"})
}

func benchmarkExpand(b *testing.B, params ExpandParams) {
	store := openBenchStore(b)
	svc := NewService(func() (*loinc.Store, error) { return store, nil })
	ctx := context.Background()
	if _, outcomeErr := svc.Expand(ctx, params); outcomeErr != nil {
		b.Fatalf("warm-up expand: %v", outcomeErr)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, outcomeErr := svc.Expand(ctx, params); outcomeErr != nil {
			b.Fatalf("expand: %v", outcomeErr)
		}
	}
}

// BenchmarkQuestionnaire measures warm Questionnaire/{id} (§4.11) for a real LOINC panel
// (89689-4, per §11's acceptance checklist).
func BenchmarkQuestionnaire(b *testing.B) {
	store := openBenchStore(b)
	svc := NewService(func() (*loinc.Store, error) { return store, nil })
	ctx := context.Background()
	if _, outcomeErr := svc.Questionnaire(ctx, "89689-4"); outcomeErr != nil {
		b.Fatalf("warm-up questionnaire: %v", outcomeErr)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, outcomeErr := svc.Questionnaire(ctx, "89689-4"); outcomeErr != nil {
			b.Fatalf("questionnaire: %v", outcomeErr)
		}
	}
}
