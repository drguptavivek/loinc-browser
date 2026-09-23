package terminology

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"loinc-browser/internal/loinc"
)

// TestOpenReadOnlyServesLookupWithoutIndexes ingests a fixture release straight to a DB path and
// opens it with Open (read-only) without ever calling loinc.OpenStore first, so the FHIR/ValueSet
// lazy indexes (only ever created by a writable OpenStore) are genuinely absent. Lookup must still
// succeed: correctness, not speed, is what a read-only Mode A caller gets from an unindexed DB.
func TestOpenReadOnlyServesLookupWithoutIndexes(t *testing.T) {
	ctx := context.Background()
	releaseDir := writeTerminologyTestRelease(t)
	dbPath := filepath.Join(t.TempDir(), "loinc.sqlite")
	if _, err := loinc.Ingest(ctx, loinc.IngestOptions{ReleaseDir: releaseDir, DBPath: dbPath}); err != nil {
		t.Fatalf("ingest: %v", err)
	}

	svc, err := Open(dbPath, OpenOptions{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer svc.Close()

	result, outcome := svc.Lookup(ctx, LookupParams{Code: "10000-1"})
	if outcome != nil {
		t.Fatalf("Lookup: %+v", outcome)
	}
	if display, ok := findParam(result.Parameter, "display"); !ok || display.ValueString == nil {
		t.Fatalf("expected a display parameter, got %+v", result.Parameter)
	}
}

// ExampleOpen shows the Mode A entry point an external module (one that cannot import
// loinc-browser/internal/loinc) uses to read a local LOINC database read-only. No "// Output:"
// comment: compile-checked documentation only, since it depends on a local release DB
// (./data/loinc-normalized.sqlite) that AGENTS.md keeps out of source control and CI.
func ExampleOpen() {
	svc, err := Open("./data/loinc-normalized.sqlite", OpenOptions{})
	if err != nil {
		fmt.Println("open failed:", err)
		return
	}
	defer svc.Close()

	result, outcome := svc.Lookup(context.Background(), LookupParams{Code: "718-7"})
	if outcome != nil {
		fmt.Println("lookup failed:", outcome.Text)
		return
	}
	display, _ := findParam(result.Parameter, "display")
	fmt.Println(display.ValueString)
}
