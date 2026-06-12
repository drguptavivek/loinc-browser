package server

var openAPISpec = map[string]any{
	"openapi": "3.1.0",
	"info": map[string]any{
		"title":       "LOINC Browser API",
		"version":     "0.1.0",
		"description": "Local API for searching and browsing an imported licensed LOINC release.",
	},
	"servers": []map[string]any{
		{
			"url":         "http://localhost:8080",
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
		"/api/accessories": map[string]any{
			"get": map[string]any{
				"summary": "Browse imported relationship and accessory rows",
				"parameters": []map[string]any{
					queryParam("kind", "Optional relationship kind such as part-primary, answer-list, panel-membership, group, or hierarchy"),
					queryParam("q", "Search relationship title, code, subtitle, or linked LOINC number"),
					intQueryParam("limit", "Maximum rows to return", 50),
					intQueryParam("offset", "Result offset for pagination", 0),
				},
				"responses": map[string]any{
					"200": response("Relationship rows", ref("AccessoryBrowseResponse")),
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
			"SearchResponse": object(map[string]any{
				"results": map[string]any{"type": "array", "items": ref("SearchResult")},
				"total":   map[string]any{"type": "integer"},
				"limit":   map[string]any{"type": "integer"},
				"offset":  map[string]any{"type": "integer"},
				"query":   map[string]any{"type": "string"},
			}),
			"SearchResult": object(map[string]any{
				"loincNum":       map[string]any{"type": "string"},
				"longCommonName": map[string]any{"type": "string"},
				"shortName":      map[string]any{"type": "string"},
				"component":      map[string]any{"type": "string"},
				"property":       map[string]any{"type": "string"},
				"system":         map[string]any{"type": "string"},
				"scale":          map[string]any{"type": "string"},
				"method":         map[string]any{"type": "string"},
				"class":          map[string]any{"type": "string"},
				"status":         map[string]any{"type": "string"},
				"orderObs":       map[string]any{"type": "string"},
				"rank":           map[string]any{"type": "number"},
			}),
			"Term": object(map[string]any{
				"loincNum":       map[string]any{"type": "string"},
				"longCommonName": map[string]any{"type": "string"},
				"shortName":      map[string]any{"type": "string"},
				"component":      map[string]any{"type": "string"},
				"property":       map[string]any{"type": "string"},
				"timeAspect":     map[string]any{"type": "string"},
				"system":         map[string]any{"type": "string"},
				"scale":          map[string]any{"type": "string"},
				"method":         map[string]any{"type": "string"},
				"class":          map[string]any{"type": "string"},
				"status":         map[string]any{"type": "string"},
				"definition":     map[string]any{"type": "string"},
				"consumerName":   map[string]any{"type": "string"},
				"relatedNames":   map[string]any{"type": "string"},
				"orderObs":       map[string]any{"type": "string"},
				"displayName":    map[string]any{"type": "string"},
				"fields":         map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
				"mapTo":          map[string]any{"type": "array", "items": ref("MapTo")},
				"parts":          map[string]any{"type": "array", "items": ref("TermAccessory")},
				"answerLists":    map[string]any{"type": "array", "items": ref("TermAccessory")},
				"panels":         map[string]any{"type": "array", "items": ref("TermAccessory")},
				"groups":         map[string]any{"type": "array", "items": ref("TermAccessory")},
				"hierarchy":      map[string]any{"type": "array", "items": ref("TermAccessory")},
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
				"loincNum":       map[string]any{"type": "string"},
				"longCommonName": map[string]any{"type": "string"},
				"shortName":      map[string]any{"type": "string"},
				"status":         map[string]any{"type": "string"},
				"system":         map[string]any{"type": "string"},
				"class":          map[string]any{"type": "string"},
			}),
			"AccessoryBrowseResponse": object(map[string]any{
				"results": map[string]any{"type": "array", "items": ref("AccessoryRecord")},
				"total":   map[string]any{"type": "integer"},
				"limit":   map[string]any{"type": "integer"},
				"offset":  map[string]any{"type": "integer"},
				"query":   map[string]any{"type": "string"},
				"kind":    map[string]any{"type": "string"},
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

func pathParam(name string, description string) map[string]any {
	return map[string]any{
		"name":        name,
		"in":          "path",
		"required":    true,
		"description": description,
		"schema":      map[string]any{"type": "string"},
	}
}
