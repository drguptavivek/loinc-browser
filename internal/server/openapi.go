package server

var openAPISpec = map[string]any{
	"openapi": "3.1.0",
	"info": map[string]any{
		"title":       "LOINC Browser API",
		"version":     "0.92",
		"description": "Local API for searching and browsing an imported licensed LOINC release.",
	},
	"servers": []map[string]any{
		{
			"url":         "/",
			"description": "This server (relative to the host serving /openapi.json)",
		},
	},
	"paths": map[string]any{
		"/api/health": map[string]any{
			"get": map[string]any{
				"summary": "Check API health",
				"responses": map[string]any{
					"200": response("Health response", ref("HealthResponse")),
				},
			},
		},
		"/api/version": map[string]any{
			"get": map[string]any{
				"summary": "Get application version information",
				"responses": map[string]any{
					"200": response("Version response", ref("VersionResponse")),
				},
			},
		},
		"/api/v1/health": map[string]any{
			"get": map[string]any{
				"summary": "Check v1 API health",
				"responses": map[string]any{
					"200": response("Health response", ref("HealthResponse")),
				},
			},
		},
		"/api/v1/version": map[string]any{
			"get": map[string]any{
				"summary": "Get application version information",
				"responses": map[string]any{
					"200": response("Version response", ref("VersionResponse")),
				},
			},
		},
		"/api/v1/terms/search": map[string]any{
			"get": map[string]any{
				"summary":     "Search ranked LOINC terms for EMR form fields",
				"description": "Excludes deprecated terms by default. Use status=DEPRECATED to browse them, or status=* to include every status. When no term matches every word, drops as few words as possible and returns relaxed=true with droppedWords.",
				"parameters":  commonTermListParameters(),
				"responses": map[string]any{
					"200": response("Term search results", ref("SearchResponse")),
				},
			},
		},
		"/api/v1/terms/top": map[string]any{
			"get": map[string]any{
				"summary":     "List top ranked LOINC terms",
				"description": "Uses shared term filters and usage ranking. Defaults to active terms.",
				"parameters":  commonTermListParameters(),
				"responses": map[string]any{
					"200": response("Top term results", ref("SearchResponse")),
				},
			},
		},
		"/api/v1/terms/{loincNum}": map[string]any{
			"get": map[string]any{
				"summary": "Get one LOINC term detail without nested relationships",
				"parameters": []map[string]any{
					pathParam("loincNum", "LOINC number, for example 14749-6"),
				},
				"responses": map[string]any{
					"200": response("LOINC term detail", ref("Term")),
					"404": response("Term not found", ref("ErrorResponse")),
				},
			},
		},
		"/api/v1/terms/{loincNum}/fit": map[string]any{
			"get": map[string]any{
				"summary": "Summarize whether a term is suitable for form-builder use",
				"parameters": []map[string]any{
					pathParam("loincNum", "LOINC number"),
				},
				"responses": map[string]any{
					"200": response("Term fit metadata", ref("TermFit")),
					"404": response("Term not found", ref("ErrorResponse")),
				},
			},
		},
		"/api/v1/terms/{loincNum}/relationships": map[string]any{
			"get": map[string]any{
				"summary": "Get grouped lightweight relationships for one term",
				"parameters": []map[string]any{
					pathParam("loincNum", "LOINC number"),
				},
				"responses": map[string]any{
					"200": response("Grouped relationships", ref("TermRelationshipGroups")),
					"404": response("Term not found", ref("ErrorResponse")),
				},
			},
		},
		"/api/v1/terms/{loincNum}/answer-lists": map[string]any{
			"get": map[string]any{
				"summary": "List answer lists linked to one term",
				"parameters": []map[string]any{
					pathParam("loincNum", "LOINC number"),
					intQueryParam("limit", "Maximum rows to return", 25),
					intQueryParam("offset", "Result offset", 0),
				},
				"responses": map[string]any{
					"200": response("Linked answer lists", ref("AnswerListPage")),
					"404": response("Term not found", ref("ErrorResponse")),
				},
			},
		},
		"/api/v1/terms/{loincNum}/panel-memberships": map[string]any{
			"get": map[string]any{
				"summary": "List panels that contain one term",
				"parameters": []map[string]any{
					pathParam("loincNum", "LOINC number"),
					intQueryParam("limit", "Maximum rows to return", 25),
					intQueryParam("offset", "Result offset", 0),
				},
				"responses": map[string]any{
					"200": response("Panel memberships", ref("TermAccessoryPage")),
					"404": response("Term not found", ref("ErrorResponse")),
				},
			},
		},
		"/api/v1/hierarchy/roots": map[string]any{
			"get": map[string]any{
				"summary": "List hierarchy root nodes",
				"responses": map[string]any{
					"200": response("Hierarchy roots", ref("HierarchyChildrenResponse")),
				},
			},
		},
		"/api/v1/hierarchy/nodes/{nodeId}": map[string]any{
			"get": map[string]any{
				"summary":    "Get one hierarchy occurrence node by nodeId",
				"parameters": []map[string]any{pathParam("nodeId", "Hierarchy occurrence node id")},
				"responses": map[string]any{
					"200": response("Hierarchy node", ref("HierarchyNode")),
					"404": response("Node not found", ref("ErrorResponse")),
				},
			},
		},
		"/api/v1/hierarchy/nodes/{nodeId}/parents": map[string]any{
			"get": map[string]any{
				"summary":    "List parent hierarchy nodes",
				"parameters": []map[string]any{pathParam("nodeId", "Hierarchy occurrence node id")},
				"responses": map[string]any{
					"200": response("Hierarchy parents", ref("HierarchyNodePage")),
					"404": response("Node not found", ref("ErrorResponse")),
				},
			},
		},
		"/api/v1/hierarchy/nodes/{nodeId}/children": map[string]any{
			"get": map[string]any{
				"summary":    "List child hierarchy nodes",
				"parameters": []map[string]any{pathParam("nodeId", "Hierarchy occurrence node id")},
				"responses": map[string]any{
					"200": response("Hierarchy children", ref("HierarchyChildrenResponse")),
				},
			},
		},
		"/api/v1/hierarchy/nodes/{nodeId}/terms": map[string]any{
			"get": map[string]any{
				"summary":    "List terms below a hierarchy node",
				"parameters": termListParameters(pathParam("nodeId", "Hierarchy occurrence node id")),
				"responses": map[string]any{
					"200": response("Hierarchy scoped term results", ref("SearchResponse")),
				},
			},
		},
		"/api/v1/panels/search": v1TermListPath("Search panels and forms"),
		"/api/v1/panels/{loincNum}": map[string]any{
			"get": map[string]any{
				"summary":    "Get panel term detail",
				"parameters": []map[string]any{pathParam("loincNum", "Panel LOINC number")},
				"responses":  map[string]any{"200": response("Panel detail", ref("Term")), "404": response("Panel not found", ref("ErrorResponse"))},
			},
		},
		"/api/v1/panels/{loincNum}/items": map[string]any{
			"get": map[string]any{
				"summary":    "List panel/questionnaire items in authored sequence",
				"parameters": []map[string]any{pathParam("loincNum", "Panel LOINC number"), intQueryParam("limit", "Maximum rows to return", 100), intQueryParam("offset", "Result offset", 0)},
				"responses":  map[string]any{"200": response("Panel items", ref("PanelItemPage"))},
			},
		},
		"/api/v1/answer-lists/search": map[string]any{
			"get": map[string]any{
				"summary":    "Search answer lists",
				"parameters": []map[string]any{queryParam("q", "Answer list id, name, or OID"), intQueryParam("limit", "Maximum rows to return", 25), intQueryParam("offset", "Result offset", 0)},
				"responses":  map[string]any{"200": response("Answer lists", ref("AnswerListPage"))},
			},
		},
		"/api/v1/answer-lists/{answerListId}": map[string]any{
			"get": map[string]any{
				"summary":    "Get answer list detail",
				"parameters": []map[string]any{pathParam("answerListId", "Answer list id")},
				"responses":  map[string]any{"200": response("Answer list detail", ref("AnswerList")), "404": response("Answer list not found", ref("ErrorResponse"))},
			},
		},
		"/api/v1/answer-lists/{answerListId}/answers": map[string]any{
			"get": map[string]any{
				"summary":    "List coded answer choices",
				"parameters": []map[string]any{pathParam("answerListId", "Answer list id"), intQueryParam("limit", "Maximum rows to return", 100), intQueryParam("offset", "Result offset", 0)},
				"responses":  map[string]any{"200": response("Answer choices", ref("AnswerListAnswerPage"))},
			},
		},
		"/api/v1/answer-lists/{answerListId}/terms": v1TermListPath("List terms linked to an answer list", pathParam("answerListId", "Answer list id")),
		"/api/v1/parts/search": map[string]any{
			"get": map[string]any{
				"summary":    "Search LOINC parts",
				"parameters": []map[string]any{queryParam("q", "Part number, name, display name, or type"), intQueryParam("limit", "Maximum rows to return", 25), intQueryParam("offset", "Result offset", 0)},
				"responses":  map[string]any{"200": response("Parts", ref("PartPage"))},
			},
		},
		"/api/v1/parts/{partNumber}": map[string]any{
			"get": map[string]any{
				"summary":    "Get part detail",
				"parameters": []map[string]any{pathParam("partNumber", "Part number")},
				"responses":  map[string]any{"200": response("Part detail", ref("Part")), "404": response("Part not found", ref("ErrorResponse"))},
			},
		},
		"/api/v1/parts/{partNumber}/terms": v1TermListPath("List terms linked to a part", pathParam("partNumber", "Part number"), queryParam("linkSet", "Optional part link-set filter")),
		"/api/v1/groups/search": map[string]any{
			"get": map[string]any{
				"summary":    "Search LOINC groups",
				"parameters": []map[string]any{queryParam("q", "Group id, name, archetype, or parent group"), intQueryParam("limit", "Maximum rows to return", 25), intQueryParam("offset", "Result offset", 0)},
				"responses":  map[string]any{"200": response("Groups", ref("GroupPage"))},
			},
		},
		"/api/v1/groups/{groupId}": map[string]any{
			"get": map[string]any{
				"summary":    "Get group detail",
				"parameters": []map[string]any{pathParam("groupId", "Group id")},
				"responses":  map[string]any{"200": response("Group detail", ref("LOINCGroup")), "404": response("Group not found", ref("ErrorResponse"))},
			},
		},
		"/api/v1/groups/{groupId}/terms": v1TermListPath("List terms linked to a group", pathParam("groupId", "Group id")),
		"/api/v1/source-organizations": map[string]any{
			"get": map[string]any{
				"summary":   "List imported source organizations and copyright metadata",
				"responses": map[string]any{"200": response("Source organizations", map[string]any{"type": "array", "items": ref("SourceOrganization")})},
			},
		},
		"/api/v1/source-organizations/{id}": map[string]any{
			"get": map[string]any{
				"summary":    "Get source organization detail",
				"parameters": []map[string]any{pathParam("id", "Source organization id")},
				"responses":  map[string]any{"200": response("Source organization", ref("SourceOrganization")), "404": response("Source organization not found", ref("ErrorResponse"))},
			},
		},
		"/api/v1/terms/{loincNum}/copyright": map[string]any{
			"get": map[string]any{
				"summary":    "Get term copyright/source metadata state",
				"parameters": []map[string]any{pathParam("loincNum", "LOINC number")},
				"responses":  map[string]any{"200": response("Term copyright metadata", ref("TermCopyright")), "404": response("Term not found", ref("ErrorResponse"))},
			},
		},
		"/api/v1/accessories": map[string]any{
			"get": map[string]any{
				"summary": "Browse imported accessory records",
				"parameters": []map[string]any{
					queryParam("kind", "Accessory kind filter such as part, answer-list, panel, group, or hierarchy"),
					queryParam("q", "Accessory text query"),
					intQueryParam("limit", "Maximum rows to return", 50),
					intQueryParam("offset", "Result offset", 0),
				},
				"responses": map[string]any{
					"200": response("Accessory records", ref("AccessoryBrowseResponse")),
				},
			},
		},
		"/api/v1/local-search/status": map[string]any{
			"get": map[string]any{
				"summary":     "Check local Lucene-style search index status",
				"description": "Reports whether the embedded Bleve local search index exists, can be opened, and has indexed documents.",
				"responses": map[string]any{
					"200": response("Local search index status", ref("LocalSearchStatus")),
				},
			},
		},
		"/api/v1/local-search/rebuild": map[string]any{
			"post": map[string]any{
				"summary":     "Rebuild local Lucene-style search index",
				"description": "Deletes and rebuilds the generated Bleve index from the canonical normalized SQLite database.",
				"responses": map[string]any{
					"200": response("Local search index status", ref("LocalSearchStatus")),
					"503": response("Local SQLite database unavailable", ref("ErrorResponse")),
				},
			},
		},
		"/api/v1/local-search/query": map[string]any{
			"post": map[string]any{
				"summary":     "Run a basic local Lucene-style search",
				"description": "Searches the generated Bleve index for one scope and hydrates typed results from the canonical SQLite database.",
				"requestBody": map[string]any{
					"required": true,
					"content": map[string]any{
						"application/json": map[string]any{"schema": ref("LocalSearchRequest")},
					},
				},
				"responses": map[string]any{
					"200": response("Local search results", ref("LocalSearchResponse")),
					"400": response("Invalid local search query", ref("ErrorResponse")),
					"503": response("Local search index unavailable", ref("ErrorResponse")),
				},
			},
		},
		"/fhir/metadata": map[string]any{
			"get": map[string]any{
				"tags":        []string{"FHIR"},
				"summary":     "FHIR CapabilityStatement (or TerminologyCapabilities with ?mode=terminology)",
				"description": "Local, wire-compatible clone of https://fhir.loinc.org/metadata (plan §4.1). Serving paths never call fhir.loinc.org.",
				"parameters": append([]map[string]any{
					queryParamEx("mode", "Set to \"terminology\" for the TerminologyCapabilities shape instead of CapabilityStatement", "terminology"),
				}, summaryResultParams()...),
				"responses": map[string]any{
					"200": fhirResponse("CapabilityStatement or TerminologyCapabilities, see docs/exemplars/fhir.loinc.org/metadata*.json"),
					"406": fhirResponse("OperationOutcome: XML is not supported"),
				},
			},
		},
		"/fhir/CodeSystem": map[string]any{
			"get": map[string]any{
				"tags":        []string{"FHIR"},
				"summary":     "Search the LOINC CodeSystem",
				"description": "Local clone of https://fhir.loinc.org/CodeSystem?url=... (plan §4.2). The `http://loinc.org` CodeSystem covers terms, LP parts, LL answer lists, LA answers, and LG groups.",
				"parameters": append([]map[string]any{
					queryParamEx("url", "Canonical CodeSystem URL", "http://loinc.org"),
					queryParam("version", "LOINC version, for example 2.82"),
				}, summaryResultParams()...),
				"responses": map[string]any{
					"200": fhirResponse("searchset Bundle containing the CodeSystem resource, see docs/exemplars/fhir.loinc.org/codesystem-search-url.json"),
				},
			},
		},
		"/fhir/CodeSystem/{id}": map[string]any{
			"get": map[string]any{
				"tags":        []string{"FHIR"},
				"summary":     "Read the LOINC CodeSystem by id",
				"description": "`loinc` and `loinc-2.82` both resolve (plan §4.2).",
				"parameters":  append([]map[string]any{pathParamEx("id", "CodeSystem id", "loinc-2.82")}, summaryResultParams()...),
				"responses": map[string]any{
					"200": fhirResponse("CodeSystem resource"),
					"404": fhirResponse("OperationOutcome: not-found"),
				},
			},
		},
		"/fhir/CodeSystem/$lookup":             codeSystemOperationPath("$lookup", "CodeSystem $lookup", "Resolves a term, LP part, LL answer list, LA answer, or LG group (plan §4.3).", codeSystemLookupParams()),
		"/fhir/CodeSystem/{id}/$lookup":        codeSystemOperationPathWithID("$lookup", "CodeSystem $lookup (instance form)", "Same as /fhir/CodeSystem/$lookup, scoped to a CodeSystem instance.", codeSystemLookupParams()),
		"/fhir/CodeSystem/$validate-code":      codeSystemOperationPath("$validate-code", "CodeSystem $validate-code", "`result` is a valueString \"true\"/\"false\", matching upstream (plan §4.4).", codeSystemValidateCodeParams()),
		"/fhir/CodeSystem/{id}/$validate-code": codeSystemOperationPathWithID("$validate-code", "CodeSystem $validate-code (instance form)", "Same as /fhir/CodeSystem/$validate-code, scoped to a CodeSystem instance.", codeSystemValidateCodeParams()),
		"/fhir/CodeSystem/$subsumes":           codeSystemOperationPath("$subsumes", "CodeSystem $subsumes", "Walks the Component Hierarchy by System (plan §4.5). LP384441-4 subsumes 30064-0.", codeSystemSubsumesParams()),
		"/fhir/CodeSystem/{id}/$subsumes":      codeSystemOperationPathWithID("$subsumes", "CodeSystem $subsumes (instance form)", "Same as /fhir/CodeSystem/$subsumes, scoped to a CodeSystem instance.", codeSystemSubsumesParams()),
		"/fhir/ValueSet": map[string]any{
			"get": map[string]any{
				"tags":        []string{"FHIR"},
				"summary":     "Search the ValueSet catalogue",
				"description": "Answer lists, groups, and named/implicit value sets (plan §4.6). Not all upstream sets are served; unserved ones 404 (plan §4.6.2).",
				"parameters": append([]map[string]any{
					queryParamEx("url", "Canonical ValueSet URL", "http://loinc.org/vs/LL1162-8"),
					queryParam("name", "Name prefix match"),
					queryParam("name:in", "Name substring match (upstream alias for contains)"),
					queryParam("name:contains", "Name substring match"),
					queryParam("_id", "ValueSet id"),
					intQueryParam("_count", "Page size (default 20, max 100)", 20),
					intQueryParam("_offset", "Result offset", 0),
				}, summaryResultParams()...),
				"responses": map[string]any{
					"200": fhirResponse("searchset Bundle of ValueSet resources"),
					"400": fhirResponse("OperationOutcome: invalid, for a bad _summary value or _summary combined with _elements"),
				},
			},
		},
		"/fhir/ValueSet/{id}": map[string]any{
			"get": map[string]any{
				"tags":        []string{"FHIR"},
				"summary":     "Read a ValueSet definition",
				"description": "Embeds compose.include.concept when the member count is <= 10,000, otherwise the intensional filter form (plan §4.6.3).",
				"parameters":  append([]map[string]any{pathParamEx("id", "ValueSet id, e.g. an LL/LG code or named slug", "LL1162-8")}, summaryResultParams()...),
				"responses": map[string]any{
					"200": fhirResponse("ValueSet resource"),
					"404": fhirResponse("OperationOutcome: not-found"),
				},
			},
		},
		"/fhir/ValueSet/$expand":             valueSetOperationPath("$expand", "ValueSet $expand", "Flattens a ValueSet to its member concepts, incl. POSTed inline compose (plan §4.7).", valueSetExpandParams()),
		"/fhir/ValueSet/{id}/$expand":        valueSetOperationPathWithID("$expand", "ValueSet $expand (instance form)", "Same as /fhir/ValueSet/$expand, scoped to a ValueSet instance.", valueSetExpandParams()),
		"/fhir/ValueSet/$validate-code":      valueSetOperationPath("$validate-code", "ValueSet $validate-code", "`result` is a valueBoolean here, unlike CodeSystem $validate-code (plan §4.8).", valueSetValidateCodeParams()),
		"/fhir/ValueSet/{id}/$validate-code": valueSetOperationPathWithID("$validate-code", "ValueSet $validate-code (instance form)", "Same as /fhir/ValueSet/$validate-code, scoped to a ValueSet instance.", valueSetValidateCodeParams()),
		"/fhir/ConceptMap": map[string]any{
			"get": map[string]any{
				"tags":        []string{"FHIR"},
				"summary":     "Search the ConceptMap catalogue",
				"description": "LOINC-to-external-system maps such as IEEE, RadLex, RxNorm, SNOMED CT, and the local loinc-map-to replacement map (plan §4.9). Search results never embed group.",
				"parameters": append([]map[string]any{
					queryParamEx("url", "Canonical ConceptMap URL", "http://loinc.org/cm/loinc-to-ieee-11073-10101"),
					queryParam("source-system", "Source system URI filter"),
					queryParam("target-system", "Target system URI filter"),
					intQueryParam("_count", "Page size", 20),
					intQueryParam("_offset", "Result offset", 0),
				}, summaryResultParams()...),
				"responses": map[string]any{
					"200": fhirResponse("searchset Bundle of ConceptMap resources (without group)"),
				},
			},
		},
		"/fhir/ConceptMap/{id}": map[string]any{
			"get": map[string]any{
				"tags":        []string{"FHIR"},
				"summary":     "Read a ConceptMap definition",
				"description": "Embeds group[source,target,element[...]], capped at 1000 elements; beyond the cap, use $translate (plan §4.9).",
				"parameters":  append([]map[string]any{pathParamEx("id", "ConceptMap id", "loinc-to-ieee-11073-10101")}, summaryResultParams()...),
				"responses": map[string]any{
					"200": fhirResponse("ConceptMap resource"),
					"404": fhirResponse("OperationOutcome: not-found"),
				},
			},
		},
		"/fhir/ConceptMap/$translate":      conceptMapOperationPath("$translate", "ConceptMap $translate"),
		"/fhir/ConceptMap/{id}/$translate": conceptMapOperationPathWithID("$translate", "ConceptMap $translate (instance form)"),
		"/fhir/Questionnaire": map[string]any{
			"get": map[string]any{
				"tags":        []string{"FHIR"},
				"summary":     "Search LOINC panel Questionnaires",
				"description": "One Questionnaire per LOINC panel/form term (plan §4.11).",
				"parameters": append([]map[string]any{
					queryParamEx("url", "Canonical Questionnaire URL", "http://loinc.org/q/89689-4"),
				}, summaryResultParams()...),
				"responses": map[string]any{
					"200": fhirResponse("searchset Bundle of Questionnaire resources"),
				},
			},
		},
		"/fhir/Questionnaire/{id}": map[string]any{
			"get": map[string]any{
				"tags":        []string{"FHIR"},
				"summary":     "Read a panel Questionnaire",
				"description": "id is the panel's LOINC number. A non-panel LOINC 404s (plan §4.11).",
				"parameters":  append([]map[string]any{pathParamEx("id", "Panel LOINC number", "89689-4")}, summaryResultParams()...),
				"responses": map[string]any{
					"200": fhirResponse("Questionnaire resource, see docs/exemplars/fhir.loinc.org/questionnaire-89689-4.json"),
					"404": fhirResponse("OperationOutcome: not-found"),
				},
			},
		},
		"/searchapi/{scope}": map[string]any{
			"get": map[string]any{
				"tags":        []string{"LOINC Search API"},
				"summary":     "Local LOINC Search API clone",
				"description": "Wire-compatible local clone of the official https://loinc.regenstrief.org/searchapi/{scope} endpoint, answered entirely from the local database and search index. Basic-auth headers are accepted and ignored.",
				"parameters": []map[string]any{
					pathParam("scope", "Result scope: loincs, parts, answerlists, or groups"),
					queryParam("query", "Search query text, using the same field syntax as the local Lucene search"),
					intQueryParam("rows", "Rows to return (default 20, max 500)", 20),
					intQueryParam("offset", "Zero-based row offset", 0),
					queryParam("sortorder", "Field name plus optional \" asc\"/\" desc\" (for example \"loinc_num desc\")"),
					queryParam("language", "LinguisticVariants ID; loincs rows swap in that language's translated fields"),
					boolQueryParam("includefiltercounts", "Include a FilterCounts facet summary (loincs scope only)"),
				},
				"responses": map[string]any{
					"200": response("Search API response envelope (ResponseSummary, Results, optional FilterCounts)", map[string]any{"type": "object"}),
					"404": response("Unknown scope", object(map[string]any{"Message": map[string]any{"type": "string"}})),
					"503": response("Local search index not built", object(map[string]any{"Message": map[string]any{"type": "string"}})),
				},
			},
		},
		"/api/v1/official/credentials/status": map[string]any{
			"get": map[string]any{
				"summary":     "Check saved official LOINC API credential status",
				"description": "Reports whether encrypted official LOINC API credentials are saved and usable by the local proxy.",
				"responses": map[string]any{
					"200": response("Official API credential status", ref("OfficialCredentialStatus")),
				},
			},
		},
		"/api/v1/official/credentials": map[string]any{
			"delete": map[string]any{
				"summary":     "Delete saved official LOINC API credentials",
				"description": "Removes encrypted credentials from the local file-backed settings store. It does not delete the app key file.",
				"responses": map[string]any{
					"200": response("Official API credential status", ref("OfficialCredentialStatus")),
				},
			},
		},
		"/api/v1/official/search": map[string]any{
			"post": map[string]any{
				"summary":     "Proxy official LOINC Search API queries",
				"description": "Server-side proxy for the official LOINC Search API. Credentials are accepted in the JSON body or loaded from encrypted local settings so they are never sent in URL query strings.",
				"requestBody": map[string]any{
					"required": true,
					"content": map[string]any{
						"application/json": map[string]any{"schema": ref("OfficialSearchRequest")},
					},
				},
				"responses": map[string]any{
					"200": response("Official LOINC Search API response envelope", ref("OfficialSearchResponse")),
					"400": response("Invalid official API request", ref("ErrorResponse")),
					"401": response("Missing or rejected official API credentials", ref("ErrorResponse")),
					"502": response("Official API upstream error", ref("ErrorResponse")),
				},
			},
		},
		"/api/search": map[string]any{
			"get": map[string]any{
				"summary":     "Search LOINC terms",
				"description": "Search by exact LOINC number or SQLite FTS text query. Facet query parameters narrow results.",
				"parameters": []map[string]any{
					queryParam("q", "Full-text query or exact LOINC number"),
					queryParam("class", "LOINC class filter"),
					arrayQueryParam("status", "LOINC status filter. Repeat the parameter for multiple values."),
					queryParam("system", "System axis filter"),
					arrayQueryParam("timeAspect", "Time aspect axis filter. Repeat the parameter for multiple values."),
					arrayQueryParam("scale", "Scale axis filter. Repeat the parameter for multiple values."),
					arrayQueryParam("method", "Method axis filter. Repeat the parameter for multiple values."),
					queryParam("property", "Property axis filter"),
					arrayQueryParam("orderObs", "Order/observation filter. Repeat the parameter for multiple values."),
					boolQueryParam("rankedOnly", "When true, return only terms with COMMON_TEST_RANK > 0."),
					intQueryParam("limit", "Maximum results to return", 25),
					intQueryParam("offset", "Result offset for pagination", 0),
				},
				"responses": map[string]any{
					"200": response("Search results", ref("SearchResponse")),
				},
			},
		},
		"/api/terms/{loincNum}": map[string]any{
			"get": map[string]any{
				"summary": "Get one LOINC term",
				"parameters": []map[string]any{
					pathParam("loincNum", "LOINC number, for example 14749-6"),
				},
				"responses": map[string]any{
					"200": response("LOINC term detail", ref("Term")),
					"404": response("Term not found", ref("ErrorResponse")),
				},
			},
		},
		"/api/terms/{loincNum}/relationships": map[string]any{
			"get": map[string]any{
				"summary":     "Get direct and shared-concept relationships for one LOINC term",
				"description": "Returns outgoing and incoming MapTo links plus accessory concepts with sample terms sharing each concept.",
				"parameters": []map[string]any{
					pathParam("loincNum", "LOINC number, for example 14749-6"),
				},
				"responses": map[string]any{
					"200": response("LOINC relationship graph", ref("TermRelationshipGraph")),
					"404": response("Term not found", ref("ErrorResponse")),
				},
			},
		},
		"/api/facets": map[string]any{
			"get": map[string]any{
				"summary": "Get facet counts",
				"responses": map[string]any{
					"200": response("Facet counts", ref("Facets")),
				},
			},
		},
		"/api/cache": map[string]any{
			"get": map[string]any{
				"summary": "Get in-memory cache statistics",
				"responses": map[string]any{
					"200": response("Cache statistics", ref("CacheStats")),
				},
			},
		},
		"/api/source-organizations": map[string]any{
			"get": map[string]any{
				"summary": "List imported source organizations and copyright metadata",
				"responses": map[string]any{
					"200": response("Source organizations", map[string]any{"type": "array", "items": ref("SourceOrganization")}),
				},
			},
		},
		"/api/import/upload": map[string]any{
			"post": map[string]any{
				"summary":     "Upload and ingest a LOINC release zip",
				"description": "Accepts a multipart form upload with field releaseZip. The zip must contain LoincTable/Loinc.csv.",
				"requestBody": map[string]any{
					"required": true,
					"content": map[string]any{
						"multipart/form-data": map[string]any{
							"schema": object(map[string]any{
								"releaseZip": map[string]any{
									"type":   "string",
									"format": "binary",
								},
							}),
						},
					},
				},
				"responses": map[string]any{
					"200": response("Upload import summary", ref("UploadResponse")),
					"400": response("Upload or ingest error", ref("ErrorResponse")),
				},
			},
		},
	},
	"components": map[string]any{
		"schemas": map[string]any{
			"HealthResponse": object(map[string]any{
				"ok": map[string]any{"type": "boolean"},
			}),
			"VersionResponse": object(map[string]any{
				"version": map[string]any{"type": "string"},
				"commit":  map[string]any{"type": "string"},
				"date":    map[string]any{"type": "string"},
				"goos":    map[string]any{"type": "string"},
				"goarch":  map[string]any{"type": "string"},
			}),
			"SearchResponse": object(map[string]any{
				"results": map[string]any{"type": "array", "items": ref("SearchResult")},
				"total":   map[string]any{"type": "integer"},
				"limit":   map[string]any{"type": "integer"},
				"offset":  map[string]any{"type": "integer"},
				"hasMore": map[string]any{"type": "boolean"},
				"query":   map[string]any{"type": "string"},
				"_links":  linksSchema(),
			}),
			"SearchResult": object(map[string]any{
				"loincNum":        map[string]any{"type": "string"},
				"longCommonName":  map[string]any{"type": "string"},
				"shortName":       map[string]any{"type": "string"},
				"component":       map[string]any{"type": "string"},
				"property":        map[string]any{"type": "string"},
				"system":          map[string]any{"type": "string"},
				"scale":           map[string]any{"type": "string"},
				"method":          map[string]any{"type": "string"},
				"class":           map[string]any{"type": "string"},
				"status":          map[string]any{"type": "string"},
				"orderObs":        map[string]any{"type": "string"},
				"commonTestRank":  map[string]any{"type": "integer"},
				"commonOrderRank": map[string]any{"type": "integer"},
				"usageTypes":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"rank":            map[string]any{"type": "number"},
				"_links":          linksSchema(),
			}),
			"Term": object(map[string]any{
				"loincNum":        map[string]any{"type": "string"},
				"longCommonName":  map[string]any{"type": "string"},
				"shortName":       map[string]any{"type": "string"},
				"component":       map[string]any{"type": "string"},
				"property":        map[string]any{"type": "string"},
				"timeAspect":      map[string]any{"type": "string"},
				"system":          map[string]any{"type": "string"},
				"scale":           map[string]any{"type": "string"},
				"method":          map[string]any{"type": "string"},
				"class":           map[string]any{"type": "string"},
				"status":          map[string]any{"type": "string"},
				"definition":      map[string]any{"type": "string"},
				"consumerName":    map[string]any{"type": "string"},
				"relatedNames":    map[string]any{"type": "string"},
				"orderObs":        map[string]any{"type": "string"},
				"displayName":     map[string]any{"type": "string"},
				"commonTestRank":  map[string]any{"type": "integer"},
				"commonOrderRank": map[string]any{"type": "integer"},
				"usageTypes":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"_links":          linksSchema(),
				"fields":          map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
				"mapTo":           map[string]any{"type": "array", "items": ref("MapTo")},
				"parts":           map[string]any{"type": "array", "items": ref("TermAccessory")},
				"answerLists":     map[string]any{"type": "array", "items": ref("TermAccessory")},
				"panels":          map[string]any{"type": "array", "items": ref("TermAccessory")},
				"groups":          map[string]any{"type": "array", "items": ref("TermAccessory")},
				"hierarchy":       map[string]any{"type": "array", "items": ref("TermAccessory")},
			}),
			"MapTo": object(map[string]any{
				"loinc":   map[string]any{"type": "string"},
				"mapTo":   map[string]any{"type": "string"},
				"comment": map[string]any{"type": "string"},
			}),
			"TermAccessory": object(map[string]any{
				"kind":     map[string]any{"type": "string"},
				"code":     map[string]any{"type": "string"},
				"title":    map[string]any{"type": "string"},
				"subtitle": map[string]any{"type": "string"},
				"fields":   map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
			}),
			"TermRelationshipGraph": object(map[string]any{
				"loincNum":       map[string]any{"type": "string"},
				"outgoingMapTo":  map[string]any{"type": "array", "items": ref("MapTo")},
				"incomingMapTo":  map[string]any{"type": "array", "items": ref("MapTo")},
				"sharedConcepts": map[string]any{"type": "array", "items": ref("RelationshipConcept")},
			}),
			"TermRelationshipGroups": object(map[string]any{
				"loincNum":         map[string]any{"type": "string"},
				"mapTo":            map[string]any{"type": "array", "items": ref("MapTo")},
				"mappedFrom":       map[string]any{"type": "array", "items": ref("MapTo")},
				"parts":            map[string]any{"type": "array", "items": ref("TermAccessory")},
				"answerLists":      map[string]any{"type": "array", "items": ref("TermAccessory")},
				"panelMemberships": map[string]any{"type": "array", "items": ref("TermAccessory")},
				"panelItems":       map[string]any{"type": "array", "items": ref("TermAccessory")},
				"groups":           map[string]any{"type": "array", "items": ref("TermAccessory")},
				"hierarchy":        map[string]any{"type": "array", "items": ref("TermAccessory")},
				"sharedConcepts":   map[string]any{"type": "array", "items": ref("RelationshipConcept")},
				"_links":           linksSchema(),
			}),
			"TermFit": object(map[string]any{
				"loincNum":             map[string]any{"type": "string"},
				"status":               map[string]any{"type": "string"},
				"deprecated":           map[string]any{"type": "boolean"},
				"discouraged":          map[string]any{"type": "boolean"},
				"inactive":             map[string]any{"type": "boolean"},
				"orderObs":             map[string]any{"type": "string"},
				"usageTypes":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"commonTestRank":       map[string]any{"type": "integer"},
				"commonOrderRank":      map[string]any{"type": "integer"},
				"hasAnswerLists":       map[string]any{"type": "boolean"},
				"hasPanelItems":        map[string]any{"type": "boolean"},
				"hasPanelMemberships":  map[string]any{"type": "boolean"},
				"hasHierarchy":         map[string]any{"type": "boolean"},
				"hasExternalCopyright": map[string]any{"type": "boolean"},
				"_links":               linksSchema(),
			}),
			"RelationshipConcept": object(map[string]any{
				"kind":         map[string]any{"type": "string"},
				"code":         map[string]any{"type": "string"},
				"title":        map[string]any{"type": "string"},
				"subtitle":     map[string]any{"type": "string"},
				"fields":       map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
				"relatedTotal": map[string]any{"type": "integer"},
				"relatedTerms": map[string]any{"type": "array", "items": ref("TermSummary")},
			}),
			"TermSummary": object(map[string]any{
				"loincNum":        map[string]any{"type": "string"},
				"longCommonName":  map[string]any{"type": "string"},
				"shortName":       map[string]any{"type": "string"},
				"displayName":     map[string]any{"type": "string"},
				"status":          map[string]any{"type": "string"},
				"orderObs":        map[string]any{"type": "string"},
				"usageTypes":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"commonTestRank":  map[string]any{"type": "integer"},
				"commonOrderRank": map[string]any{"type": "integer"},
				"system":          map[string]any{"type": "string"},
				"class":           map[string]any{"type": "string"},
				"scale":           map[string]any{"type": "string"},
				"property":        map[string]any{"type": "string"},
				"_links":          linksSchema(),
			}),
			"HierarchyNode": object(map[string]any{
				"nodeId":       map[string]any{"type": "string"},
				"code":         map[string]any{"type": "string"},
				"label":        map[string]any{"type": "string"},
				"pathKey":      map[string]any{"type": "string"},
				"path":         map[string]any{"type": "string"},
				"parentNodeId": map[string]any{"type": "string"},
				"parentCode":   map[string]any{"type": "string"},
				"termCount":    map[string]any{"type": "integer"},
				"childCount":   map[string]any{"type": "integer"},
				"isTerm":       map[string]any{"type": "boolean"},
				"hasChildren":  map[string]any{"type": "boolean"},
				"_links":       linksSchema(),
			}),
			"HierarchyNodePage": pageSchema(ref("HierarchyNode")),
			"HierarchyChildrenResponse": object(map[string]any{
				"parentNodeId": map[string]any{"type": "string"},
				"parentCode":   map[string]any{"type": "string"},
				"query":        map[string]any{"type": "string"},
				"results":      map[string]any{"type": "array", "items": ref("HierarchyNode")},
				"_links":       linksSchema(),
			}),
			"AccessoryBrowseResponse": object(map[string]any{
				"results": map[string]any{"type": "array", "items": ref("AccessoryRecord")},
				"total":   map[string]any{"type": "integer"},
				"limit":   map[string]any{"type": "integer"},
				"offset":  map[string]any{"type": "integer"},
				"hasMore": map[string]any{"type": "boolean"},
				"query":   map[string]any{"type": "string"},
				"kind":    map[string]any{"type": "string"},
				"_links":  linksSchema(),
			}),
			"AccessoryRecord": object(map[string]any{
				"loincNum":       map[string]any{"type": "string"},
				"longCommonName": map[string]any{"type": "string"},
				"shortName":      map[string]any{"type": "string"},
				"status":         map[string]any{"type": "string"},
				"kind":           map[string]any{"type": "string"},
				"code":           map[string]any{"type": "string"},
				"title":          map[string]any{"type": "string"},
				"subtitle":       map[string]any{"type": "string"},
				"fields":         map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
			}),
			"SourceOrganization": object(map[string]any{
				"id":          map[string]any{"type": "string"},
				"copyrightId": map[string]any{"type": "string"},
				"name":        map[string]any{"type": "string"},
				"copyright":   map[string]any{"type": "string"},
				"termsOfUse":  map[string]any{"type": "string"},
				"url":         map[string]any{"type": "string"},
				"fields":      map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
			}),
			"AnswerList": object(map[string]any{
				"answerListId":   map[string]any{"type": "string"},
				"answerListName": map[string]any{"type": "string"},
				"answerListOid":  map[string]any{"type": "string"},
				"extDefinedYn":   map[string]any{"type": "string"},
				"_links":         linksSchema(),
			}),
			"AnswerListPage":       pageSchema(ref("AnswerList")),
			"AnswerListAnswerPage": pageSchema(ref("AnswerListAnswer")),
			"AnswerListAnswer": object(map[string]any{
				"answerListId":          map[string]any{"type": "string"},
				"answerStringId":        map[string]any{"type": "string"},
				"localAnswerCode":       map[string]any{"type": "string"},
				"localAnswerCodeSystem": map[string]any{"type": "string"},
				"sequenceNumber":        map[string]any{"type": "integer"},
				"displayText":           map[string]any{"type": "string"},
				"extCodeId":             map[string]any{"type": "string"},
				"extCodeDisplayName":    map[string]any{"type": "string"},
				"extCodeSystem":         map[string]any{"type": "string"},
				"score":                 map[string]any{"type": "string"},
				"_links":                linksSchema(),
			}),
			"Part": object(map[string]any{
				"partNumber":      map[string]any{"type": "string"},
				"partTypeName":    map[string]any{"type": "string"},
				"partName":        map[string]any{"type": "string"},
				"partDisplayName": map[string]any{"type": "string"},
				"status":          map[string]any{"type": "string"},
				"_links":          linksSchema(),
			}),
			"PartPage": pageSchema(ref("Part")),
			"LOINCGroup": object(map[string]any{
				"groupId":              map[string]any{"type": "string"},
				"parentGroupId":        map[string]any{"type": "string"},
				"groupName":            map[string]any{"type": "string"},
				"archetype":            map[string]any{"type": "string"},
				"status":               map[string]any{"type": "string"},
				"versionFirstReleased": map[string]any{"type": "string"},
				"_links":               linksSchema(),
			}),
			"GroupPage":         pageSchema(ref("LOINCGroup")),
			"TermAccessoryPage": pageSchema(ref("TermAccessory")),
			"PanelItem": object(map[string]any{
				"parentLoincNum":       map[string]any{"type": "string"},
				"childLoincNum":        map[string]any{"type": "string"},
				"sequence":             map[string]any{"type": "integer"},
				"itemId":               map[string]any{"type": "string"},
				"displayNameForForm":   map[string]any{"type": "string"},
				"observationRequired":  map[string]any{"type": "string"},
				"entryType":            map[string]any{"type": "string"},
				"dataTypeInForm":       map[string]any{"type": "string"},
				"answerListIdOverride": map[string]any{"type": "string"},
				"childTerm":            ref("TermSummary"),
				"_links":               linksSchema(),
			}),
			"PanelItemPage": pageSchema(ref("PanelItem")),
			"TermCopyright": object(map[string]any{
				"loincNum":             map[string]any{"type": "string"},
				"status":               map[string]any{"type": "string"},
				"hasExternalCopyright": map[string]any{"type": "boolean"},
				"state":                map[string]any{"type": "string"},
				"sourceOrganizations":  map[string]any{"type": "array", "items": ref("SourceOrganization")},
				"_links":               linksSchema(),
			}),
			"OfficialCredentialStatus": object(map[string]any{
				"saved":          map[string]any{"type": "boolean"},
				"usable":         map[string]any{"type": "boolean"},
				"maskedUsername": map[string]any{"type": "string"},
				"message":        map[string]any{"type": "string"},
			}),
			"OfficialSearchRequest": object(map[string]any{
				"scope":               map[string]any{"type": "string", "enum": []string{"loincs", "answerlists", "parts", "groups"}},
				"query":               map[string]any{"type": "string"},
				"rows":                map[string]any{"type": "integer"},
				"offset":              map[string]any{"type": "integer"},
				"sortorder":           map[string]any{"type": "string"},
				"language":            map[string]any{"type": "integer"},
				"includefiltercounts": map[string]any{"type": "boolean"},
				"username":            map[string]any{"type": "string"},
				"password":            map[string]any{"type": "string", "format": "password"},
				"remember":            map[string]any{"type": "boolean"},
				"useSavedCredentials": map[string]any{"type": "boolean"},
			}),
			"OfficialSearchResponse": object(map[string]any{
				"scope":          map[string]any{"type": "string"},
				"params":         map[string]any{"type": "object", "additionalProperties": true},
				"upstreamStatus": map[string]any{"type": "integer"},
				"payload":        map[string]any{},
				"local":          ref("OfficialLocalIntegration"),
			}),
			"OfficialLocalIntegration": object(map[string]any{
				"available": map[string]any{"type": "boolean"},
				"loincNums": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"matched":   map[string]any{"type": "integer"},
				"missing":   map[string]any{"type": "integer"},
				"matches":   map[string]any{"type": "object", "additionalProperties": ref("OfficialLocalMatch")},
				"message":   map[string]any{"type": "string"},
			}),
			"OfficialLocalMatch": object(map[string]any{
				"loincNum": map[string]any{"type": "string"},
				"found":    map[string]any{"type": "boolean"},
				"term":     ref("OfficialLocalTermSummary"),
				"localUrl": map[string]any{"type": "string"},
			}),
			"OfficialLocalTermSummary": object(map[string]any{
				"loincNum":       map[string]any{"type": "string"},
				"longCommonName": map[string]any{"type": "string"},
				"shortName":      map[string]any{"type": "string"},
				"status":         map[string]any{"type": "string"},
				"system":         map[string]any{"type": "string"},
				"class":          map[string]any{"type": "string"},
				"property":       map[string]any{"type": "string"},
				"scale":          map[string]any{"type": "string"},
			}),
			"LocalSearchStatus": object(map[string]any{
				"state":         map[string]any{"type": "string", "enum": []string{"missing", "ready", "requires_reingest", "error"}},
				"indexPath":     map[string]any{"type": "string"},
				"docCount":      map[string]any{"type": "integer"},
				"updatedAt":     map[string]any{"type": "string"},
				"fieldCoverage": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
				"warnings":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"message":       map[string]any{"type": "string"},
			}),
			"LocalSearchRequest": object(map[string]any{
				"scope":  map[string]any{"type": "string", "enum": []string{"loincs", "answerlists", "parts", "groups"}},
				"query":  map[string]any{"type": "string"},
				"limit":  map[string]any{"type": "integer"},
				"offset": map[string]any{"type": "integer"},
			}),
			"LocalSearchResponse": object(map[string]any{
				"scope":       map[string]any{"type": "string"},
				"query":       map[string]any{"type": "string"},
				"results":     map[string]any{"type": "array", "items": ref("LocalSearchResult")},
				"total":       map[string]any{"type": "integer"},
				"limit":       map[string]any{"type": "integer"},
				"offset":      map[string]any{"type": "integer"},
				"warnings":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"indexStatus": map[string]any{"type": "string"},
			}),
			"LocalSearchResult": object(map[string]any{
				"id":     map[string]any{"type": "string"},
				"scope":  map[string]any{"type": "string"},
				"key":    map[string]any{"type": "string"},
				"score":  map[string]any{"type": "number"},
				"result": map[string]any{"type": "object", "additionalProperties": true},
			}),
			"Facets": object(map[string]any{
				"classes":     stringIntMap(),
				"statuses":    stringIntMap(),
				"systems":     stringIntMap(),
				"timeAspects": stringIntMap(),
				"scales":      stringIntMap(),
				"methods":     stringIntMap(),
				"properties":  stringIntMap(),
				"orderObs":    stringIntMap(),
			}),
			"CacheStats": object(map[string]any{
				"termHits":            map[string]any{"type": "integer"},
				"termMisses":          map[string]any{"type": "integer"},
				"relationshipHits":    map[string]any{"type": "integer"},
				"relationshipMisses":  map[string]any{"type": "integer"},
				"accessoryHits":       map[string]any{"type": "integer"},
				"accessoryMisses":     map[string]any{"type": "integer"},
				"facetHits":           map[string]any{"type": "integer"},
				"facetMisses":         map[string]any{"type": "integer"},
				"termEntries":         map[string]any{"type": "integer"},
				"relationshipEntries": map[string]any{"type": "integer"},
				"accessoryEntries":    map[string]any{"type": "integer"},
				"facetEntries":        map[string]any{"type": "integer"},
			}),
			"UploadResponse": object(map[string]any{
				"ok":         map[string]any{"type": "boolean"},
				"termCount":  map[string]any{"type": "integer"},
				"dbPath":     map[string]any{"type": "string"},
				"releaseDir": map[string]any{"type": "string"},
				"importedAt": map[string]any{"type": "string", "format": "date-time"},
			}),
			"ErrorResponse": object(map[string]any{
				"error": map[string]any{"type": "string"},
			}),
		},
	},
}

