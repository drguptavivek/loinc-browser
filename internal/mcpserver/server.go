package mcpserver

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"loinc-browser/internal/loinc"
	"loinc-browser/internal/version"
	"loinc-browser/pkg/terminology"
)

type Options struct {
	// StoreGetter, when set, is resolved fresh on every tool call so a store hot-swapped after an
	// upload import (internal/server's app.currentStore) is always picked up. Store is a
	// back-compat fallback for callers (the stdio `mcp` command, existing tests) that only ever
	// have one fixed store for the process lifetime; New wraps it in an equivalent getter when
	// StoreGetter is nil.
	StoreGetter func() (*loinc.Store, error)
	Store       *loinc.Store
	DocsDir     string
	OpenAPIJSON func() string
	// Terminology, when non-nil, is the pkg/terminology Service the FHIR-backed tools
	// (loinc_lookup_code, loinc_validate_code, loinc_subsumes, loinc_expand_value_set,
	// loinc_search_value_sets, loinc_validate_value_set_membership, loinc_translate,
	// loinc_list_concept_maps, loinc_get_questionnaire) delegate to. Reuse the same
	// store-getter-backed Service internal/server.New builds (via its Options.Terminology
	// out-param) rather than constructing a second one. Nil disables those tools' registration.
	Terminology *terminology.Service
	// LuceneSearch, when non-nil, backs loinc_lucene_search via the same local Bleve index
	// /searchapi and /api/v1/local-search/query use. Nil (e.g. the stdio mcp command when no
	// index path is configured) makes the tool report "not available" instead of a transport
	// error.
	LuceneSearch LuceneSearchFunc
	// SemanticSearch, when non-nil, answers loinc_search_terms with mode "semantic" or "hybrid"
	// (meaning-based search through the configured embeddings endpoint). Nil makes those modes
	// report "not available".
	SemanticSearch SemanticSearchFunc
}

func New(options Options) *mcp.Server {
	getStore := options.StoreGetter
	if getStore == nil {
		store := options.Store
		getStore = func() (*loinc.Store, error) {
			if store == nil {
				return nil, errors.New("LOINC database is not loaded")
			}
			return store, nil
		}
	}
	docs := NewDocs(options.DocsDir)
	service := NewService(getStore, docs, options.Terminology, options.LuceneSearch)
	service.semanticSearch = options.SemanticSearch
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "loinc-browser",
		Title:   "LOINC Browser MCP",
		Version: version.Version,
	}, nil)
	registerResources(server, docs, options.OpenAPIJSON)
	registerTools(server, service)
	return server
}

func StreamableHTTPHandler(server *mcp.Server) http.Handler {
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
	})
}

