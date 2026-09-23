package terminology

import (
	"context"
	"strings"
	"testing"
)

func expandCount(n int) *int { return &n }

func TestReadValueSetCatalogue(t *testing.T) {
	svc := newValueSetTestService(t)
	ctx := context.Background()

	cases := []struct {
		id        string
		wantForm  composeFormWant
		wantCount int // -1 to skip
	}{
		{"loinc-all", wantFilter, -1},
		{"loinc-top-ranked", wantFilter, -1},
		{"deprecated-loinc-terms", wantConcept, 2},
		{"valid-hl7-attachment-requests", wantConcept, 1},
		{"valid-hl7-attachment-responses", wantConcept, 2},
		{"valid-hl7-attachment-responses-ig-exists", wantConcept, 1},
		{"valid-hl7-attachment-responses-no-ig-exists", wantConcept, 1},
		{"top-lab-orders", wantConcept, 2},
		{"loinc-document-ontology", wantConcept, 2},
		{"loinc-rsna-radiology-playbook", wantConcept, 2},
		{"loinc-universal-order-set", wantConcept, 2},
		{"loinc-imaging-document-codes", wantConcept, 1},
		{"LL1162-8-does-not-exist", wantNotFound, -1},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			vs, err := svc.ReadValueSet(ctx, tc.id)
			if tc.wantForm == wantNotFound {
				if err == nil || err.Code != "not-found" {
					t.Fatalf("expected not-found, got %v / %v", vs, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ReadValueSet(%s): %v", tc.id, err)
			}
			if vs.ID != tc.id {
				t.Errorf("id = %q, want %q", vs.ID, tc.id)
			}
			if vs.URL != loincSystem+"/vs/"+tc.id {
				t.Errorf("url = %q", vs.URL)
			}
			if vs.Compose == nil || len(vs.Compose.Include) != 1 {
				t.Fatalf("compose = %+v", vs.Compose)
			}
			include := vs.Compose.Include[0]
			switch tc.wantForm {
			case wantFilter:
				// loinc-all has a bare system-only include (no filter[]); the others carry one.
				if include.Concept != nil {
					t.Errorf("expected filter/bare form (no concept[]), got %+v", include)
				}
			case wantConcept:
				if include.Concept == nil {
					t.Errorf("expected concept form, got %+v", include)
				}
				if tc.wantCount >= 0 && len(include.Concept) != tc.wantCount {
					t.Errorf("concept count = %d, want %d: %+v", len(include.Concept), tc.wantCount, include.Concept)
				}
			}
		})
	}
}

type composeFormWant int

const (
	wantFilter composeFormWant = iota
	wantConcept
	wantNotFound
)

func TestReadValueSetAnswerListAndGroup(t *testing.T) {
	svc := newValueSetTestService(t)
	ctx := context.Background()

	vs, err := svc.ReadValueSet(ctx, "LL1000-1")
	if err != nil {
		t.Fatalf("read LL1000-1: %v", err)
	}
	if vs.Name != "Positive negative" {
		t.Errorf("name = %q", vs.Name)
	}
	if len(vs.Identifier) != 1 || vs.Identifier[0].Value != "urn:oid:1.2.3.4" {
		t.Errorf("identifier = %+v", vs.Identifier)
	}
	concepts := vs.Compose.Include[0].Concept
	if len(concepts) != 2 || concepts[0].Code != "LA1-1" || concepts[1].Code != "LA2-2" {
		t.Errorf("concepts (want sequence order) = %+v", concepts)
	}

	vsGroup, err := svc.ReadValueSet(ctx, "LG1000-1")
	if err != nil {
		t.Fatalf("read LG1000-1: %v", err)
	}
	if vsGroup.Name != "Chemistry tests" {
		t.Errorf("group name = %q", vsGroup.Name)
	}
	if len(vsGroup.Compose.Include[0].Concept) != 1 || vsGroup.Compose.Include[0].Concept[0].Code != "10000-1" {
		t.Errorf("group concepts = %+v", vsGroup.Compose.Include[0].Concept)
	}

	// id-version form.
	version, verr := svc.getReleaseVersionForTest(ctx)
	if verr != nil {
		t.Fatalf("release version: %v", verr)
	}
	if _, err := svc.ReadValueSet(ctx, "LL1000-1-"+version); err != nil {
		t.Fatalf("read LL1000-1-%s: %v", version, err)
	}
}