func response(description string, schema map[string]any) map[string]any {
	return map[string]any{
		"description": description,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": schema,
			},
		},
	}
}

func ref(name string) map[string]any {
	return map[string]any{"$ref": "#/components/schemas/" + name}
}

func object(properties map[string]any) map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": properties,
	}
}

func stringIntMap() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": map[string]any{"type": "integer"},
	}
}

func linksSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": map[string]any{"type": "string"},
	}
}

func pageSchema(itemSchema map[string]any) map[string]any {
	return object(map[string]any{
		"results": map[string]any{"type": "array", "items": itemSchema},
		"total":   map[string]any{"type": "integer"},
		"limit":   map[string]any{"type": "integer"},
		"offset":  map[string]any{"type": "integer"},
		"hasMore": map[string]any{"type": "boolean"},
		"_links": object(map[string]any{
			"self": map[string]any{"type": "string"},
			"next": map[string]any{"type": "string"},
			"prev": map[string]any{"type": "string"},
		}),
	})
}

func commonTermListParameters() []map[string]any {
	return []map[string]any{
		queryParam("q", "Full-text query or exact LOINC number"),
		arrayQueryParam("class", "LOINC class filter; repeat to allow several classes (e.g. class=CHEM&class=SERO)"),
		queryParam("classType", "LOINC CLASSTYPE filter: lab, clinical (includes radiology), attachment, or survey"),
		arrayQueryParam("status", "LOINC status filter. Defaults to all statuses except DEPRECATED. Use status=DEPRECATED to browse deprecated terms, or status=* for all statuses."),
		queryParam("usageType", "Term usage filter: any, observation, or order"),
		queryParam("rankMode", "Ranking mode: observation or order"),
		queryParam("sort", "Sort mode: relevance, usage, or alpha"),
		queryParam("hierarchyNodeId", "Restrict term results to a hierarchy occurrence node id"),
		queryParam("system", "System axis filter"),
		arrayQueryParam("timeAspect", "Time aspect axis filter"),
		arrayQueryParam("scale", "Scale axis filter"),
		arrayQueryParam("method", "Method axis filter"),
		queryParam("property", "Property axis filter"),
		arrayQueryParam("orderObs", "Raw ORDER_OBS filter"),
		boolQueryParam("rankedOnly", "When true, return only terms with a positive rank in the selected rank mode."),
		intQueryParam("limit", "Maximum results to return. Maximum 100.", 25),
		intQueryParam("offset", "Result offset for pagination", 0),
	}
}

