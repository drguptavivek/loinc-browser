package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"loinc-browser/internal/loinc"
	loincmcp "loinc-browser/internal/mcpserver"
	"loinc-browser/internal/version"
)

type Options struct {
	Store        *loinc.Store
	Assets       http.FileSystem
	DBPath       string
	UploadDir    string
	CacheEntries int
	EnableMCP    bool
	MCPPath      string
	DocsDir      string
}

func New(options Options) http.Handler {
	app := &app{
		store:        options.Store,
		assets:       options.Assets,
		dbPath:       options.DBPath,
		uploadDir:    options.UploadDir,
		cacheEntries: options.CacheEntries,
		docsDir:      options.DocsDir,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", app.health)
	mux.HandleFunc("GET /api/version", app.version)
	mux.HandleFunc("GET /api/search", app.search)
	mux.HandleFunc("GET /api/terms/{loincNum}", app.term)
	mux.HandleFunc("GET /api/terms/{loincNum}/relationships", app.termRelationships)
	mux.HandleFunc("GET /api/facets", app.facets)
	mux.HandleFunc("GET /api/source-organizations", app.sourceOrganizations)
	mux.HandleFunc("GET /api/accessories", app.accessories)
	mux.HandleFunc("GET /api/hierarchy", app.hierarchy)
	mux.HandleFunc("GET /api/cache", app.cacheStats)
	mux.HandleFunc("POST /api/import/upload", app.uploadImport)
	mux.HandleFunc("GET /api/v1/health", app.health)
	mux.HandleFunc("GET /api/v1/version", app.version)
	mux.HandleFunc("GET /api/v1/terms/search", app.v1TermsSearch)
	mux.HandleFunc("GET /api/v1/terms/top", app.v1TermsTop)
	mux.HandleFunc("GET /api/v1/terms/{loincNum}", app.v1Term)
	mux.HandleFunc("GET /api/v1/terms/{loincNum}/fit", app.v1TermFit)
	mux.HandleFunc("GET /api/v1/terms/{loincNum}/relationships", app.v1TermRelationships)
	mux.HandleFunc("GET /api/v1/terms/{loincNum}/answer-lists", app.v1TermAnswerLists)
	mux.HandleFunc("GET /api/v1/terms/{loincNum}/panel-memberships", app.v1TermPanelMemberships)
	mux.HandleFunc("GET /api/v1/terms/{loincNum}/copyright", app.v1TermCopyright)
	mux.HandleFunc("GET /api/v1/hierarchy/roots", app.v1HierarchyRoots)
	mux.HandleFunc("GET /api/v1/hierarchy/nodes/{nodeId}", app.v1HierarchyNode)
	mux.HandleFunc("GET /api/v1/hierarchy/nodes/{nodeId}/parents", app.v1HierarchyParents)
	mux.HandleFunc("GET /api/v1/hierarchy/nodes/{nodeId}/children", app.v1HierarchyChildren)
	mux.HandleFunc("GET /api/v1/hierarchy/nodes/{nodeId}/terms", app.v1HierarchyTerms)
	mux.HandleFunc("GET /api/v1/panels/search", app.v1PanelsSearch)
	mux.HandleFunc("GET /api/v1/panels/{loincNum}", app.v1Panel)
	mux.HandleFunc("GET /api/v1/panels/{loincNum}/items", app.v1PanelItems)
	mux.HandleFunc("GET /api/v1/answer-lists/search", app.v1AnswerListsSearch)
	mux.HandleFunc("GET /api/v1/answer-lists/{answerListId}", app.v1AnswerList)
	mux.HandleFunc("GET /api/v1/answer-lists/{answerListId}/answers", app.v1AnswerListAnswers)
	mux.HandleFunc("GET /api/v1/answer-lists/{answerListId}/terms", app.v1AnswerListTerms)
	mux.HandleFunc("GET /api/v1/parts/search", app.v1PartsSearch)
	mux.HandleFunc("GET /api/v1/parts/{partNumber}", app.v1Part)
	mux.HandleFunc("GET /api/v1/parts/{partNumber}/terms", app.v1PartTerms)
	mux.HandleFunc("GET /api/v1/groups/search", app.v1GroupsSearch)
	mux.HandleFunc("GET /api/v1/groups/{groupId}", app.v1Group)
	mux.HandleFunc("GET /api/v1/groups/{groupId}/terms", app.v1GroupTerms)
	mux.HandleFunc("GET /api/v1/source-organizations", app.sourceOrganizations)
	mux.HandleFunc("GET /api/v1/source-organizations/{id}", app.v1SourceOrganization)
	mux.HandleFunc("GET /api/v1/accessories", app.accessories)
	mux.HandleFunc("GET /api/docs", app.swaggerDocs)
	mux.HandleFunc("GET /openapi.json", app.openapi)
	mux.HandleFunc("GET /docs/mcp", app.markdownDoc("MCP.md", "docs"))
	mux.HandleFunc("GET /docs/concepts", app.markdownDoc("LOINC_CONCEPTS.md", "agent"))
	mux.HandleFunc("GET /docs/agent-guide", app.markdownDoc("LOINC_AGENT_GUIDE.md", "agent"))
	if options.EnableMCP {
		mcpServer := loincmcp.New(loincmcp.Options{
			Store:       options.Store,
			DocsDir:     options.DocsDir,
			OpenAPIJSON: OpenAPIJSON,
		})
		mux.Handle(normalizeMCPPath(options.MCPPath), loincmcp.StreamableHTTPHandler(mcpServer))
	}
	mux.HandleFunc("/", app.frontend)
	return mux
}

func normalizeMCPPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/mcp"
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

type app struct {
	mu           sync.RWMutex
	store        *loinc.Store
	assets       http.FileSystem
	dbPath       string
	uploadDir    string
	cacheEntries int
	docsDir      string
}

func (a *app) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *app) version(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, version.Get())
}

