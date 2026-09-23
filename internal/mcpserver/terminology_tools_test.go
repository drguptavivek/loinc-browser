package mcpserver

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"loinc-browser/internal/loinc"
	"loinc-browser/pkg/terminology"
)

func TestLookupCodeReturnsCompactAxesAndRelated(t *testing.T) {
	service := newTestService(t)

	result, err := service.LookupCode(context.Background(), LookupCodeRequest{Code: "1000-1"})
	if err != nil {
		t.Fatalf("lookup code: %v", err)
	}
	if result.Kind != "term" {
		t.Fatalf("expected kind term, got %q", result.Kind)
	}
	if result.Display == "" || result.Status == "" {
		t.Fatalf("expected display and status, got %#v", result)
	}
	found := false
	for _, axis := range result.Axes {
		if axis.Axis == "COMPONENT" && axis.Code == "LP1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected COMPONENT axis LP1, got %#v", result.Axes)
	}
	if result.BrowserURL != "/?term=1000-1" {
		t.Fatalf("unexpected browser url %q", result.BrowserURL)
	}
	if result.Raw != nil {
		t.Fatalf("rawFhir should be omitted by default, got %#v", result.Raw)
	}
}

func TestLookupCodePartKind(t *testing.T) {
	service := newTestService(t)

	result, err := service.LookupCode(context.Background(), LookupCodeRequest{Code: "LP1", RawFHIR: true})
	if err != nil {
		t.Fatalf("lookup part: %v", err)
	}
	if result.Kind != "part" {
		t.Fatalf("expected kind part, got %q", result.Kind)
	}
	if result.Raw == nil {
		t.Fatal("expected rawFhir when requested")
	}
}

func TestLookupCodeUnknownCodeIsToolError(t *testing.T) {
	service := newTestService(t)

	_, err := service.LookupCode(context.Background(), LookupCodeRequest{Code: "999999-9"})
	if err == nil {
		t.Fatal("expected error for unknown code")
	}
	var oe *terminology.OutcomeError
	if !errors.As(err, &oe) {
		t.Fatalf("expected *terminology.OutcomeError, got %T: %v", err, err)
	}
}

func TestValidateCodeActiveAndUnknown(t *testing.T) {
	service := newTestService(t)

	active, err := service.ValidateCode(context.Background(), ValidateCodeRequest{Code: "1000-1"})
	if err != nil {
		t.Fatalf("validate active code: %v", err)
	}
	if !active.Valid || !active.Active {
		t.Fatalf("expected valid active code, got %#v", active)
	}

	unknown, err := service.ValidateCode(context.Background(), ValidateCodeRequest{Code: "999999-9"})
	if err != nil {
		t.Fatalf("validate unknown code should not transport-error: %v", err)
	}
	if unknown.Valid {
		t.Fatalf("expected invalid result for unknown code, got %#v", unknown)
	}
}

func TestSubsumesEquivalent(t *testing.T) {
	service := newTestService(t)

	result, err := service.Subsumes(context.Background(), SubsumesRequest{CodeA: "1000-1", CodeB: "1000-1"})
	if err != nil {
		t.Fatalf("subsumes: %v", err)
	}
	if result.Outcome != "equivalent" {
		t.Fatalf("expected equivalent outcome, got %q", result.Outcome)
	}
}

func TestSubsumesUnknownCodeIsToolError(t *testing.T) {
	service := newTestService(t)

	_, err := service.Subsumes(context.Background(), SubsumesRequest{CodeA: "1000-1", CodeB: "999999-9"})
	if err == nil {
		t.Fatal("expected error for unknown codeB")
	}
}

func TestExpandValueSetAnswerListCapsCount(t *testing.T) {
	service := newTestService(t)

	result, err := service.ExpandValueSet(context.Background(), ExpandValueSetRequest{URL: "http://loinc.org/vs/LL1", Count: 5000})
	if err != nil {
		t.Fatalf("expand value set: %v", err)
	}
	if result.Count != maxExpandRows {
		t.Fatalf("expected count capped at %d, got %d", maxExpandRows, result.Count)
	}
	if len(result.Members) == 0 || result.Members[0].Code != "LA1" {
		t.Fatalf("expected LA1 member, got %#v", result.Members)
	}
}

func TestExpandValueSetUnknownURLIsToolError(t *testing.T) {
	service := newTestService(t)

	_, err := service.ExpandValueSet(context.Background(), ExpandValueSetRequest{URL: "http://loinc.org/vs/does-not-exist"})
	if err == nil {
		t.Fatal("expected error for unknown value set")
	}
}

func TestSearchValueSetsFindsAnswerList(t *testing.T) {
	service := newTestService(t)

	results, err := service.SearchValueSets(context.Background(), SearchValueSetsRequest{NameContains: "positive"})
	if err != nil {
		t.Fatalf("search value sets: %v", err)
	}
	if len(results) == 0 || results[0].ID == "" {
		t.Fatalf("expected at least one value set, got %#v", results)
	}
}