func v1TermListPath(summary string, leadingParams ...map[string]any) map[string]any {
	return map[string]any{
		"get": map[string]any{
			"summary":    summary,
			"parameters": termListParameters(leadingParams...),
			"responses": map[string]any{
				"200": response("Term results", ref("SearchResponse")),
			},
		},
	}
}

func termListParameters(leadingParams ...map[string]any) []map[string]any {
	params := make([]map[string]any, 0, len(leadingParams)+len(commonTermListParameters()))
	params = append(params, leadingParams...)
	params = append(params, commonTermListParameters()...)
	return params
}

func queryParam(name string, description string) map[string]any {
	return map[string]any{
		"name":        name,
		"in":          "query",
		"description": description,
		"schema":      map[string]any{"type": "string"},
	}
}

func arrayQueryParam(name string, description string) map[string]any {
	return map[string]any{
		"name":        name,
		"in":          "query",
		"description": description,
		"style":       "form",
		"explode":     true,
		"schema": map[string]any{
			"type":  "array",
			"items": map[string]any{"type": "string"},
		},
	}
}

func intQueryParam(name string, description string, defaultValue int) map[string]any {
	return map[string]any{
		"name":        name,
		"in":          "query",
		"description": description,
		"schema": map[string]any{
			"type":    "integer",
			"default": defaultValue,
		},
	}
}