// getReleaseVersionForTest exposes releaseVersion for the id-version read test above.
func (s *Service) getReleaseVersionForTest(ctx context.Context) (string, *OutcomeError) {
	store, err := s.store()
	if err != nil {
		return "", err
	}
	return s.releaseVersion(ctx, store)
}

func TestExpandPagingAndOrder(t *testing.T) {
	svc := newValueSetTestService(t)
	ctx := context.Background()

	// All-LOINC has 5 terms in the fixture; page through with count=2 and check disjoint, stable,
	// numeric-ascending order and a constant total.
	var seen []string
	offset := 0
	for {
		vs, err := svc.Expand(ctx, ExpandParams{URL: "http://loinc.org/vs", Offset: &offset, Count: expandCount(2)})
		if err != nil {
			t.Fatalf("expand offset=%d: %v", offset, err)
		}
		if vs.Expansion.Total != 5 {
			t.Fatalf("total = %d, want 5 (offset=%d)", vs.Expansion.Total, offset)
		}
		if len(vs.Expansion.Contains) == 0 {
			break
		}
		for _, c := range vs.Expansion.Contains {
			seen = append(seen, c.Code)
		}
		offset += 2
	}
	want := []string{"10000-1", "11000-0", "12000-8", "13000-6", "20000-8"}
	if strings.Join(seen, ",") != strings.Join(want, ",") {
		t.Errorf("paged codes = %v, want %v", seen, want)
	}
}

func TestExpandCountEdgeCases(t *testing.T) {
	svc := newValueSetTestService(t)
	ctx := context.Background()

	vs, err := svc.Expand(ctx, ExpandParams{URL: "http://loinc.org/vs", Count: expandCount(0)})
	if err != nil {
		t.Fatalf("count=0: %v", err)
	}
	if vs.Expansion.Total != 5 || len(vs.Expansion.Contains) != 0 {
		t.Errorf("count=0: total=%d contains=%d, want 5/0", vs.Expansion.Total, len(vs.Expansion.Contains))
	}

	if _, err := svc.Expand(ctx, ExpandParams{URL: "http://loinc.org/vs", Count: expandCount(-1)}); err == nil || err.Code != "invalid" {
		t.Errorf("negative count: err = %v, want invalid", err)
	}
	if _, err := svc.Expand(ctx, ExpandParams{URL: "http://loinc.org/vs", Offset: expandCount(-1)}); err == nil || err.Code != "invalid" {
		t.Errorf("negative offset: err = %v, want invalid", err)
	}

	// count > max clamps rather than erroring.
	vsClamped, err := svc.Expand(ctx, ExpandParams{URL: "http://loinc.org/vs", Count: expandCount(5000)})
	if err != nil {
		t.Fatalf("count=5000: %v", err)
	}
	if got := *vsClamped.Expansion.Parameter[1].ValueInteger; got != maxExpandCount {
		t.Errorf("clamped count parameter = %d, want %d", got, maxExpandCount)
	}
}

func TestExpandActiveOnlyAndInactiveFlag(t *testing.T) {
	svc := newValueSetTestService(t)
	ctx := context.Background()

	all, err := svc.Expand(ctx, ExpandParams{URL: "http://loinc.org/vs", Count: expandCount(100)})
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	var deprecatedSeen, inactiveFlagged int
	for _, c := range all.Expansion.Contains {
		if c.Code == "20000-8" || c.Code == "13000-6" {
			deprecatedSeen++
			if !c.Inactive {
				t.Errorf("code %s should carry inactive:true", c.Code)
			}
			inactiveFlagged++
		} else if c.Inactive {
			t.Errorf("code %s should not be inactive", c.Code)
		}
	}
	if deprecatedSeen != 2 {
		t.Fatalf("expected 2 deprecated terms in unrestricted expand, saw %d", deprecatedSeen)
	}

	active, err := svc.Expand(ctx, ExpandParams{URL: "http://loinc.org/vs", ActiveOnly: true, Count: expandCount(100)})
	if err != nil {
		t.Fatalf("expand activeOnly: %v", err)
	}
	if active.Expansion.Total != 3 {
		t.Errorf("activeOnly total = %d, want 3", active.Expansion.Total)
	}
	for _, c := range active.Expansion.Contains {
		if c.Code == "20000-8" || c.Code == "13000-6" {
			t.Errorf("activeOnly should exclude deprecated code %s", c.Code)
		}
	}
}

