package mcpserver

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"loinc-browser/internal/loinc"
)

type Options struct {
	Store       *loinc.Store
	DocsDir     string
	OpenAPIJSON func() string
}

func New(options Options) *mcp.Server {
	docs := NewDocs(options.DocsDir)
	service := NewService(options.Store, docs)
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "loinc-browser",
		Title:   "LOINC Browser MCP",
		Version: "0.1.0",
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