func boolQueryParam(name string, description string) map[string]any {
	return map[string]any{
		"name":        name,
		"in":          "query",
		"description": description,
		"schema":      map[string]any{"type": "boolean"},
	}
}

func pathParam(name string, description string) map[string]any {
	return map[string]any{
		"name":        name,
		"in":          "path",
		"required":    true,
		"description": description,
		"schema":      map[string]any{"type": "string"},
	}
}

// --- FHIR (/fhir/...) helpers ---
//
// The FHIR routes registered by internal/fhirhttp respond with
// "application/fhir+json" bodies whose exact shape is pinned to the captured
// golden responses in docs/exemplars/fhir.loinc.org/ (see
// docs/FHIR_TERMINOLOGY_PLAN.md §4). Rather than duplicating ~80 CodeSystem
// properties and every resource shape as OpenAPI schemas, responses here
// document media type, status, and a pointer to the matching exemplar file;
// Swagger UI's "Try it out" still works against the live routes.

func fhirResponse(description string) map[string]any {
	return map[string]any{
		"description": description,
		"content": map[string]any{
			"application/fhir+json": map[string]any{
				"schema": map[string]any{"type": "object"},
			},
		},
	}
}

// summaryResultParams documents the FHIR R4 `_summary`/`_elements` search result parameters
// (plan §4.13), shared by every read/search/$expand operation this server filters.
func summaryResultParams() []map[string]any {
	return []map[string]any{
		queryParam("_summary", "true | text | data | count | false. true keeps R4 summary elements, text keeps text+mandatory, data drops text, count (search only) returns an empty Bundle with just total, false is the default full resource. Combining with _elements is 400 invalid."),
		queryParam("_elements", "Comma-separated top-level element names to keep, plus resourceType/id/meta/mandatory elements. Unknown names are ignored."),
	}
}

