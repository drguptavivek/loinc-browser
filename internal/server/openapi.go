package server

var openAPISpec = map[string]any{
	"openapi": "3.1.0",
	"info": map[string]any{
		"title":       "LOINC Browser API",
		"version":     "0.90",
		"description": "Local API for searching and browsing an imported licensed LOINC release.",
	},
	"servers": []map[string]any{
		{
			"url":         "http://localhost:9005",
			"description": "Default local development server",
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
				"description": "Excludes inactive terms by default. Use status=INACTIVE to search inactive terms, or status=* to include every status.",
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
		queryParam("class", "LOINC class filter"),
		arrayQueryParam("status", "LOINC status filter. Defaults to all statuses except INACTIVE. Use status=INACTIVE to search inactive terms, or status=* for all statuses."),
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
