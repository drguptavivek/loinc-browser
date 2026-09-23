package terminology

import (
	"context"
	"path/filepath"
	"testing"

	"loinc-browser/internal/loinc"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	ctx := context.Background()
	releaseDir := writeTerminologyTestRelease(t)
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

func findParam(params []Parameter, name string) (Parameter, bool) {
	for _, p := range params {
		if p.Name == name {
			return p, true
		}
	}
	return Parameter{}, false
}

func findParamAt(params []Parameter, name string, occurrence int) (Parameter, bool) {
	n := 0
	for _, p := range params {
		if p.Name == name {
			if n == occurrence {
				return p, true
			}
			n++
		}
	}
	return Parameter{}, false
}

func partValue(p Parameter, name string) (Parameter, bool) {
	return findParam(p.Part, name)
}

func TestLookupTerm(t *testing.T) {
	svc := newTestService(t)
	result, err := svc.Lookup(context.Background(), LookupParams{Code: "10000-1"})
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	code, _ := findParam(result.Parameter, "code")
	if code.ValueCode == nil || *code.ValueCode != "10000-1" {
		t.Fatalf("code = %+v", code)
	}
	system, _ := findParam(result.Parameter, "system")
	if system.ValueString == nil || *system.ValueString != "http://loinc.org" {
		t.Fatalf("system = %+v", system)
	}
	version, _ := findParam(result.Parameter, "version")
	if version.ValueString == nil || *version.ValueString != "2.80" {
		t.Fatalf("version = %+v", version)
	}
	display, _ := findParam(result.Parameter, "display")
	if display.ValueString == nil || *display.ValueString != "Cholesterol [Mass/volume] in Serum" {
		t.Fatalf("display = %+v", display)
	}
	status, _ := findParam(result.Parameter, "status")
	if status.ValueCode == nil || *status.ValueCode != "active" {
		t.Fatalf("status = %+v", status)
	}

	// en-US LONG_COMMON_NAME designation must be first.
	firstDesignation, ok := findParamAt(result.Parameter, "designation", 0)
	if !ok {
		t.Fatal("no designation parameters")
	}
	lang, _ := partValue(firstDesignation, "language")
	if lang.ValueCode == nil || *lang.ValueCode != "en-US" {
		t.Fatalf("first designation language = %+v", lang)
	}
	use, _ := partValue(firstDesignation, "use")
	if use.ValueCoding == nil || use.ValueCoding.Code != "LONG_COMMON_NAME" {
		t.Fatalf("first designation use = %+v", use)
	}

	// Both linguistic variants (German and French, item 7's UNION ALL across per-language raw
	// tables) must appear, in LinguisticVariants.csv ID order (German ID=1 before French ID=2).
	var designationLanguages []string
	for _, p := range result.Parameter {
		if p.Name != "designation" {
			continue
		}
		lang, _ := partValue(p, "language")
		if lang.ValueCode != nil {
			designationLanguages = append(designationLanguages, *lang.ValueCode)
		}
	}
	firstGerman, firstFrench := -1, -1
	for i, lang := range designationLanguages {
		if lang == "de-DE" && firstGerman == -1 {
			firstGerman = i
		}
		if lang == "fr-FR" && firstFrench == -1 {
			firstFrench = i
		}
	}
	if firstGerman == -1 {
		t.Fatalf("expected a de-DE designation, got languages %v", designationLanguages)
	}
	if firstFrench == -1 {
		t.Fatalf("expected a fr-FR designation, got languages %v", designationLanguages)
	}
	if firstGerman > firstFrench {
		t.Fatalf("expected de-DE (LinguisticVariants ID=1) before fr-FR (ID=2), got languages %v", designationLanguages)
	}

	// ConsumerName designation comes from the accessory file, not the blank Loinc.csv column.
	foundConsumerName := false
	for _, p := range result.Parameter {
		if p.Name != "designation" {
			continue
		}
		use, _ := partValue(p, "use")
		if use.ValueCoding != nil && use.ValueCoding.Code == "ConsumerName" {
			foundConsumerName = true
			value, _ := partValue(p, "value")
			if value.ValueString == nil || *value.ValueString != "Cholesterol, Blood" {
				t.Fatalf("ConsumerName value = %+v", value)
			}
		}
	}
	if !foundConsumerName {
		t.Fatal("expected a ConsumerName designation")
	}

	// COMPONENT axis property must resolve to the LP part.
	foundComponent := false
	foundParentHierarchy := false
	foundParentGroup := false
	foundAnswerList := false
	for _, p := range result.Parameter {
		if p.Name != "property" {
			continue
		}
		code, _ := partValue(p, "code")
		value, _ := partValue(p, "value")
		if code.ValueCode == nil {
			continue
		}
		switch *code.ValueCode {
		case "COMPONENT":
			foundComponent = true
			if value.ValueCoding == nil || value.ValueCoding.Code != "LP1000-1" {
				t.Fatalf("COMPONENT property = %+v", value)
			}
		case "parent":
			if value.ValueCoding == nil {
				continue
			}
			if value.ValueCoding.Code == "LP1000-1" {
				foundParentHierarchy = true
			}
			if value.ValueCoding.Code == "LG1000-1" {
				foundParentGroup = true
			}
		case "answer-list":
			foundAnswerList = true
			if value.ValueCoding == nil || value.ValueCoding.Code != "LL1000-1" {
				t.Fatalf("answer-list property = %+v", value)
			}
		}
	}
	if !foundComponent {
		t.Fatal("expected a COMPONENT property")
	}
	if !foundParentHierarchy {
		t.Fatal("expected a hierarchy parent property (LP1000-1)")
	}
	if !foundParentGroup {
		t.Fatal("expected a group parent property (LG1000-1)")
	}
	if !foundAnswerList {
		t.Fatal("expected an answer-list property (LL1000-1)")
	}
}

func TestLookupTermPropertyFilter(t *testing.T) {
	svc := newTestService(t)
	result, err := svc.Lookup(context.Background(), LookupParams{Code: "10000-1", Property: []string{"COMPONENT"}})
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	count := 0
	for _, p := range result.Parameter {
		if p.Name == "property" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 property with the filter, got %d", count)
	}
}

func TestLookupPart(t *testing.T) {
	svc := newTestService(t)
	result, err := svc.Lookup(context.Background(), LookupParams{Code: "lp1000-1"}) // lower-case input
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	code, _ := findParam(result.Parameter, "code")
	if code.ValueCode == nil || *code.ValueCode != "LP1000-1" {
		t.Fatalf("code should be normalized to upper case, got %+v", code)
	}
	display, _ := findParam(result.Parameter, "display")
	if display.ValueString == nil || *display.ValueString != "Cholesterol" {
		t.Fatalf("display = %+v", display)
	}
	foundParent, foundChild := false, false
	for _, p := range result.Parameter {
		if p.Name != "property" {
			continue
		}
		code, _ := partValue(p, "code")
		value, _ := partValue(p, "value")
		if code.ValueCode == nil || value.ValueCoding == nil {
			continue
		}
		if *code.ValueCode == "parent" && value.ValueCoding.Code == "LP2000-1" {
			foundParent = true
		}
		if *code.ValueCode == "child" && value.ValueCoding.Code == "10000-1" {
			foundChild = true
		}
	}
	if !foundParent {
		t.Fatal("expected parent LP2000-1")
	}
	if !foundChild {
		t.Fatal("expected child 10000-1")
	}
}

func TestLookupAnswerList(t *testing.T) {
	svc := newTestService(t)
	result, err := svc.Lookup(context.Background(), LookupParams{Code: "LL1000-1"})
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	display, _ := findParam(result.Parameter, "display")
	if display.ValueString == nil || *display.ValueString != "Positive negative" {
		t.Fatalf("display = %+v", display)
	}
	foundAnswersFor, childCount := false, 0
	for _, p := range result.Parameter {
		if p.Name != "property" {
			continue
		}
		code, _ := partValue(p, "code")
		value, _ := partValue(p, "value")
		if code.ValueCode == nil {
			continue
		}
		switch *code.ValueCode {
		case "answers-for":
			if value.ValueCoding != nil && value.ValueCoding.Code == "10000-1" {
				foundAnswersFor = true
			}
		case "child":
			childCount++
		}
	}
	if !foundAnswersFor {
		t.Fatal("expected answers-for 10000-1")
	}
	if childCount != 2 {
		t.Fatalf("expected 2 child answers, got %d", childCount)
	}
}

func TestLookupAnswer(t *testing.T) {
	svc := newTestService(t)
	result, err := svc.Lookup(context.Background(), LookupParams{Code: "LA1-1"})
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	display, _ := findParam(result.Parameter, "display")
	if display.ValueString == nil || *display.ValueString != "Positive" {
		t.Fatalf("display = %+v", display)
	}
	score, _ := findParam(result.Parameter, "property")
	_ = score
	foundScore, foundParent := false, false
	for _, p := range result.Parameter {
		if p.Name != "property" {
			continue
		}
		code, _ := partValue(p, "code")
		value, _ := partValue(p, "value")
		if code.ValueCode == nil {
			continue
		}
		if *code.ValueCode == "Score" && value.ValueString != nil && *value.ValueString == "1" {
			foundScore = true
		}
		if *code.ValueCode == "parent" && value.ValueCoding != nil && value.ValueCoding.Code == "LL1000-1" {
			foundParent = true
		}
	}
	if !foundScore {
		t.Fatal("expected Score=1")
	}
	if !foundParent {
		t.Fatal("expected parent LL1000-1")
	}
}

func TestLookupGroup(t *testing.T) {
	svc := newTestService(t)
	result, err := svc.Lookup(context.Background(), LookupParams{Code: "LG1000-1"})
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	display, _ := findParam(result.Parameter, "display")
	if display.ValueString == nil || *display.ValueString != "Chemistry tests" {
		t.Fatalf("display = %+v", display)
	}
	foundParent, foundChild := false, false
	for _, p := range result.Parameter {
		if p.Name != "property" {
			continue
		}
		code, _ := partValue(p, "code")
		value, _ := partValue(p, "value")
		if code.ValueCode == nil {
			continue
		}
		if *code.ValueCode == "parent" && value.ValueCoding != nil && value.ValueCoding.Code == "PG1000" {
			foundParent = true
		}
		if *code.ValueCode == "child" && value.ValueCoding != nil && value.ValueCoding.Code == "10000-1" {
			foundChild = true
		}
	}
	if !foundParent {
		t.Fatal("expected parent PG1000")
	}
	if !foundChild {
		t.Fatal("expected child 10000-1")
	}
}

func TestLookupDeprecatedTerm(t *testing.T) {
	svc := newTestService(t)
	result, err := svc.Lookup(context.Background(), LookupParams{Code: "20000-8"})
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	status, _ := findParam(result.Parameter, "status")
	if status.ValueCode == nil || *status.ValueCode != "retired" {
		t.Fatalf("status = %+v", status)
	}
	foundMapTo := false
	for _, p := range result.Parameter {
		if p.Name != "property" {
			continue
		}
		code, _ := partValue(p, "code")
		value, _ := partValue(p, "value")
		if code.ValueCode != nil && *code.ValueCode == "MAP_TO" && value.ValueCoding != nil && value.ValueCoding.Code == "10000-1" {
			foundMapTo = true
		}
	}
	if !foundMapTo {
		t.Fatal("expected MAP_TO 10000-1")
	}
}

func TestLookupUnknown(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.Lookup(context.Background(), LookupParams{Code: "99999-9"})
	if err == nil {
		t.Fatal("expected an error")
	}
	if err.Status != 404 || err.Code != "not-found" {
		t.Fatalf("err = %+v", err)
	}
	if err.Text != "Unable to find code for system/version = 99999-9" {
		t.Fatalf("err.Text = %q", err.Text)
	}
}

func TestValidateCodeTrue(t *testing.T) {
	svc := newTestService(t)
	result, err := svc.ValidateCode(context.Background(), ValidateCodeParams{Code: "10000-1"})
	if err != nil {
		t.Fatalf("validate-code: %v", err)
	}
	res, _ := findParam(result.Parameter, "result")
	if res.ValueString == nil || *res.ValueString != "true" {
		t.Fatalf("result = %+v", res)
	}
	active, _ := findParam(result.Parameter, "active")
	if active.ValueBoolean == nil || !*active.ValueBoolean {
		t.Fatalf("active = %+v", active)
	}
}

func TestValidateCodeBadDisplay(t *testing.T) {
	svc := newTestService(t)
	result, err := svc.ValidateCode(context.Background(), ValidateCodeParams{Code: "10000-1", Display: "Not the right display"})
	if err != nil {
		t.Fatalf("validate-code: %v", err)
	}
	res, _ := findParam(result.Parameter, "result")
	if res.ValueString == nil || *res.ValueString != "false" {
		t.Fatalf("result = %+v", res)
	}
	message, _ := findParam(result.Parameter, "message")
	if message.ValueString == nil || *message.ValueString != "The code exists but the display is not valid" {
		t.Fatalf("message = %+v", message)
	}
}

func TestValidateCodeUnknown(t *testing.T) {
	svc := newTestService(t)
	result, err := svc.ValidateCode(context.Background(), ValidateCodeParams{Code: "99999-9"})
	if err != nil {
		t.Fatalf("validate-code: %v", err)
	}
	res, _ := findParam(result.Parameter, "result")
	if res.ValueString == nil || *res.ValueString != "false" {
		t.Fatalf("result = %+v", res)
	}
	message, _ := findParam(result.Parameter, "message")
	if message.ValueString == nil || *message.ValueString != "The code does not exist for the supplied code system and/or version" {
		t.Fatalf("message = %+v", message)
	}
	if _, ok := findParam(result.Parameter, "display"); ok {
		t.Fatal("unknown code must not carry a display parameter")
	}
}

func TestValidateCodeDeprecatedInactive(t *testing.T) {
	svc := newTestService(t)
	result, err := svc.ValidateCode(context.Background(), ValidateCodeParams{Code: "20000-8"})
	if err != nil {
		t.Fatalf("validate-code: %v", err)
	}
	res, _ := findParam(result.Parameter, "result")
	if res.ValueString == nil || *res.ValueString != "true" {
		t.Fatalf("result = %+v", res)
	}
	active, _ := findParam(result.Parameter, "active")
	if active.ValueBoolean == nil || *active.ValueBoolean {
		t.Fatalf("active should be false for a deprecated term, got %+v", active)
	}
}

func TestSubsumes(t *testing.T) {
	svc := newTestService(t)
	cases := []struct {
		name, a, b, want string
	}{
		{"equivalent", "10000-1", "10000-1", "equivalent"},
		{"part subsumes term", "LP1000-1", "10000-1", "subsumes"},
		{"term subsumed-by part", "10000-1", "LP1000-1", "subsumed-by"},
		{"part subsumes part", "LP2000-1", "LP1000-1", "subsumes"},
		{"not subsumed", "LP2000-1", "20000-8", "not-subsumed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := svc.Subsumes(context.Background(), SubsumesParams{CodeA: tc.a, CodeB: tc.b})
			if err != nil {
				t.Fatalf("subsumes: %v", err)
			}
			outcome, _ := findParam(result.Parameter, "outcome")
			if outcome.ValueString == nil || *outcome.ValueString != tc.want {
				t.Fatalf("outcome = %+v, want %s", outcome, tc.want)
			}
		})
	}
}