func queryParamEx(name string, description string, example string) map[string]any {
	p := queryParam(name, description)
	p["example"] = example
	return p
}

func pathParamEx(name string, description string, example string) map[string]any {
	p := pathParam(name, description)
	p["example"] = example
	return p
}

func codeSystemLookupParams() []map[string]any {
	return []map[string]any{
		queryParamEx("system", "Must be http://loinc.org", "http://loinc.org"),
		queryParamEx("code", "Term, LP part, LL answer list, LA answer, or LG group code", "718-7"),
		queryParam("version", "LOINC version; absent or a prefix of the loaded version matches"),
		queryParam("coding", "Coding-typed alternative to system+code, as system|code"),
		queryParam("displayLanguage", "BCP-47 language tag; narrows designations to en-US plus this language"),
		arrayQueryParam("property", "Repeatable; restrict the returned property list to these codes, in request order"),
	}
}

func codeSystemValidateCodeParams() []map[string]any {
	return []map[string]any{
		queryParamEx("url", "CodeSystem canonical URL (or use system)", "http://loinc.org"),
		queryParam("system", "Alternative to url"),
		queryParamEx("code", "Code to validate", "718-7"),
		queryParam("version", "LOINC version"),
		queryParam("display", "Expected display text; mismatch returns result=false"),
		queryParam("coding", "Coding-typed alternative to system+code, as system|code"),
		queryParam("codeableConcept", "CodeableConcept-typed alternative; first LOINC coding wins"),
		queryParam("displayLanguage", "BCP-47 language tag used when checking display"),
	}
}

