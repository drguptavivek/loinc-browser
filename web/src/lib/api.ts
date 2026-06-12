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
	rank: number;
};

export type SearchResponse = {
	results: SearchResult[];
	total: number;
	limit: number;
	offset: number;
	query: string;
};

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
	status: string;
	system: string;
	class: string;
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
	outgoingMapTo: MapTo[];
	incomingMapTo: MapTo[];
	sharedConcepts: RelationshipConcept[];
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
	query: string;
	kind: string;
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
	return requestJSON<SearchResponse>(`/api/search?${query.toString()}`);
}

export function getTerm(loincNum: string, includeRelationships = false): Promise<Term> {
	const suffix = includeRelationships ? '?include=relationships' : '';
	return requestJSON<Term>(`/api/terms/${encodeURIComponent(loincNum)}${suffix}`);
}

export function getTermRelationships(loincNum: string): Promise<TermRelationshipGraph> {
	return requestJSON<TermRelationshipGraph>(`/api/terms/${encodeURIComponent(loincNum)}/relationships`);
}

export function getFacets(): Promise<Facets> {
	return requestJSON<Facets>('/api/facets');
}

export function getCacheStats(): Promise<CacheStats> {
	return requestJSON<CacheStats>('/api/cache');
}

export function browseAccessories(params: { kind?: string; q?: string; limit?: number; offset?: number }): Promise<AccessoryBrowseResponse> {
	const query = new URLSearchParams();
	for (const [key, value] of Object.entries(params)) {
		if (value !== undefined && value !== '') query.set(key, String(value));
	}
	return requestJSON<AccessoryBrowseResponse>(`/api/accessories?${query.toString()}`);
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