func TestValidateValueSetMembership(t *testing.T) {
	service := newTestService(t)

	member, err := service.ValidateValueSetMembership(context.Background(), ValidateValueSetMembershipRequest{URL: "http://loinc.org/vs/LL1", Code: "LA1"})
	if err != nil {
		t.Fatalf("validate membership: %v", err)
	}
	if !member.Member {
		t.Fatalf("expected LA1 to be a member, got %#v", member)
	}

	nonMember, err := service.ValidateValueSetMembership(context.Background(), ValidateValueSetMembershipRequest{URL: "http://loinc.org/vs/LL1", Code: "LA999"})
	if err != nil {
		t.Fatalf("validate non-membership: %v", err)
	}
	if nonMember.Member {
		t.Fatalf("expected LA999 to not be a member, got %#v", nonMember)
	}
}

func TestTranslateFindsMapToMatch(t *testing.T) {
	service := newTestService(t)

	result, err := service.Translate(context.Background(), TranslateRequest{Code: "1002-7", ID: "loinc-map-to"})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if !result.Result || len(result.Matches) == 0 {
		t.Fatalf("expected a match via loinc-map-to, got %#v", result)
	}
	if result.Matches[0].Code != "1000-1" {
		t.Fatalf("expected target code 1000-1, got %#v", result.Matches[0])
	}
}

func TestTranslateDefaultsSystemToLOINC(t *testing.T) {
	service := newTestService(t)

	// No system/url/conceptMapId given: unlike FHIR's own $translate route (which requires an
	// explicit system), the MCP tool should default to searching every map from http://loinc.org.
	result, err := service.Translate(context.Background(), TranslateRequest{Code: "1002-7"})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if !result.Result || len(result.Matches) == 0 {
		t.Fatalf("expected a match via the default http://loinc.org system, got %#v", result)
	}
	if !strings.Contains(result.FHIRURL, "system=http%3A%2F%2Floinc.org") {
		t.Fatalf("expected fhirUrl to reflect the defaulted system, got %q", result.FHIRURL)
	}
}

func TestTranslateUnknownConceptMapIsToolError(t *testing.T) {
	service := newTestService(t)

	_, err := service.Translate(context.Background(), TranslateRequest{Code: "1002-7", ID: "does-not-exist"})
	if err == nil {
		t.Fatal("expected error for unknown concept map id")
	}
}

func TestListConceptMaps(t *testing.T) {
	service := newTestService(t)

	results, err := service.ListConceptMaps(context.Background(), ListConceptMapsRequest{})
	if err != nil {
		t.Fatalf("list concept maps: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one served concept map")
	}
}

func TestGetQuestionnaireReturnsCompactItemTree(t *testing.T) {
	service := newTestService(t)

	result, err := service.GetQuestionnaire(context.Background(), GetQuestionnaireRequest{LOINCNum: "1001-9"})
	if err != nil {
		t.Fatalf("get questionnaire: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected two questionnaire items, got %#v", result.Items)
	}
	if result.Items[0].Code != "1000-1" {
		t.Fatalf("expected first item code 1000-1, got %#v", result.Items[0])
	}
}

func TestGetQuestionnaireNonPanelIsToolError(t *testing.T) {
	service := newTestService(t)

	_, err := service.GetQuestionnaire(context.Background(), GetQuestionnaireRequest{LOINCNum: "1000-1"})
	if err == nil {
		t.Fatal("expected error for non-panel LOINC number")
	}
}

func TestLuceneSearchWithoutFuncIsNotAvailable(t *testing.T) {
	service := newTestService(t)

	_, err := service.LuceneSearch(context.Background(), LuceneSearchRequest{Scope: "loincs", Query: "glucose"})
	if err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected not-available error, got %v", err)
	}
}

func TestLuceneSearchCapsRowsAndBuildsCompactResult(t *testing.T) {
	service := newTestService(t)
	service.luceneSearch = func(ctx context.Context, scope, query string, rows, offset int) ([]loinc.LocalSearchResult, uint64, error) {
		if rows != maxLuceneRows {
			t.Fatalf("expected rows capped at %d, got %d", maxLuceneRows, rows)
		}
		return []loinc.LocalSearchResult{
			{LocalSearchHit: loinc.LocalSearchHit{Key: "1000-1", Score: 1.5}, Result: loinc.SearchResult{LOINCNum: "1000-1", ShortName: "Glucose P", Status: "ACTIVE"}},
		}, 1, nil
	}

	result, err := service.LuceneSearch(context.Background(), LuceneSearchRequest{Scope: "loincs", Query: "glucose", Rows: 9999})
	if err != nil {
		t.Fatalf("lucene search: %v", err)
	}
	if result.Total != 1 || len(result.Rows) != 1 {
		t.Fatalf("unexpected result %#v", result)
	}
	if result.Rows[0].Display != "Glucose P" || result.Rows[0].BrowserURL != "/?term=1000-1" {
		t.Fatalf("unexpected row %#v", result.Rows[0])
	}
}