func codeSystemSubsumesParams() []map[string]any {
	return []map[string]any{
		queryParamEx("codeA", "First code (LOINC term or LP part)", "LP384441-4"),
		queryParamEx("codeB", "Second code", "30064-0"),
		queryParamEx("system", "Must be http://loinc.org", "http://loinc.org"),
		queryParam("version", "LOINC version"),
		queryParam("codingA", "Coding-typed alternative to codeA, as system|code"),
		queryParam("codingB", "Coding-typed alternative to codeB, as system|code"),
	}
}

func codeSystemOperationPath(op string, summary string, description string, params []map[string]any) map[string]any {
	return fhirOperationPath(op, summary, description, params)
}

func codeSystemOperationPathWithID(op string, summary string, description string, params []map[string]any) map[string]any {
	return fhirOperationPath(op, summary, description, append([]map[string]any{pathParamEx("id", "CodeSystem id", "loinc-2.82")}, params...))
}

func valueSetExpandParams() []map[string]any {
	return []map[string]any{
		queryParamEx("url", "ValueSet canonical URL", "http://loinc.org/vs/LL1162-8"),
		queryParam("valueSet", "POST only: an inline ValueSet resource to expand, e.g. compose.include[].filter[]"),
		queryParam("valueSetVersion", "ValueSet version"),
		queryParam("filter", "Case-insensitive word-prefix match on display, all tokens required"),
		intQueryParam("offset", "Result offset", 0),
		intQueryParam("count", "Page size (default 100, max 1000; count=0 returns no contains but a real total)", 100),
		boolQueryParam("activeOnly", "Exclude DEPRECATED members when true (default false)"),
		boolQueryParam("includeDesignations", "Include contains[].designation[]"),
		queryParam("displayLanguage", "BCP-47 language tag for designations"),
		queryParam("_summary", "true | text | data | false (not count: $expand returns one ValueSet, not a searchset). See plan §4.13."),
		queryParam("_elements", "Comma-separated top-level element names to keep, plus resourceType/id/meta/mandatory. Unknown names are ignored."),
	}
}