func TestExpandFilter(t *testing.T) {
	svc := newValueSetTestService(t)
	ctx := context.Background()

	vs, err := svc.Expand(ctx, ExpandParams{URL: "http://loinc.org/vs", Filter: "sodium", Count: expandCount(100)})
	if err != nil {
		t.Fatalf("expand filter: %v", err)
	}
	if vs.Expansion.Total != 1 || len(vs.Expansion.Contains) != 1 || vs.Expansion.Contains[0].Code != "11000-0" {
		t.Errorf("filter=sodium: total=%d contains=%+v", vs.Expansion.Total, vs.Expansion.Contains)
	}

	// case-insensitive, word-prefix: "CHOLES" prefixes "Cholesterol" in both the active and the
	// deprecated term's display, so both match.
	vs2, err := svc.Expand(ctx, ExpandParams{URL: "http://loinc.org/vs", Filter: "CHOLES", Count: expandCount(100)})
	if err != nil {
		t.Fatalf("expand filter prefix: %v", err)
	}
	if vs2.Expansion.Total != 2 {
		t.Errorf("filter=CHOLES: total=%d contains=%+v, want 2", vs2.Expansion.Total, vs2.Expansion.Contains)
	}

	// filter word unique to one term's display.
	vs3, err := svc.Expand(ctx, ExpandParams{URL: "http://loinc.org/vs", Filter: "mass", Count: expandCount(100)})
	if err != nil {
		t.Fatalf("expand filter mass: %v", err)
	}
	if vs3.Expansion.Total != 1 || vs3.Expansion.Contains[0].Code != "10000-1" {
		t.Errorf("filter=mass: total=%d contains=%+v", vs3.Expansion.Total, vs3.Expansion.Contains)
	}

	// small-source (answer list) filter.
	llVs, err := svc.Expand(ctx, ExpandParams{URL: "http://loinc.org/vs/LL2000-2", Filter: "mild", Count: expandCount(100)})
	if err != nil {
		t.Fatalf("expand LL filter: %v", err)
	}
	if llVs.Expansion.Total != 1 || llVs.Expansion.Contains[0].Code != "LA9-9" {
		t.Errorf("LL filter=mild: total=%d contains=%+v", llVs.Expansion.Total, llVs.Expansion.Contains)
	}
}

func TestExpandUnknownValueSet(t *testing.T) {
	svc := newValueSetTestService(t)
	ctx := context.Background()
	if _, err := svc.Expand(ctx, ExpandParams{URL: "http://loinc.org/vs/LL9999-9"}); err == nil || err.Code != "not-found" {
		t.Errorf("unknown LL: err = %v, want not-found", err)
	}
}