func (a *app) search(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	query := r.URL.Query()
	params := loinc.SearchParams{
		Query:          query.Get("q"),
		Class:          query.Get("class"),
		Statuses:       queryValues(query, "status"),
		System:         query.Get("system"),
		TimeAspects:    queryValues(query, "timeAspect"),
		Scales:         queryValues(query, "scale"),
		Methods:        queryValues(query, "method"),
		Property:       query.Get("property"),
		OrderObsValues: queryValues(query, "orderObs"),
		RankedOnly:     parseBool(query.Get("rankedOnly")),
		HierarchyCode:  query.Get("hierarchy"),
		Limit:          parseInt(query.Get("limit"), 25),
		Offset:         parseInt(query.Get("offset"), 0),
	}
	response, err := store.Search(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *app) term(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	var term loinc.Term
	term, err = store.Term(r.Context(), r.PathValue("loincNum"))
	if err != nil {
		if errors.Is(err, loinc.ErrNotFound) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, term)
}

func (a *app) termRelationships(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	graph, err := store.TermRelationships(r.Context(), r.PathValue("loincNum"))
	if err != nil {
		if errors.Is(err, loinc.ErrNotFound) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, graph)
}

func (a *app) facets(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	facets, err := store.Facets(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, facets)
}

func (a *app) cacheStats(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	writeJSON(w, http.StatusOK, store.CacheStats())
}

func (a *app) sourceOrganizations(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	items, err := store.SourceOrganizations(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *app) accessories(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	query := r.URL.Query()
	response, err := store.BrowseAccessories(r.Context(), loinc.AccessoryBrowseParams{
		Kind:   query.Get("kind"),
		Query:  query.Get("q"),
		Limit:  parseInt(query.Get("limit"), 50),
		Offset: parseInt(query.Get("offset"), 0),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *app) hierarchy(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	query := r.URL.Query()
	response, err := store.HierarchyChildren(r.Context(), query.Get("parent"), query.Get("q"), query.Get("navOnly") == "true" || query.Get("navOnly") == "1")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *app) v1TermsSearch(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	response, err := store.Search(r.Context(), termListParamsFromRequest(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *app) v1TermsTop(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	params := termListParamsFromRequest(r)
	params.Query = ""
	if strings.TrimSpace(params.Sort) == "" || params.Sort == "relevance" {
		params.Sort = "usage"
	}
	response, err := store.Search(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *app) v1Term(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	term, err := store.Term(r.Context(), r.PathValue("loincNum"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, term)
}

func (a *app) v1TermFit(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	fit, err := store.TermFit(r.Context(), r.PathValue("loincNum"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, fit)
}

func (a *app) v1TermRelationships(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	relationships, err := store.TermRelationshipGroups(r.Context(), r.PathValue("loincNum"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, relationships)
}

func (a *app) v1TermAnswerLists(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	response, err := store.TermAnswerLists(r.Context(), r.PathValue("loincNum"), parseInt(r.URL.Query().Get("limit"), 25), parseInt(r.URL.Query().Get("offset"), 0))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *app) v1TermPanelMemberships(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	relationships, err := store.TermRelationshipGroups(r.Context(), r.PathValue("loincNum"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	results := relationships.PanelMemberships
	writeJSON(w, http.StatusOK, loinc.Page[loinc.TermAccessory]{
		Results: results,
		Total:   len(results),
		Limit:   len(results),
		Offset:  0,
		HasMore: false,
	})
}

func (a *app) v1TermCopyright(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	item, err := store.TermCopyright(r.Context(), r.PathValue("loincNum"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *app) v1HierarchyRoots(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	response, err := store.HierarchyChildren(r.Context(), "", r.URL.Query().Get("q"), true)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *app) v1HierarchyNode(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	node, err := store.HierarchyNode(r.Context(), r.PathValue("nodeId"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (a *app) v1HierarchyParents(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	nodes, err := store.HierarchyParents(r.Context(), r.PathValue("nodeId"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, loinc.Page[loinc.HierarchyNode]{Results: nodes, Total: len(nodes), Limit: len(nodes), Offset: 0, HasMore: false})
}

func (a *app) v1HierarchyChildren(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	response, err := store.HierarchyChildren(r.Context(), r.PathValue("nodeId"), r.URL.Query().Get("q"), true)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *app) v1HierarchyTerms(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	params := termListParamsFromRequest(r)
	params.HierarchyNodeID = r.PathValue("nodeId")
	response, err := store.Search(r.Context(), params)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *app) v1PanelsSearch(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	response, err := store.SearchPanels(r.Context(), termListParamsFromRequest(r))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *app) v1Panel(w http.ResponseWriter, r *http.Request) {
	a.v1Term(w, r)
}

func (a *app) v1PanelItems(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	response, err := store.PanelItems(r.Context(), r.PathValue("loincNum"), parseInt(r.URL.Query().Get("limit"), 100), parseInt(r.URL.Query().Get("offset"), 0))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *app) v1AnswerListsSearch(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	response, err := store.SearchAnswerLists(r.Context(), r.URL.Query().Get("q"), parseInt(r.URL.Query().Get("limit"), 25), parseInt(r.URL.Query().Get("offset"), 0))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *app) v1AnswerList(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	item, err := store.AnswerList(r.Context(), r.PathValue("answerListId"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *app) v1AnswerListAnswers(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	response, err := store.AnswerListAnswers(r.Context(), r.PathValue("answerListId"), parseInt(r.URL.Query().Get("limit"), 100), parseInt(r.URL.Query().Get("offset"), 0))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *app) v1AnswerListTerms(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	params := termListParamsFromRequest(r)
	params.AnswerListID = r.PathValue("answerListId")
	response, err := store.Search(r.Context(), params)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *app) v1PartsSearch(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	response, err := store.SearchParts(r.Context(), r.URL.Query().Get("q"), parseInt(r.URL.Query().Get("limit"), 25), parseInt(r.URL.Query().Get("offset"), 0))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *app) v1Part(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	item, err := store.Part(r.Context(), r.PathValue("partNumber"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *app) v1PartTerms(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	params := termListParamsFromRequest(r)
	params.PartNumber = r.PathValue("partNumber")
	params.PartLinkSet = r.URL.Query().Get("linkSet")
	response, err := store.Search(r.Context(), params)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *app) v1GroupsSearch(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	response, err := store.SearchGroups(r.Context(), r.URL.Query().Get("q"), parseInt(r.URL.Query().Get("limit"), 25), parseInt(r.URL.Query().Get("offset"), 0))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *app) v1Group(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	item, err := store.Group(r.Context(), r.PathValue("groupId"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *app) v1GroupTerms(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	params := termListParamsFromRequest(r)
	params.GroupID = r.PathValue("groupId")
	response, err := store.Search(r.Context(), params)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *app) v1SourceOrganization(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	item, err := store.SourceOrganization(r.Context(), r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *app) openapi(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, openAPISpec)
}

func OpenAPIJSON() string {
	data, err := json.MarshalIndent(openAPISpec, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(data)
}

func (a *app) swaggerDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>LOINC Browser API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
  <style>
    body { margin: 0; background: #f8fafc; }
    .swagger-ui .topbar { display: none; }
    .api-title { padding: 16px 24px; border-bottom: 1px solid #e4e4e7; background: white; font: 600 18px system-ui, sans-serif; }
  </style>
</head>
<body>
  <div class="api-title">LOINC Browser API</div>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({
      url: "/openapi.json",
      dom_id: "#swagger-ui",
      deepLinking: true,
      presets: [SwaggerUIBundle.presets.apis],
      layout: "BaseLayout"
    });
  </script>
</body>
</html>`)
}

func (a *app) markdownDoc(name string, scope string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := ""
		if scope == "agent" {
			path = filepath.Join(a.agentDocsDir(), name)
		} else {
			path = filepath.Join(filepath.Dir(a.agentDocsDir()), name)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			writeError(w, http.StatusNotFound, fmt.Errorf("documentation file not found: %s", path))
			return
		}
		w.Header().Set("content-type", "text/markdown; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}
}

func (a *app) agentDocsDir() string {
	if strings.TrimSpace(a.docsDir) != "" {
		return a.docsDir
	}
	return filepath.Join(".", "docs", "agent")
}

func (a *app) currentStore() (*loinc.Store, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.store == nil {
		return nil, errors.New("LOINC database is not loaded")
	}
	return a.store, nil
}

func (a *app) swapStore(store *loinc.Store) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	old := a.store
	a.store = store
	if old != nil {
		return old.Close()
	}
	return nil
}

func (a *app) frontend(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	if a.assets == nil {
		w.Header().Set("content-type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<html><body><h1>LOINC Browser</h1><p>Frontend assets are not built yet.</p></body></html>`)
		return
	}
	http.FileServer(a.assets).ServeHTTP(w, r)
}

func parseInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func parseBool(raw string) bool {
	raw = strings.TrimSpace(strings.ToLower(raw))
	return raw == "1" || raw == "true" || raw == "yes"
}

func queryValues(values map[string][]string, key string) []string {
	raw := values[key]
	filtered := make([]string, 0, len(raw))
	for _, value := range raw {
		value = strings.TrimSpace(value)
		if value != "" {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func termListParamsFromRequest(r *http.Request) loinc.SearchParams {
	query := r.URL.Query()
	return loinc.SearchParams{
		Query:           query.Get("q"),
		Class:           query.Get("class"),
		Statuses:        queryValues(query, "status"),
		UsageType:       query.Get("usageType"),
		RankMode:        query.Get("rankMode"),
		Sort:            query.Get("sort"),
		System:          query.Get("system"),
		TimeAspects:     queryValues(query, "timeAspect"),
		Scales:          queryValues(query, "scale"),
		Methods:         queryValues(query, "method"),
		Property:        query.Get("property"),
		OrderObsValues:  queryValues(query, "orderObs"),
		RankedOnly:      parseBool(query.Get("rankedOnly")),
		HierarchyNodeID: firstNonEmpty(query.Get("hierarchyNodeId"), query.Get("hierarchy")),
		Limit:           parseInt(query.Get("limit"), 25),
		Offset:          parseInt(query.Get("offset"), 0),
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, loinc.ErrNotFound) {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeError(w, http.StatusInternalServerError, err)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{
		"error": err.Error(),
	})
}