func valueSetValidateCodeParams() []map[string]any {
	return []map[string]any{
		queryParam("url", "ValueSet canonical URL (or use the instance id form)"),
		queryParam("valueSet", "POST only: inline ValueSet resource"),
		queryParamEx("code", "Code to validate against the value set", "LA15679-6"),
		queryParamEx("system", "Code system, normally http://loinc.org", "http://loinc.org"),
		queryParam("display", "Expected display text"),
		queryParam("coding", "Coding-typed alternative to system+code, as system|code"),
		queryParam("codeableConcept", "CodeableConcept-typed alternative"),
		boolQueryParam("activeOnly", "Reject DEPRECATED codes when true"),
	}
}

func valueSetOperationPath(op string, summary string, description string, params []map[string]any) map[string]any {
	return fhirOperationPath(op, summary, description, params)
}

func valueSetOperationPathWithID(op string, summary string, description string, params []map[string]any) map[string]any {
	return fhirOperationPath(op, summary, description, append([]map[string]any{pathParamEx("id", "ValueSet id", "LL1162-8")}, params...))
}

func conceptMapTranslateParams() []map[string]any {
	return []map[string]any{
		queryParam("url", "ConceptMap canonical URL (or use the instance id form); omit to search every map matching system"),
		queryParam("conceptMap", "POST only: inline ConceptMap resource"),
		queryParamEx("code", "Source code to translate", "11556-8"),
		queryParamEx("system", "Source system, normally http://loinc.org", "http://loinc.org"),
		queryParam("version", "Source system version"),
		queryParam("source", "Source value set URI"),
		queryParam("coding", "Coding-typed alternative to system+code, as system|code"),
		queryParam("codeableConcept", "CodeableConcept-typed alternative"),
		queryParam("target", "Target value set URI"),
		queryParam("targetsystem", "Target system URI filter"),
		boolQueryParam("reverse", "Translate via the map's reverse direction"),
	}
}