func TestExpandInlineComposeFilters(t *testing.T) {
	svc := newValueSetTestService(t)
	ctx := context.Background()

	expandInline := func(t *testing.T, inc ValueSetInclude) *ValueSet {
		t.Helper()
		vs, err := svc.Expand(ctx, ExpandParams{
			Inline: &InlineValueSet{Compose: &ValueSetCompose{Include: []ValueSetInclude{inc}}},
			Count:  expandCount(100),
		})
		if err != nil {
			t.Fatalf("expand inline: %v", err)
		}
		return vs
	}
	codes := func(vs *ValueSet) []string {
		var out []string
		for _, c := range vs.Expansion.Contains {
			out = append(out, c.Code)
		}
		return out
	}

	t.Run("concept", func(t *testing.T) {
		vs := expandInline(t, ValueSetInclude{System: loincSystem, Concept: []ValueSetIncludeConcept{{Code: "10000-1"}, {Code: "12000-8"}}})
		if got := codes(vs); strings.Join(got, ",") != "10000-1,12000-8" {
			t.Errorf("concept[] codes = %v", got)
		}
	})
	t.Run("property equals coding", func(t *testing.T) {
		vs := expandInline(t, ValueSetInclude{System: loincSystem, Filter: []ValueSetIncludeFilter{{Property: "COMPONENT", Op: "=", Value: "LP1000-1"}}})
		if got := codes(vs); strings.Join(got, ",") != "10000-1,20000-8" {
			t.Errorf("COMPONENT=LP1000-1 codes = %v", got)
		}
	})
	t.Run("property regex coding matches anchored pattern", func(t *testing.T) {
		vs := expandInline(t, ValueSetInclude{System: loincSystem, Filter: []ValueSetIncludeFilter{{Property: "COMPONENT", Op: "regex", Value: "^Chol"}}})
		if got := codes(vs); strings.Join(got, ",") != "10000-1,20000-8" {
			t.Errorf("COMPONENT regex ^Chol codes = %v", got)
		}
	})
	t.Run("property regex coding anchored pattern excludes non-match", func(t *testing.T) {
		// A LIKE-substring implementation would match "Cholesterol" on a bare "hol" fragment
		// too, but "^hol" anchored to the start must not: this is the real-regex regression.
		vs := expandInline(t, ValueSetInclude{System: loincSystem, Filter: []ValueSetIncludeFilter{{Property: "COMPONENT", Op: "regex", Value: "^hol"}}})
		if got := codes(vs); len(got) != 0 {
			t.Errorf("COMPONENT regex ^hol should match nothing, got %v", got)
		}
	})
	t.Run("property regex string column", func(t *testing.T) {
		vs := expandInline(t, ValueSetInclude{System: loincSystem, Filter: []ValueSetIncludeFilter{{Property: "STATUS", Op: "regex", Value: "^DEPR"}}})
		got := codes(vs)
		if len(got) != 2 {
			t.Errorf("STATUS regex ^DEPR codes = %v", got)
		}
	})
	t.Run("property regex rejects oversized pattern", func(t *testing.T) {
		_, err := svc.Expand(ctx, ExpandParams{
			Inline: &InlineValueSet{Compose: &ValueSetCompose{Include: []ValueSetInclude{{System: loincSystem, Filter: []ValueSetIncludeFilter{{Property: "STATUS", Op: "regex", Value: strings.Repeat("a", 257)}}}}}},
			Count:  expandCount(100),
		})
		if err == nil || err.Status != 400 || err.Code != "invalid" {
			t.Fatalf("expected 400 invalid for an oversized regex pattern, got %+v", err)
		}
	})
	t.Run("property regex rejects a pattern that fails to compile", func(t *testing.T) {
		_, err := svc.Expand(ctx, ExpandParams{
			Inline: &InlineValueSet{Compose: &ValueSetCompose{Include: []ValueSetInclude{{System: loincSystem, Filter: []ValueSetIncludeFilter{{Property: "STATUS", Op: "regex", Value: "(unclosed"}}}}}},
			Count:  expandCount(100),
		})
		if err == nil || err.Status != 400 || err.Code != "invalid" {
			t.Fatalf("expected 400 invalid for an uncompilable regex pattern, got %+v", err)
		}
	})
	t.Run("property in string", func(t *testing.T) {
		vs := expandInline(t, ValueSetInclude{System: loincSystem, Filter: []ValueSetIncludeFilter{{Property: "STATUS", Op: "in", Value: "DEPRECATED"}}})
		got := codes(vs)
		if len(got) != 2 {
			t.Errorf("STATUS in DEPRECATED codes = %v", got)
		}
	})
	t.Run("copyright 3rdParty", func(t *testing.T) {
		vs := expandInline(t, ValueSetInclude{System: loincSystem, Filter: []ValueSetIncludeFilter{{Property: "copyright", Op: "=", Value: "3rdParty"}}})
		if got := codes(vs); strings.Join(got, ",") != "11000-0" {
			t.Errorf("copyright=3rdParty codes = %v", got)
		}
	})
	t.Run("copyright LOINC", func(t *testing.T) {
		vs := expandInline(t, ValueSetInclude{System: loincSystem, Filter: []ValueSetIncludeFilter{{Property: "copyright", Op: "=", Value: "LOINC"}}})
		got := codes(vs)
		for _, c := range got {
			if c == "11000-0" {
				t.Errorf("copyright=LOINC should exclude 11000-0, got %v", got)
			}
		}
	})
	t.Run("ancestor", func(t *testing.T) {
		vs := expandInline(t, ValueSetInclude{System: loincSystem, Filter: []ValueSetIncludeFilter{{Property: "ancestor", Op: "=", Value: "LP2000-1"}}})
		if got := codes(vs); strings.Join(got, ",") != "10000-1" {
			t.Errorf("ancestor=LP2000-1 codes = %v", got)
		}
	})
	t.Run("parent", func(t *testing.T) {
		vs := expandInline(t, ValueSetInclude{System: loincSystem, Filter: []ValueSetIncludeFilter{{Property: "parent", Op: "=", Value: "LP1000-1"}}})
		if got := codes(vs); strings.Join(got, ",") != "10000-1" {
			t.Errorf("parent=LP1000-1 codes = %v", got)
		}
	})
	t.Run("concept is-a", func(t *testing.T) {
		vs := expandInline(t, ValueSetInclude{System: loincSystem, Filter: []ValueSetIncludeFilter{{Property: "concept", Op: "is-a", Value: "LP1000-1"}}})
		if got := codes(vs); strings.Join(got, ",") != "10000-1" {
			t.Errorf("concept is-a LP1000-1 codes = %v", got)
		}
	})
	t.Run("concept descendent-of", func(t *testing.T) {
		vs := expandInline(t, ValueSetInclude{System: loincSystem, Filter: []ValueSetIncludeFilter{{Property: "concept", Op: "descendent-of", Value: "LP2000-1"}}})
		if got := codes(vs); strings.Join(got, ",") != "10000-1" {
			t.Errorf("concept descendent-of LP2000-1 codes = %v", got)
		}
	})
	t.Run("unsupported filter property", func(t *testing.T) {
		vs, err := svc.Expand(ctx, ExpandParams{
			Inline: &InlineValueSet{Compose: &ValueSetCompose{Include: []ValueSetInclude{{System: loincSystem, Filter: []ValueSetIncludeFilter{{Property: "not-a-real-property", Op: "=", Value: "x"}}}}}},
			Count:  expandCount(100),
		})
		if err == nil || err.Code != "not-supported" {
			t.Errorf("unsupported property: vs=%v err=%v, want not-supported", vs, err)
		}
	})
	t.Run("exclude", func(t *testing.T) {
		vs, err := svc.Expand(ctx, ExpandParams{
			Inline: &InlineValueSet{Compose: &ValueSetCompose{
				Include: []ValueSetInclude{{System: loincSystem}}, // bare system include = every term
				Exclude: []ValueSetInclude{{System: loincSystem, Concept: []ValueSetIncludeConcept{{Code: "20000-8"}, {Code: "13000-6"}}}},
			}},
			Count: expandCount(100),
		})
		if err != nil {
			t.Fatalf("expand exclude: %v", err)
		}
		got := codes(vs)
		for _, c := range got {
			if c == "20000-8" || c == "13000-6" {
				t.Errorf("exclude should drop %s, got %v", c, got)
			}
		}
		if len(got) != 3 {
			t.Errorf("exclude codes = %v, want 3 remaining", got)
		}
	})
}

