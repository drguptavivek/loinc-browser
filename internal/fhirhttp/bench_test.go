package fhirhttp_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"loinc-browser/internal/fhirhttp"
	"loinc-browser/internal/loinc"
	"loinc-browser/pkg/terminology"
)

// openBenchServer starts an httptest server over the real local DB named by LOINC_TEST_DB,
// skipping when it is not set (docs/FHIR_TERMINOLOGY_PLAN.md §2/§10.1/§11: the HTTP-level
// budget, ≤5ms warm, sits on top of the in-process budget measured in
// pkg/terminology/bench_test.go).
func openBenchServer(b *testing.B) *httptest.Server {
	b.Helper()
	dbPath := os.Getenv("LOINC_TEST_DB")
	if dbPath == "" {
		b.Skip("LOINC_TEST_DB is not set; skipping HTTP benchmark")
	}
	store, err := loinc.OpenStore(dbPath, loinc.StoreOptions{CacheEntries: 4})
	if err != nil {
		b.Fatalf("open %s: %v", dbPath, err)
	}
	b.Cleanup(func() { store.Close() })

	mux := http.NewServeMux()
	fhirhttp.Register(mux, terminology.NewService(func() (*loinc.Store, error) { return store, nil }))
	server := httptest.NewServer(mux)
	b.Cleanup(server.Close)
	return server
}

// BenchmarkHTTPLookup measures $lookup end-to-end over HTTP (§2 Mode B/C budget: ≤5ms + RTT
// warm), on top of the in-process cost BenchmarkLookupTerm already measures.
func BenchmarkHTTPLookup(b *testing.B) {
	server := openBenchServer(b)
	client := server.Client()
	url := server.URL + "/fhir/CodeSystem/$lookup?code=718-7"
	warmUp, err := client.Get(url)
	if err != nil {
		b.Fatalf("warm-up GET: %v", err)
	}
	warmUp.Body.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(url)
		if err != nil {
			b.Fatalf("GET: %v", err)
		}
		resp.Body.Close()
	}
}

// BenchmarkHTTPExpandCount100 measures $expand of the full value set at the default page size
// end-to-end over HTTP (§11: count=100 ≤25ms).
func BenchmarkHTTPExpandCount100(b *testing.B) {
	server := openBenchServer(b)
	client := server.Client()
	url := server.URL + "/fhir/ValueSet/$expand?url=http://loinc.org/vs&count=100"
	warmUp, err := client.Get(url)
	if err != nil {
		b.Fatalf("warm-up GET: %v", err)
	}
	warmUp.Body.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(url)
		if err != nil {
			b.Fatalf("GET: %v", err)
		}
		resp.Body.Close()
	}
}
