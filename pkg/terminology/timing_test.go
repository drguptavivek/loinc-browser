package terminology

import (
	"context"
	"os"
	"sort"
	"testing"
	"time"

	"loinc-browser/internal/loinc"
)

// timingSamples runs N=500 warm calls of fn and returns their durations, sorted ascending, for
// percentile reporting (§9 item 4, §10.1, §11: p95/p99 against the real DB).
func timingSamples(t *testing.T, n int, fn func()) []time.Duration {
	t.Helper()
	fn() // warm-up, excluded
	samples := make([]time.Duration, n)
	for i := 0; i < n; i++ {
		start := time.Now()
		fn()
		samples[i] = time.Since(start)
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	return samples
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(p * float64(len(sorted)))
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func reportPercentiles(t *testing.T, name string, samples []time.Duration) {
	t.Helper()
	p50 := percentile(samples, 0.50)
	p95 := percentile(samples, 0.95)
	p99 := percentile(samples, 0.99)
	t.Logf("%-28s n=%-4d p50=%-10s p95=%-10s p99=%-10s max=%s",
		name, len(samples), p50, p95, p99, samples[len(samples)-1])
}

// TestTimingPercentiles records p50/p95/p99 in-process latency for the operations in §11's
// acceptance checklist and §10.1's DuckDB-mirror gate, against the real local DB. It only
// asserts the store opens and each call succeeds; the printed table is the actual acceptance
// evidence, transcribed into docs/FHIR_TERMINOLOGY_PLAN.md by hand after a run.
func TestTimingPercentiles(t *testing.T) {
	dbPath := os.Getenv("LOINC_TEST_DB")
	if dbPath == "" {
		t.Skip("LOINC_TEST_DB is not set; skipping timing percentile test")
	}
	store, err := loinc.OpenStore(dbPath, loinc.StoreOptions{CacheEntries: 4})
	if err != nil {
		t.Fatalf("open %s: %v", dbPath, err)
	}
	defer store.Close()
	svc := NewService(func() (*loinc.Store, error) { return store, nil })
	ctx := context.Background()
	const n = 500

	t.Run("lookup term 718-7", func(t *testing.T) {
		samples := timingSamples(t, n, func() {
			if _, err := svc.Lookup(ctx, LookupParams{Code: "718-7"}); err != nil {
				t.Fatalf("lookup: %v", err)
			}
		})
		reportPercentiles(t, "lookup(718-7)", samples)
		if p95 := percentile(samples, 0.95); p95 > 100*time.Microsecond {
			t.Logf("WARNING: lookup p95 %s exceeds the §2 Mode A budget (100µs warm)", p95)
		}
	})

	t.Run("expand http://loinc.org/vs count=1000", func(t *testing.T) {
		count := 1000
		params := ExpandParams{URL: loincSystem + "/vs", Count: &count}
		samples := timingSamples(t, n, func() {
			if _, err := svc.Expand(ctx, params); err != nil {
				t.Fatalf("expand: %v", err)
			}
		})
		reportPercentiles(t, "expand(vs, count=1000)", samples)
		if p95 := percentile(samples, 0.95); p95 > 50*time.Millisecond {
			t.Logf("WARNING: expand count=1000 p95 %s trips the §10.1 DuckDB-mirror gate (>50ms)", p95)
		}
	})

	t.Run("expand http://loinc.org/vs count=100", func(t *testing.T) {
		count := 100
		params := ExpandParams{URL: loincSystem + "/vs", Count: &count}
		samples := timingSamples(t, n, func() {
			if _, err := svc.Expand(ctx, params); err != nil {
				t.Fatalf("expand: %v", err)
			}
		})
		reportPercentiles(t, "expand(vs, count=100)", samples)
		if p95 := percentile(samples, 0.95); p95 > 25*time.Millisecond {
			t.Logf("WARNING: expand count=100 p95 %s exceeds the §11 budget (25ms)", p95)
		}
	})

	t.Run("validate-code 718-7", func(t *testing.T) {
		samples := timingSamples(t, n, func() {
			if _, err := svc.ValidateCode(ctx, ValidateCodeParams{Code: "718-7"}); err != nil {
				t.Fatalf("validate-code: %v", err)
			}
		})
		reportPercentiles(t, "validate-code(718-7)", samples)
	})

	t.Run("subsumes LP384441-4/30064-0", func(t *testing.T) {
		// §11's own upstream 2.83 exemplar pair; LP384441-4 is hierarchy-only in the local 2.82
		// release, resolved via the same fallback $lookup uses (see BenchmarkSubsumes).
		samples := timingSamples(t, n, func() {
			if _, err := svc.Subsumes(ctx, SubsumesParams{CodeA: "LP384441-4", CodeB: "30064-0"}); err != nil {
				t.Fatalf("subsumes: %v", err)
			}
		})
		reportPercentiles(t, "subsumes(LP384441-4,30064-0)", samples)
	})
}