func TestValidateValueSetCode(t *testing.T) {
	svc := newValueSetTestService(t)
	ctx := context.Background()

	hit, err := svc.ValidateValueSetCode(ctx, ValidateValueSetCodeParams{URL: "http://loinc.org/vs/LL1000-1", Code: "LA1-1"})
	if err != nil {
		t.Fatalf("validate hit: %v", err)
	}
	if !boolParam(t, hit, "result") {
		t.Errorf("hit result = false")
	}
	if s := stringParam(t, hit, "display"); s != "Positive" {
		t.Errorf("hit display = %q", s)
	}

	miss, err := svc.ValidateValueSetCode(ctx, ValidateValueSetCodeParams{URL: "http://loinc.org/vs/LL1000-1", Code: "10000-1"})
	if err != nil {
		t.Fatalf("validate miss: %v", err)
	}
	if boolParam(t, miss, "result") {
		t.Errorf("miss result = true")
	}
	if msg := stringParam(t, miss, "message"); !strings.Contains(msg, "was not found") {
		t.Errorf("miss message = %q", msg)
	}

	mismatch, err := svc.ValidateValueSetCode(ctx, ValidateValueSetCodeParams{URL: "http://loinc.org/vs/LL1000-1", Code: "LA1-1", Display: "Nope"})
	if err != nil {
		t.Fatalf("validate mismatch: %v", err)
	}
	if boolParam(t, mismatch, "result") {
		t.Errorf("mismatch result = true")
	}
	if msg := stringParam(t, mismatch, "message"); msg != "The code exists but the display is not valid" {
		t.Errorf("mismatch message = %q", msg)
	}

	if _, err := svc.ValidateValueSetCode(ctx, ValidateValueSetCodeParams{URL: "http://loinc.org/vs/LL9999-9", Code: "x"}); err == nil || err.Code != "not-found" {
		t.Errorf("unknown value set: err = %v, want not-found", err)
	}
}