func TestLuceneSearchRejectsUnknownScope(t *testing.T) {
	service := newTestService(t)
	service.luceneSearch = func(ctx context.Context, scope, query string, rows, offset int) ([]loinc.LocalSearchResult, uint64, error) {
		return nil, 0, nil
	}

	_, err := service.LuceneSearch(context.Background(), LuceneSearchRequest{Scope: "bogus", Query: "x"})
	if err == nil {
		t.Fatal("expected error for unsupported scope")
	}
}

func TestHotSwapGetStoreIsResolvedPerCall(t *testing.T) {
	service := newTestService(t)
	original := service.getStore
	failing := false
	service.getStore = func() (*loinc.Store, error) {
		if failing {
			return nil, errors.New("store swapped out")
		}
		return original()
	}

	if _, err := service.GetTerm(context.Background(), LOINCRequest{LOINCNum: "1000-1"}); err != nil {
		t.Fatalf("expected get term to succeed before swap: %v", err)
	}
	failing = true
	if _, err := service.GetTerm(context.Background(), LOINCRequest{LOINCNum: "1000-1"}); err == nil {
		t.Fatal("expected get term to fail once the store getter starts failing")
	}
}

// TestToolsListIncludesNewToolsWithReadOnlyAnnotations exercises the SDK's in-memory transport to
// confirm every terminology/search-api tool is registered, read-only/idempotent, and that a
// successful call returns typed StructuredContent (SDK v1.8.0's automatic typed-output support).
func TestToolsListIncludesNewToolsWithReadOnlyAnnotations(t *testing.T) {
	service := newTestService(t)
	server := mcp.NewServer(&mcp.Implementation{Name: "loinc-browser-test", Version: "test"}, nil)
	registerTools(server, service)

	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("connect server: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect client: %v", err)
	}
	defer session.Close()

	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	byName := map[string]*mcp.Tool{}
	for _, tool := range listed.Tools {
		byName[tool.Name] = tool
	}
	wantNewTools := []string{
		"loinc_lookup_code", "loinc_validate_code", "loinc_subsumes",
		"loinc_expand_value_set", "loinc_search_value_sets", "loinc_validate_value_set_membership",
		"loinc_translate", "loinc_list_concept_maps", "loinc_get_questionnaire", "loinc_lucene_search",
	}
	for _, name := range wantNewTools {
		tool, ok := byName[name]
		if !ok {
			t.Fatalf("expected tool %s to be listed", name)
		}
		if tool.Annotations == nil || tool.Annotations.ReadOnlyHint != true {
			t.Fatalf("expected %s to be read-only, got %#v", name, tool.Annotations)
		}
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "loinc_lookup_code", Arguments: map[string]any{"code": "1000-1"}})
	if err != nil {
		t.Fatalf("call loinc_lookup_code: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected successful call, got error content %#v", result.Content)
	}
	if result.StructuredContent == nil {
		t.Fatal("expected structuredContent on a successful typed tool call")
	}

	errResult, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "loinc_lookup_code", Arguments: map[string]any{"code": "999999-9"}})
	if err != nil {
		t.Fatalf("call loinc_lookup_code with unknown code (transport): %v", err)
	}
	if !errResult.IsError {
		t.Fatalf("expected IsError for unknown code, got %#v", errResult)
	}
}

// TestToolsListAcrossProtocolVersions confirms the server answers tools/list and tools/call
// identically whether the client negotiates the legacy 2025-11-25 initialize handshake or the
// stateless 2026-07-28 protocol (SDK v1.8.0 supports both; nil ServerOptions.SupportedProtocolVersions
// advertises every version the SDK knows).
func TestToolsListAcrossProtocolVersions(t *testing.T) {
	for _, version := range []string{"2025-11-25", "2026-07-28"} {
		t.Run(version, func(t *testing.T) {
			service := newTestService(t)
			server := mcp.NewServer(&mcp.Implementation{Name: "loinc-browser-test", Version: "test"}, nil)
			registerTools(server, service)

			ctx := context.Background()
			clientTransport, serverTransport := mcp.NewInMemoryTransports()
			if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
				t.Fatalf("connect server: %v", err)
			}
			client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
			session, err := client.Connect(ctx, clientTransport, &mcp.ClientSessionOptions{ProtocolVersion: version})
			if err != nil {
				t.Fatalf("connect client at %s: %v", version, err)
			}
			defer session.Close()

			listed, err := session.ListTools(ctx, nil)
			if err != nil {
				t.Fatalf("list tools at %s: %v", version, err)
			}
			if len(listed.Tools) < 23 {
				t.Fatalf("expected at least 23 tools at %s, got %d", version, len(listed.Tools))
			}

			result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "loinc_translate", Arguments: map[string]any{"code": "1002-7", "conceptMapId": "loinc-map-to"}})
			if err != nil {
				t.Fatalf("call loinc_translate at %s: %v", version, err)
			}
			if result.IsError {
				t.Fatalf("expected successful call at %s, got %#v", version, result.Content)
			}
			if result.StructuredContent == nil {
				t.Fatalf("expected structuredContent at %s", version)
			}
		})
	}
}