func TestSubsumesUnknown(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.Subsumes(context.Background(), SubsumesParams{CodeA: "99999-9", CodeB: "10000-1"})
	if err == nil {
		t.Fatal("expected an error")
	}
	if err.Status != 400 || err.Code != "invalid" {
		t.Fatalf("err = %+v", err)
	}
}

func TestSubsumesNotBelowLP2000(t *testing.T) {
	// LP2000-1 does not subsume the deprecated term (20000-8 is not in the hierarchy fixture).
	svc := newTestService(t)
	result, err := svc.Subsumes(context.Background(), SubsumesParams{CodeA: "LP2000-1", CodeB: "20000-8"})
	if err != nil {
		t.Fatalf("subsumes: %v", err)
	}
	outcome, _ := findParam(result.Parameter, "outcome")
	if outcome.ValueString == nil || *outcome.ValueString != "not-subsumed" {
		t.Fatalf("outcome = %+v", outcome)
	}
}

// TestSubsumesHierarchyOnlyPart guards against assertCodeExists rejecting a hierarchy-only LP
// node (present in hierarchy_concepts, never published as a Part.csv row) with a 400 "invalid"
// instead of resolving it through the same path $lookup uses. LP3000-1 is such a node in the
// fixture (child of LP2000-1, no Part.csv row).
func TestSubsumesHierarchyOnlyPart(t *testing.T) {
	svc := newTestService(t)

	result, err := svc.Subsumes(context.Background(), SubsumesParams{CodeA: "LP2000-1", CodeB: "LP3000-1"})
	if err != nil {
		t.Fatalf("subsumes: %+v", err)
	}
	outcome, _ := findParam(result.Parameter, "outcome")
	if outcome.ValueString == nil || *outcome.ValueString != "subsumes" {
		t.Fatalf("outcome = %+v, want subsumes", outcome)
	}

	reversed, err := svc.Subsumes(context.Background(), SubsumesParams{CodeA: "LP3000-1", CodeB: "LP2000-1"})
	if err != nil {
		t.Fatalf("subsumes reversed: %+v", err)
	}
	outcome, _ = findParam(reversed.Parameter, "outcome")
	if outcome.ValueString == nil || *outcome.ValueString != "subsumed-by" {
		t.Fatalf("outcome = %+v, want subsumed-by", outcome)
	}
}