func boolParam(t *testing.T, p *Parameters, name string) bool {
	t.Helper()
	for _, param := range p.Parameter {
		if param.Name == name && param.ValueBoolean != nil {
			return *param.ValueBoolean
		}
	}
	t.Fatalf("no boolean parameter %q in %+v", name, p)
	return false
}

func stringParam(t *testing.T, p *Parameters, name string) string {
	t.Helper()
	for _, param := range p.Parameter {
		if param.Name == name && param.ValueString != nil {
			return *param.ValueString
		}
	}
	return ""
}

func TestSearchValueSetsNameAndPaging(t *testing.T) {
	svc := newValueSetTestService(t)
	ctx := context.Background()

	// "chem" prefix-matches nothing by name:in substring except our two groups' "chemistry" names.
	bundle, err := svc.SearchValueSets(ctx, ValueSetSearchParams{NameContains: "chemistry", Count: 1, Offset: 0})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if bundle.Total != 2 {
		t.Fatalf("total = %d, want 2", bundle.Total)
	}
	if len(bundle.Entry) != 1 {
		t.Fatalf("page 1 entries = %d, want 1", len(bundle.Entry))
	}
	bundle2, err := svc.SearchValueSets(ctx, ValueSetSearchParams{NameContains: "chemistry", Count: 1, Offset: 1})
	if err != nil {
		t.Fatalf("search page 2: %v", err)
	}
	if len(bundle2.Entry) != 1 {
		t.Fatalf("page 2 entries = %d, want 1", len(bundle2.Entry))
	}

	byURL, err := svc.SearchValueSets(ctx, ValueSetSearchParams{URL: "http://loinc.org/vs/LL1000-1"})
	if err != nil {
		t.Fatalf("search by url: %v", err)
	}
	if byURL.Total != 1 {
		t.Errorf("search by url total = %d", byURL.Total)
	}

	byID, err := svc.SearchValueSets(ctx, ValueSetSearchParams{ID: "LG1000-1"})
	if err != nil {
		t.Fatalf("search by id: %v", err)
	}
	if byID.Total != 1 {
		t.Errorf("search by id total = %d", byID.Total)
	}

	unknown, err := svc.SearchValueSets(ctx, ValueSetSearchParams{URL: "http://loinc.org/vs/does-not-exist"})
	if err != nil {
		t.Fatalf("search unknown url: %v", err)
	}
	if unknown.Total != 0 {
		t.Errorf("search unknown url total = %d, want 0", unknown.Total)
	}
}