func registerTools(server *mcp.Server, service *Service) {
	notDestructive := false
	closedWorld := false
	tool := func(name, title, description string) *mcp.Tool {
		return &mcp.Tool{
			Name:        name,
			Title:       title,
			Description: description,
			Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, DestructiveHint: &notDestructive, OpenWorldHint: &closedWorld},
		}
	}
	mcp.AddTool(server, tool("loinc_explain_concepts", "Explain LOINC Concepts", "Return a compact explanation for a LOINC concept topic from editable Markdown docs."), service.explainConceptTool)
	mcp.AddTool(server, tool("loinc_search_terms", "Search LOINC Terms", "Search compact LOINC term candidates with context-capped pagination."), service.searchTermsTool)
	mcp.AddTool(server, tool("loinc_get_term", "Get LOINC Term", "Get one selected LOINC term by LOINC number."), service.getTermTool)
	mcp.AddTool(server, tool("loinc_get_term_fit", "Get LOINC Term Fit", "Get compact form-builder suitability metadata for one LOINC term."), service.getTermFitTool)
	mcp.AddTool(server, tool("loinc_get_term_relationships", "Get LOINC Term Relationships", "Get grouped lightweight relationships for one LOINC term."), service.getTermRelationshipsTool)
	mcp.AddTool(server, tool("loinc_search_panels", "Search LOINC Panels", "Search panels and forms with compact candidates."), service.searchPanelsTool)
	mcp.AddTool(server, tool("loinc_get_panel_items", "Get LOINC Panel Items", "List panel or form items in authored sequence."), service.getPanelItemsTool)
	mcp.AddTool(server, tool("loinc_search_answer_lists", "Search LOINC Answer Lists", "Search answer lists by ID, name, or OID."), service.searchAnswerListsTool)
	mcp.AddTool(server, tool("loinc_get_answer_list_answers", "Get LOINC Answer List Answers", "List answer choices for an answer list in sequence."), service.getAnswerListAnswersTool)
	mcp.AddTool(server, tool("loinc_browse_hierarchy", "Browse LOINC Hierarchy", "Browse hierarchy roots or children using occurrence node IDs."), service.browseHierarchyTool)
	mcp.AddTool(server, tool("loinc_get_hierarchy_terms", "Get LOINC Hierarchy Terms", "List compact term candidates under a hierarchy occurrence node."), service.getHierarchyTermsTool)
	mcp.AddTool(server, tool("loinc_search_parts", "Search LOINC Parts", "Search LOINC parts by number, name, display name, or type."), service.searchPartsTool)
	mcp.AddTool(server, tool("loinc_search_groups", "Search LOINC Groups", "Search LOINC groups by ID, name, archetype, or parent group."), service.searchGroupsTool)
	mcp.AddTool(server, tool("loinc_match_names", "Map Local Test Names to LOINC", "Map a local lab test master (1-1000 names) to LOINC term candidates in one call, one word search per name, same list filters as loinc_search_terms. Each result is bucketed: confident (typed LOINC number, a CLCI name match, or a clear top result) needs only a spot check; review means pick among the returned candidates; none means nothing was found for that name. Example: {\"names\":[\"CBC\",\"Fasting glucose\"],\"classType\":\"lab\"}."), service.matchNamesTool)
	lookupCode := tool("loinc_lookup_code", "Lookup LOINC Code", "Look up any LOINC code kind (term, LP part, LL answer list, LA answer, or LG group): display, status, key axis/relation properties, and parents. Example: {\"code\":\"718-7\"}.")
	// Loose object schema: LookupCodeResult.Raw carries the raw FHIR Parameters resource, whose
	// Part []Parameter self-reference the SDK's output-schema reflection cannot express.
	lookupCode.OutputSchema = &jsonschema.Schema{Type: "object"}
	mcp.AddTool(server, lookupCode, service.lookupCodeTool)
	mcp.AddTool(server, tool("loinc_validate_code", "Validate LOINC Code", "Check whether a code is a valid, active LOINC code, optionally checking a display string. Example: {\"code\":\"718-7\",\"display\":\"Hemoglobin [Mass/volume] in Blood\"}."), service.validateCodeTool)
	mcp.AddTool(server, tool("loinc_subsumes", "Check LOINC Subsumption", "Check whether one LOINC/part code subsumes, is subsumed by, or is equivalent to another via the Component Hierarchy by System. Example: {\"codeA\":\"LP14559-6\",\"codeB\":\"718-7\"}."), service.subsumesTool)
	mcp.AddTool(server, tool("loinc_expand_value_set", "Expand LOINC Value Set", "Expand a LOINC ValueSet (named catalogue set, LL answer list, LG group, or implicit LP part-hierarchy set) into its member codes, paginated. Example: {\"url\":\"http://loinc.org/vs/LL1162-8\"}."), service.expandValueSetTool)
	mcp.AddTool(server, tool("loinc_search_value_sets", "Search LOINC Value Sets", "Search served ValueSets by name (answer lists, groups, and named catalogue sets). Example: {\"nameContains\":\"positive\"}."), service.searchValueSetsTool)
	mcp.AddTool(server, tool("loinc_validate_value_set_membership", "Validate LOINC Value Set Membership", "Check whether a code is a member of a given ValueSet. Example: {\"url\":\"http://loinc.org/vs/LL1162-8\",\"code\":\"LA6576-8\"}."), service.validateValueSetMembershipTool)
	mcp.AddTool(server, tool("loinc_translate", "Translate LOINC Code", "Translate a code through a LOINC ConceptMap: find a deprecated LOINC term's replacement, or map a third-party code onto/from LOINC. Example: {\"code\":\"11556-8\"}."), service.translateTool)
	mcp.AddTool(server, tool("loinc_list_concept_maps", "List LOINC Concept Maps", "List the ConceptMaps this server serves (loinc-to-loinc replacement mappings and MAP_TO-derived maps to other code systems). Example: {}."), service.listConceptMapsTool)
	getQuestionnaire := tool("loinc_get_questionnaire", "Get LOINC Panel Questionnaire", "Get a LOINC panel/form as a compact FHIR Questionnaire item tree (linkId, code, text, type, required, capped answer options). Example: {\"loincNum\":\"24357-6\"}.")
	// Loose object schema: QuestionnaireItemCompact.Items self-references, which the SDK's
	// output-schema reflection cannot express (same reason as loinc_lookup_code above).
	getQuestionnaire.OutputSchema = &jsonschema.Schema{Type: "object"}
	mcp.AddTool(server, getQuestionnaire, service.getQuestionnaireTool)
	mcp.AddTool(server, tool("loinc_lucene_search", "Lucene Search Local Index", "Run a Lucene-style query (fielded, boolean, wildcard, range) against the local Bleve search index over loincs, parts, answerlists, or groups. Example: {\"scope\":\"loincs\",\"query\":\"Component:glucose System:bld\"}."), service.luceneSearchTool)
}