func TestCapabilitiesAndTerminologyCapabilities(t *testing.T) {
	svc := newTestService(t)
	cs, err := svc.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("capabilities: %v", err)
	}
	if cs.ResourceType != "CapabilityStatement" || cs.FhirVersion != "4.0.1" {
		t.Fatalf("capabilities = %+v", cs)
	}

	resourceByType := map[string]CapabilityResource{}
	for _, r := range cs.Rest[0].Resource {
		resourceByType[r.Type] = r
	}
	for _, want := range []string{"CodeSystem", "ValueSet", "ConceptMap", "Questionnaire"} {
		r, ok := resourceByType[want]
		if !ok {
			t.Fatalf("expected a %s resource in CapabilityStatement, got %+v", want, resourceByType)
		}
		hasRead, hasSearch := false, false
		for _, in := range r.Interaction {
			hasRead = hasRead || in.Code == "read"
			hasSearch = hasSearch || in.Code == "search-type"
		}
		if !hasRead || !hasSearch {
			t.Fatalf("%s should support read+search-type, got %+v", want, r.Interaction)
		}
	}
	codeSystemParams := map[string]bool{}
	for _, p := range resourceByType["CodeSystem"].SearchParam {
		codeSystemParams[p.Name] = true
	}
	if codeSystemParams["code"] {
		t.Fatalf("CodeSystem search-type does not support 'code'; capability should not advertise it")
	}
	conceptMapOpNames := map[string]bool{}
	for _, op := range resourceByType["ConceptMap"].Operation {
		conceptMapOpNames[op.Name] = true
	}
	if !conceptMapOpNames["translate"] {
		t.Fatalf("ConceptMap resource should advertise the translate operation, got %+v", resourceByType["ConceptMap"].Operation)
	}
	conceptMapParams := map[string]bool{}
	for _, p := range resourceByType["ConceptMap"].SearchParam {
		conceptMapParams[p.Name] = true
	}
	for _, want := range []string{"url", "source-system", "target-system", "source-code", "target-code"} {
		if !conceptMapParams[want] {
			t.Fatalf("ConceptMap search params missing %q, got %+v", want, resourceByType["ConceptMap"].SearchParam)
		}
	}

	tc, err := svc.TerminologyCapabilities(context.Background())
	if err != nil {
		t.Fatalf("terminology capabilities: %v", err)
	}
	if tc.ResourceType != "TerminologyCapabilities" || len(tc.CodeSystem) != 1 || tc.CodeSystem[0].Version[0].Code != "2.80" {
		t.Fatalf("terminology capabilities = %+v", tc)
	}
	if tc.Translation.NeedsMap {
		t.Fatalf("terminology capabilities translation.needsMap should be false (we serve local ConceptMaps), got %+v", tc.Translation)
	}
}

func TestCodeSystemResourceAndSearch(t *testing.T) {
	svc := newTestService(t)
	cs, err := svc.CodeSystemResource(context.Background(), "loinc")
	if err != nil {
		t.Fatalf("read loinc: %v", err)
	}
	if cs.URL != "http://loinc.org" || len(cs.Property) != 83 {
		t.Fatalf("codesystem = url=%s properties=%d", cs.URL, len(cs.Property))
	}
	if _, err := svc.CodeSystemResource(context.Background(), "loinc-2.80"); err != nil {
		t.Fatalf("read loinc-2.80: %v", err)
	}
	if _, err := svc.CodeSystemResource(context.Background(), "nope"); err == nil {
		t.Fatal("expected not-found for an unknown id")
	}

	bundle, err := svc.SearchCodeSystem(context.Background(), "http://loinc.org", "")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if bundle.Total != 1 || len(bundle.Entry) != 1 {
		t.Fatalf("bundle = %+v", bundle)
	}

	empty, err := svc.SearchCodeSystem(context.Background(), "http://example.org/not-loinc", "")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if empty.Total != 0 {
		t.Fatalf("expected an empty bundle for an unrelated url, got %+v", empty)
	}
}