func conceptMapOperationPath(op string, summary string) map[string]any {
	return fhirOperationPath(op, summary, "`result` is a valueBoolean; each match's equivalence, concept, and source map url follow (plan §4.10). 11556-8 -> IEEE 160116; 30657-1 -> RadLex relatedto.", conceptMapTranslateParams())
}

func conceptMapOperationPathWithID(op string, summary string) map[string]any {
	return fhirOperationPath(op, summary, "Same as /fhir/ConceptMap/$translate, scoped to a ConceptMap instance.", append([]map[string]any{pathParamEx("id", "ConceptMap id", "loinc-to-ieee-11073-10101")}, conceptMapTranslateParams()...))
}

// fhirOperationPath documents one FHIR "$operation" path with both GET (query
// parameters) and POST (a FHIR Parameters body, application/fhir+json or
// application/json) forms, per plan §1's "every operation accepts GET query
// parameters and POST Parameters bodies" convention. Note the literal "$" in
// the path segment.
func fhirOperationPath(op string, summary string, description string, params []map[string]any) map[string]any {
	return map[string]any{
		"get": map[string]any{
			"tags":        []string{"FHIR"},
			"summary":     summary,
			"description": description,
			"parameters":  params,
			"responses": map[string]any{
				"200": fhirResponse("Parameters resource with the " + op + " result"),
				"400": fhirResponse("OperationOutcome: invalid or required"),
				"404": fhirResponse("OperationOutcome: not-found"),
			},
		},
		"post": map[string]any{
			"tags":        []string{"FHIR"},
			"summary":     summary + " (POST Parameters body)",
			"description": description + " Accepts a FHIR Parameters resource body instead of query parameters.",
			"requestBody": map[string]any{
				"content": map[string]any{
					"application/fhir+json": map[string]any{"schema": map[string]any{"type": "object", "description": "FHIR Parameters resource"}},
					"application/json":      map[string]any{"schema": map[string]any{"type": "object", "description": "FHIR Parameters resource"}},
				},
			},
			"responses": map[string]any{
				"200": fhirResponse("Parameters resource with the " + op + " result"),
				"400": fhirResponse("OperationOutcome: invalid or required"),
				"404": fhirResponse("OperationOutcome: not-found"),
			},
		},
	}
}
