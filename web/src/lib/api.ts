export type SearchResult = {
	loincNum: string;
	longCommonName: string;
	shortName: string;
	component: string;
	property: string;
	system: string;
	scale: string;
	method: string;
	class: string;
	status: string;
	orderObs: string;
	commonTestRank: number;
	commonOrderRank: number;
	usageTypes: string[];
	rank: number;
	_links?: Links;
};

export type SearchResponse = {
	results: SearchResult[];
	total: number;
	limit: number;
	offset: number;
	hasMore: boolean;
	query: string;
	_links?: Links;
};

export type Links = Record<string, string>;

export type Term = {
	loincNum: string;
	longCommonName: string;
	shortName: string;
	component: string;
	property: string;
	timeAspect: string;
	system: string;
	scale: string;
	method: string;
	class: string;
	status: string;
	definition: string;
	consumerName: string;
	relatedNames: string;
	orderObs: string;
	displayName: string;
	commonTestRank: number;
	commonOrderRank: number;
	usageTypes: string[];
	_links?: Links;
	fields: Record<string, string>;
	mapTo?: MapTo[];
	parts?: TermAccessory[];
	answerLists?: TermAccessory[];
	panels?: TermAccessory[];
	groups?: TermAccessory[];
	hierarchy?: TermAccessory[];
};

export type MapTo = {
	loinc: string;
	mapTo: string;
	comment: string;
};

export type TermAccessory = {
	kind: string;
	code: string;
	title: string;
	subtitle: string;
	fields: Record<string, string>;
};

export type TermSummary = {
	loincNum: string;
	longCommonName: string;
	shortName: string;
	displayName?: string;
	status: string;
	orderObs: string;
	usageTypes: string[];
	commonTestRank: number;
	commonOrderRank: number;
	system: string;
	class: string;
	scale?: string;
	property?: string;
	_links?: Links;
};

export type RelationshipConcept = {
	kind: string;
	code: string;
	title: string;
	subtitle: string;
	fields: Record<string, string>;
	relatedTotal: number;
	relatedTerms: TermSummary[];
};

export type TermRelationshipGraph = {
	loincNum: string;
	outgoingMapTo?: MapTo[];
	incomingMapTo?: MapTo[];
	sharedConcepts?: RelationshipConcept[];
	mapTo?: MapTo[];
	mappedFrom?: MapTo[];
	parts?: TermAccessory[];
	answerLists?: TermAccessory[];
	panelMemberships?: TermAccessory[];
	panelItems?: TermAccessory[];
	groups?: TermAccessory[];
	hierarchy?: TermAccessory[];
	_links?: Links;
};

export type AccessoryRecord = TermAccessory & {
	loincNum: string;
	longCommonName: string;
	shortName: string;
	status: string;
};

export type AccessoryBrowseResponse = {
	results: AccessoryRecord[];
	total: number;
	limit: number;
	offset: number;
	hasMore: boolean;
	query: string;
	kind: string;
	_links?: Links;
};

export type HierarchyNode = {
	nodeId: string;
	code: string;
	label: string;
	parentNodeId: string;
	parentCode: string;
	pathKey: string;
	path: string;
	termCount: number;
	childCount: number;
	isTerm: boolean;
	hasChildren: boolean;
	_links?: Links;
};

export type HierarchyChildrenResponse = {
	parentNodeId: string;
	parentCode: string;
	query: string;
	results: HierarchyNode[];
	_links?: Links;
};

export type HierarchyNodePage = {
	results: HierarchyNode[];
	total: number;
	limit: number;
	offset: number;
	hasMore: boolean;
	_links?: Links;
};

export type Facets = {
	classes: Record<string, number>;
	statuses: Record<string, number>;
	systems: Record<string, number>;
	timeAspects: Record<string, number>;
	scales: Record<string, number>;
	methods: Record<string, number>;
	properties: Record<string, number>;
	orderObs: Record<string, number>;
};