func registerResources(server *mcp.Server, docs *Docs, openAPIJSON func() string) {
	addTextResource := func(uri, name, description string) {
		server.AddResource(&mcp.Resource{URI: uri, Name: name, Description: description, MIMEType: "text/markdown"}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			text, err := docs.ReadResource(ctx, uri)
			if err != nil {
				return nil, err
			}
			return textResource(uri, "text/markdown", text.Text), nil
		})
	}
	addTextResource(resourceConcepts, "LOINC Concepts", "Editable Markdown primer for key LOINC concepts.")
	addTextResource(resourceAgentGuide, "LOINC Agent Guide", "Editable Markdown workflow guide for agents.")
	addTextResource(resourceLicense, "LOINC License Note", "Editable Markdown license and data-handling note for agents.")
	addTextResource(resourceAPIGuide, "LOINC API Guide", "Markdown guide for the normalized local LOINC API.")
	server.AddResource(&mcp.Resource{URI: "loinc://openapi", Name: "LOINC OpenAPI", Description: "Live OpenAPI JSON for the local LOINC API.", MIMEType: "application/json"}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		if openAPIJSON == nil {
			return textResource("loinc://openapi", "application/json", "{}"), nil
		}
		return textResource("loinc://openapi", "application/json", openAPIJSON()), nil
	})
}

func textResource(uri, mimeType, text string) *mcp.ReadResourceResult {
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: mimeType, Text: text}}}
}

func (s *Service) explainConceptTool(ctx context.Context, _ *mcp.CallToolRequest, req ConceptRequest) (*mcp.CallToolResult, TextResponse, error) {
	out, err := s.ExplainConcept(ctx, req)
	return nil, out, err
}

func (s *Service) searchTermsTool(ctx context.Context, _ *mcp.CallToolRequest, req SearchTermsRequest) (*mcp.CallToolResult, PageResponse[TermCandidate], error) {
	out, err := s.SearchTerms(ctx, req)
	return nil, out, err
}

func (s *Service) getTermTool(ctx context.Context, _ *mcp.CallToolRequest, req LOINCRequest) (*mcp.CallToolResult, loinc.Term, error) {
	out, err := s.GetTerm(ctx, req)
	return nil, out, err
}

func (s *Service) getTermFitTool(ctx context.Context, _ *mcp.CallToolRequest, req LOINCRequest) (*mcp.CallToolResult, TermFitResponse, error) {
	out, err := s.GetTermFit(ctx, req)
	return nil, out, err
}

func (s *Service) getTermRelationshipsTool(ctx context.Context, _ *mcp.CallToolRequest, req LOINCRequest) (*mcp.CallToolResult, loinc.TermRelationshipGroups, error) {
	out, err := s.GetTermRelationships(ctx, req)
	return nil, out, err
}

func (s *Service) searchPanelsTool(ctx context.Context, _ *mcp.CallToolRequest, req SearchTermsRequest) (*mcp.CallToolResult, PageResponse[TermCandidate], error) {
	out, err := s.SearchPanels(ctx, req)
	return nil, out, err
}

func (s *Service) getPanelItemsTool(ctx context.Context, _ *mcp.CallToolRequest, req LOINCRequest) (*mcp.CallToolResult, PageResponse[loinc.PanelItem], error) {
	out, err := s.GetPanelItems(ctx, req)
	return nil, out, err
}

func (s *Service) searchAnswerListsTool(ctx context.Context, _ *mcp.CallToolRequest, req QueryPageRequest) (*mcp.CallToolResult, PageResponse[loinc.AnswerList], error) {
	out, err := s.SearchAnswerLists(ctx, req)
	return nil, out, err
}

func (s *Service) getAnswerListAnswersTool(ctx context.Context, _ *mcp.CallToolRequest, req AnswerListRequest) (*mcp.CallToolResult, PageResponse[loinc.AnswerListAnswer], error) {
	out, err := s.GetAnswerListAnswers(ctx, req)
	return nil, out, err
}

func (s *Service) browseHierarchyTool(ctx context.Context, _ *mcp.CallToolRequest, req HierarchyRequest) (*mcp.CallToolResult, PageResponse[loinc.HierarchyNode], error) {
	out, err := s.BrowseHierarchy(ctx, req)
	return nil, out, err
}

func (s *Service) getHierarchyTermsTool(ctx context.Context, _ *mcp.CallToolRequest, req HierarchyTermsRequest) (*mcp.CallToolResult, PageResponse[TermCandidate], error) {
	out, err := s.GetHierarchyTerms(ctx, req)
	return nil, out, err
}

func (s *Service) searchPartsTool(ctx context.Context, _ *mcp.CallToolRequest, req QueryPageRequest) (*mcp.CallToolResult, PageResponse[loinc.Part], error) {
	out, err := s.SearchParts(ctx, req)
	return nil, out, err
}

func (s *Service) searchGroupsTool(ctx context.Context, _ *mcp.CallToolRequest, req QueryPageRequest) (*mcp.CallToolResult, PageResponse[loinc.LOINCGroup], error) {
	out, err := s.SearchGroups(ctx, req)
	return nil, out, err
}

func (s *Service) matchNamesTool(ctx context.Context, _ *mcp.CallToolRequest, req MatchNamesRequest) (*mcp.CallToolResult, MatchNamesResult, error) {
	out, err := s.MatchNames(ctx, req)
	return nil, out, err
}