export type CacheStats = {
	termHits: number;
	termMisses: number;
	relationshipHits: number;
	relationshipMisses: number;
	accessoryHits: number;
	accessoryMisses: number;
	facetHits: number;
	facetMisses: number;
	termEntries: number;
	relationshipEntries: number;
	accessoryEntries: number;
	facetEntries: number;
};

export type UploadImportResponse = {
	ok: boolean;
	termCount: number;
	dbPath: string;
	releaseDir: string;
	importedAt: string;
};

export type VersionInfo = {
	version: string;
	commit: string;
	date?: string;
	goos: string;
	goarch: string;
};

export type OfficialCredentialStatus = {
	saved: boolean;
	usable: boolean;
	maskedUsername?: string;
	message?: string;
};

export type OfficialSearchRequest = {
	scope: 'loincs' | 'answerlists' | 'parts' | 'groups';
	query: string;
	rows?: number;
	offset?: number;
	sortorder?: string;
	language?: number;
	includefiltercounts?: boolean;
	username?: string;
	password?: string;
	remember?: boolean;
	useSavedCredentials?: boolean;
};

export type OfficialSearchResponse = {
	scope: string;
	params: Record<string, unknown>;
	upstreamStatus: number;
	payload: unknown;
	local?: OfficialLocalIntegration;
};

export type OfficialLocalIntegration = {
	available: boolean;
	loincNums: string[];
	matched: number;
	missing: number;
	matches: Record<string, OfficialLocalMatch>;
	message?: string;
};

export type OfficialLocalMatch = {
	loincNum: string;
	found: boolean;
	term?: {
		loincNum: string;
		longCommonName: string;
		shortName: string;
		status: string;
		system: string;
		class: string;
		property: string;
		scale: string;
	};
	localUrl?: string;
};

export type LocalSearchScope = 'loincs' | 'answerlists' | 'parts' | 'groups';

export type LocalSearchStatus = {
	state: string;
	indexPath: string;
	docCount: number;
	updatedAt?: string;
	fieldCoverage?: Record<string, string>;
	warnings?: string[];
	message?: string;
};

export type LocalSearchRequest = {
	scope: LocalSearchScope;
	query: string;
	limit?: number;
	offset?: number;
};

export type LocalSearchResult = {
	id: string;
	scope: string;
	key: string;
	score: number;
	result: Record<string, unknown>;
};

export type LocalSearchResponse = {
	scope: string;
	query: string;
	results: LocalSearchResult[];
	total: number;
	limit: number;
	offset: number;
	warnings?: string[];
	indexStatus: string;
};

export type SearchParams = {
	q?: string;
	class?: string;
	status?: string | string[];
	system?: string;
	timeAspect?: string | string[];
	scale?: string | string[];
	method?: string | string[];
	property?: string;
	orderObs?: string | string[];
	rankedOnly?: boolean;
	hierarchyNodeId?: string;
	usageType?: 'any' | 'observation' | 'order';
	rankMode?: 'observation' | 'order';
	sort?: 'relevance' | 'usage' | 'alpha';
	limit?: number;
	offset?: number;
};

async function requestJSON<T>(path: string): Promise<T> {
	const response = await fetch(path);
	if (!response.ok) {
		const body = await response.json().catch(() => ({ error: response.statusText }));
		throw new Error(body.error || response.statusText);
	}
	return response.json() as Promise<T>;
}

async function requestJSONWithInit<T>(path: string, init: RequestInit): Promise<T> {
	const response = await fetch(path, init);
	if (!response.ok) {
		const body = await response.json().catch(() => ({ error: response.statusText }));
		throw new Error(body.error || response.statusText);
	}
	return response.json() as Promise<T>;
}

export function searchTerms(params: SearchParams): Promise<SearchResponse> {
	const query = new URLSearchParams();
	for (const [key, value] of Object.entries(params)) {
		if (Array.isArray(value)) {
			for (const item of value) {
				if (item !== '') query.append(key, item);
			}
		} else if (value !== undefined && value !== '') {
			query.set(key, String(value));
		}
	}
	return requestJSON<SearchResponse>(`/api/v1/terms/search?${query.toString()}`);
}

export function getTerm(loincNum: string): Promise<Term> {
	return requestJSON<Term>(`/api/v1/terms/${encodeURIComponent(loincNum)}`);
}

export function getTermRelationships(loincNum: string): Promise<TermRelationshipGraph> {
	return requestJSON<TermRelationshipGraph>(`/api/v1/terms/${encodeURIComponent(loincNum)}/relationships`);
}

export function getFacets(): Promise<Facets> {
	return requestJSON<Facets>('/api/facets');
}

export function getCacheStats(): Promise<CacheStats> {
	return requestJSON<CacheStats>('/api/cache');
}

export function getVersion(): Promise<VersionInfo> {
	return requestJSON<VersionInfo>('/api/version');
}

export function getOfficialCredentialStatus(): Promise<OfficialCredentialStatus> {
	return requestJSON<OfficialCredentialStatus>('/api/v1/official/credentials/status');
}

export function deleteOfficialCredentials(): Promise<OfficialCredentialStatus> {
	return requestJSONWithInit<OfficialCredentialStatus>('/api/v1/official/credentials', { method: 'DELETE' });
}

export function officialSearch(request: OfficialSearchRequest): Promise<OfficialSearchResponse> {
	return requestJSONWithInit<OfficialSearchResponse>('/api/v1/official/search', {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify(request),
	});
}

export function getLocalSearchStatus(): Promise<LocalSearchStatus> {
	return requestJSON<LocalSearchStatus>('/api/v1/local-search/status');
}

export function rebuildLocalSearch(): Promise<LocalSearchStatus> {
	return requestJSONWithInit<LocalSearchStatus>('/api/v1/local-search/rebuild', { method: 'POST' });
}

export function localLuceneSearch(request: LocalSearchRequest): Promise<LocalSearchResponse> {
	return requestJSONWithInit<LocalSearchResponse>('/api/v1/local-search/query', {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify(request),
	});
}

export function browseAccessories(params: { kind?: string; q?: string; limit?: number; offset?: number }): Promise<AccessoryBrowseResponse> {
	const query = new URLSearchParams();
	for (const [key, value] of Object.entries(params)) {
		if (value !== undefined && value !== '') query.set(key, String(value));
	}
	return requestJSON<AccessoryBrowseResponse>(`/api/v1/accessories?${query.toString()}`);
}

export function getHierarchyChildren(params: { parentNodeId?: string; q?: string } = {}): Promise<HierarchyChildrenResponse> {
	const query = new URLSearchParams();
	if (params.q) query.set('q', params.q);
	const suffix = query.toString() ? `?${query.toString()}` : '';
	if (params.parentNodeId) {
		return requestJSON<HierarchyChildrenResponse>(`/api/v1/hierarchy/nodes/${encodeURIComponent(params.parentNodeId)}/children${suffix}`);
	}
	return requestJSON<HierarchyChildrenResponse>(`/api/v1/hierarchy/roots${suffix}`);
}

export function getHierarchyNode(nodeId: string): Promise<HierarchyNode> {
	return requestJSON<HierarchyNode>(`/api/v1/hierarchy/nodes/${encodeURIComponent(nodeId)}`);
}

export function getHierarchyParents(nodeId: string): Promise<HierarchyNodePage> {
	return requestJSON<HierarchyNodePage>(`/api/v1/hierarchy/nodes/${encodeURIComponent(nodeId)}/parents`);
}

export async function uploadReleaseZip(file: File): Promise<UploadImportResponse> {
	const formData = new FormData();
	formData.set('releaseZip', file);
	const response = await fetch('/api/import/upload', {
		method: 'POST',
		body: formData,
	});
	if (!response.ok) {
		const body = await response.json().catch(() => ({ error: response.statusText }));
		throw new Error(body.error || response.statusText);
	}
	return response.json() as Promise<UploadImportResponse>;
}
