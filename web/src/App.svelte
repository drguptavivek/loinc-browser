<script lang="ts">
	import { onMount } from 'svelte';
	import {
		BookOpen,
		ChevronDown,
		ChevronRight,
		Database,
		FilterX,
		Maximize2,
		Menu,
		Minimize2,
		Network,
		PanelLeftClose,
		PanelLeftOpen,
		RefreshCcw,
		Search,
		Server,
		Upload,
		X,
	} from '@lucide/svelte';
	import Badge from '$lib/components/Badge.svelte';
	import Button from '$lib/components/Button.svelte';
	import DetailField from '$lib/components/DetailField.svelte';
	import EmptyState from '$lib/components/EmptyState.svelte';
	import FacetGroup from '$lib/components/FacetGroup.svelte';
	import FilterChip from '$lib/components/FilterChip.svelte';
	import HierarchyTree from '$lib/components/HierarchyTree.svelte';
	import MultiSelectDropdown from '$lib/components/MultiSelectDropdown.svelte';
	import RelationshipGraph from '$lib/components/RelationshipGraph.svelte';
	import * as Resizable from '$lib/components/ui/resizable';
	import type { PaneAPI } from 'paneforge';
	import {
		browseAccessories,
		getCacheStats,
		getFacets,
		getHierarchyNode,
		getHierarchyParents,
		getTerm,
		getTermRelationships,
		getVersion,
		searchTerms,
		uploadReleaseZip,
		type AccessoryBrowseResponse,
		type AccessoryRecord,
		type CacheStats,
		type Facets,
		type SearchResult,
		type Term,
		type TermAccessory,
		type RelationshipConcept,
		type TermRelationshipGraph,
		type HierarchyNode,
		type TermSummary,
		type VersionInfo,
	} from '$lib/api';

	type BrowseMode = 'hierarchy' | 'facets' | 'rank' | 'relationships';

	const emptyFacets: Facets = {
		classes: {},
		statuses: {},
		systems: {},
		timeAspects: {},
		scales: {},
		methods: {},
		properties: {},
		orderObs: {},
	};

	let query = '';
	let selectedClass = '';
	let system = '';
	let property = '';
	let statuses: string[] = [];
	let timeAspects: string[] = [];
	let scales: string[] = [];
	let methods: string[] = [];
	let orderObsValues: string[] = [];
	let rankedOnly = false;
	let hierarchyNodeId = '';
	let hierarchyLabel = '';
	let results: SearchResult[] = [];
	let total = 0;
	let facets: Facets = emptyFacets;
	let cacheStats: CacheStats | null = null;
	let versionInfo: VersionInfo | null = null;
	let selectedTerm: Term | null = null;
	let relationshipGraph: TermRelationshipGraph | null = null;
	let loading = false;
	let facetsLoading = false;
	let termLoading = false;
	let relationshipsLoading = false;
	let relationshipsLoaded = false;
	let importLoading = false;
	let importMessage = '';
	let importError = '';
	let selectedZip: File | null = null;
	let initialTerm = '';
	let error = '';
	let offset = 0;
	let activeView: 'browse' | 'loader' | 'accessories' | 'hierarchy' = 'browse';
	let detailOpen = false;
	let sharedConceptsOpen = false;
	let graphViewerOpen = false;
	let graphVisibleConceptLimit = 8;
	let facetsCollapsed = false;
	let browsePane: PaneAPI | null = null;
	let browseDrawerOpen = false;
	let mobileBrowseMenuOpen = false;
	let mobileFiltersOpen = false;
	let resultsFullscreen = false;
	let hierarchyPath = '';
	let hierarchyPathNodeId = '';
	let hierarchyPathRequest = 0;
	let currentBrowseMode: BrowseMode = 'facets';
	let sidePanelHeading = 'Browse facets';
	let columnWidths = {
		loinc: 96,
		name: 520,
		status: 112,
		rank: 96,
		axes: 220,
	};
	let resizingColumn: keyof typeof columnWidths | null = null;
	let tableResizeStartX = 0;
	let tableResizeStartWidth = 0;
	let accessoryKind = 'part-primary';
	let accessoryQuery = '';
	let accessoryOffset = 0;
	let accessoryLoading = false;
	let accessoryResults: AccessoryRecord[] = [];
	let accessoryTotal = 0;

	const limit = 50;
	const accessoryLimit = 50;
	const hierarchyHomeNodeId = '1';
	const hierarchyHomeLabel = '{component}';
	const accessoryKinds = [
		{ value: 'part-primary', label: 'Primary parts' },
		{ value: 'part-supplementary', label: 'Supplementary parts' },
		{ value: 'answer-list', label: 'Answer lists' },
		{ value: 'panel-membership', label: 'Panel membership' },
		{ value: 'panel-child', label: 'Panel children' },
		{ value: 'group', label: 'Groups' },
		{ value: 'hierarchy', label: 'Hierarchy' },
	];
	onMount(() => {
		void (async () => {
			applyURLState();
			void loadVersion();
			await loadFacets();
			if (activeView === 'accessories' || activeView === 'hierarchy') {
				await loadAccessories(accessoryOffset, true);
			} else if (activeView === 'loader') {
				await runSearch(offset, true, false);
				updateURL(true);
			} else {
				await runSearch(offset, true);
			}
			if (initialTerm) {
				await openTerm(initialTerm, true);
			}
		})();

		const handlePopState = async () => {
			applyURLState();
			if (activeView === 'accessories' || activeView === 'hierarchy') {
				await loadAccessories(accessoryOffset, true);
			} else if (activeView === 'loader') {
				await runSearch(offset, true, false);
			} else {
				await runSearch(offset, true);
			}
			if (initialTerm) {
				await openTerm(initialTerm, true);
			} else {
				selectedTerm = null;
				detailOpen = false;
			}
			};

			window.addEventListener('popstate', handlePopState);
			window.addEventListener('pointermove', handleTableResize);
			window.addEventListener('pointerup', stopTableResize);
			return () => {
				window.removeEventListener('popstate', handlePopState);
				window.removeEventListener('pointermove', handleTableResize);
				window.removeEventListener('pointerup', stopTableResize);
		};
	});

	$: if (hierarchyNodeId && hierarchyNodeId !== hierarchyPathNodeId) {
		void loadHierarchyPath(hierarchyNodeId);
	}

	$: if (!hierarchyNodeId && hierarchyPathNodeId) {
		hierarchyPath = '';
		hierarchyPathNodeId = '';
	}

	$: currentBrowseMode =
		activeView === 'hierarchy'
			? 'hierarchy'
			: activeView === 'accessories'
				? 'relationships'
				: hierarchyNodeId && activeView === 'browse'
					? 'hierarchy'
					: rankedOnly && activeView === 'browse'
						? 'rank'
						: 'facets';

	$: sidePanelHeading =
		currentBrowseMode === 'hierarchy'
			? 'Browse hierarchy'
			: currentBrowseMode === 'rank'
				? 'Browse rank'
				: currentBrowseMode === 'relationships'
					? 'Browse relationships'
					: 'Browse facets';

	function applyURLState() {
		const params = new URLSearchParams(window.location.search);
		const hasRouteState = Array.from(params.keys()).some((key) => key !== 'mode');
		query = params.get('q') ?? '';
		selectedClass = params.get('class') ?? '';
		system = params.get('system') ?? '';
		property = params.get('property') ?? '';
		statuses = params.getAll('status');
		timeAspects = params.getAll('timeAspect');
		scales = params.getAll('scale');
		methods = params.getAll('method');
		orderObsValues = params.getAll('orderObs');
		rankedOnly = params.get('rankedOnly') === 'true' || params.get('rankedOnly') === '1';
		hierarchyNodeId = params.get('hierarchyNodeId') ?? params.get('hierarchy') ?? '';
		hierarchyLabel = params.get('hierarchyLabel') ?? '';
		offset = Number(params.get('offset') ?? '0') || 0;
		const mode = params.get('mode') ?? '';
		const browse = params.get('browse') ?? '';
		const type = params.get('type') ?? '';
		accessoryOffset = Number(params.get('browseOffset') ?? '0') || 0;
		accessoryQuery = browse;
		if (mode === 'hierarchy') {
			accessoryKind = 'hierarchy';
			if (!hierarchyNodeId && !browse) {
				hierarchyNodeId = hierarchyHomeNodeId;
				hierarchyLabel = hierarchyHomeLabel;
			}
			activeView = hierarchyNodeId ? 'browse' : 'hierarchy';
		} else if (mode === 'relationships') {
			activeView = 'accessories';
			accessoryKind = type || 'part-primary';
		} else if (mode === 'loader') {
			activeView = 'loader';
		} else {
			activeView = 'browse';
			if (mode === 'rank') rankedOnly = true;
			if (mode === 'facets') rankedOnly = false;
			if (!mode && !hasRouteState) {
				hierarchyNodeId = hierarchyHomeNodeId;
				hierarchyLabel = hierarchyHomeLabel;
			}
		}
		initialTerm = params.get('term') ?? '';
	}

	function updateURL(replace = true) {
		const params = new URLSearchParams();
		const mode = activeBrowseMode();
		if (activeView === 'loader') params.set('mode', 'loader');
		else params.set('mode', mode);
		if (query.trim()) params.set('q', query.trim());
		if (selectedClass) params.set('class', selectedClass);
		if (system) params.set('system', system);
		if (property) params.set('property', property);
		for (const value of statuses) params.append('status', value);
		for (const value of timeAspects) params.append('timeAspect', value);
		for (const value of scales) params.append('scale', value);
		for (const value of methods) params.append('method', value);
		for (const value of orderObsValues) params.append('orderObs', value);
		if (rankedOnly) params.set('rankedOnly', 'true');
		if (hierarchyNodeId) params.set('hierarchyNodeId', hierarchyNodeId);
		if (hierarchyLabel) params.set('hierarchyLabel', hierarchyLabel);
		if (activeView === 'accessories') {
			params.set('type', accessoryKind);
			if (accessoryQuery.trim()) params.set('browse', accessoryQuery.trim());
			if (accessoryOffset > 0) params.set('browseOffset', String(accessoryOffset));
		}
		if (activeView === 'hierarchy') {
			if (accessoryQuery.trim()) params.set('browse', accessoryQuery.trim());
			if (accessoryOffset > 0) params.set('browseOffset', String(accessoryOffset));
		}
		if (offset > 0) params.set('offset', String(offset));
		if (selectedTerm?.loincNum) params.set('term', selectedTerm.loincNum);
		const nextURL = `${window.location.pathname}${params.toString() ? `?${params.toString()}` : ''}`;
		if (replace) {
			window.history.replaceState(null, '', nextURL);
		} else {
			window.history.pushState(null, '', nextURL);
		}
	}

	async function loadFacets() {
		facetsLoading = true;
		try {
			facets = await getFacets();
			cacheStats = await getCacheStats();
		} catch (err) {
			error = errorMessage(err);
		} finally {
			facetsLoading = false;
		}
	}

	async function loadVersion() {
		try {
			versionInfo = await getVersion();
		} catch {
			versionInfo = null;
		}
	}

	async function runSearch(nextOffset = 0, replaceURL = false, switchToBrowse = true) {
		loading = true;
		error = '';
		if (switchToBrowse) activeView = 'browse';
		offset = nextOffset;
		if (nextOffset === 0 && selectedTerm) {
			selectedTerm = null;
			relationshipGraph = null;
			relationshipsLoaded = false;
			detailOpen = false;
		}
		try {
			const response = await searchTerms({
				q: query.trim(),
				class: selectedClass,
				system,
				property,
				status: statuses,
				timeAspect: timeAspects,
				scale: scales,
				method: methods,
				orderObs: orderObsValues,
				rankedOnly,
				hierarchyNodeId,
				limit,
				offset,
			});
			results = response.results;
			total = response.total;
			updateURL(replaceURL);
		} catch (err) {
			error = errorMessage(err);
		} finally {
			loading = false;
		}
	}

	async function openTerm(loincNum: string, replaceURL = false) {
		termLoading = true;
		detailOpen = true;
		graphViewerOpen = false;
		sharedConceptsOpen = false;
		graphVisibleConceptLimit = 8;
		relationshipGraph = null;
		relationshipsLoaded = false;
		relationshipsLoading = false;
		error = '';
		try {
			selectedTerm = await getTerm(loincNum);
			cacheStats = await getCacheStats();
			updateURL(replaceURL);
		} catch (err) {
			error = errorMessage(err);
		} finally {
			termLoading = false;
		}
	}

	async function loadRelationshipDetails() {
		if (!selectedTerm || relationshipsLoaded || relationshipsLoading) return;
		relationshipsLoading = true;
		error = '';
		try {
			const graph = await getTermRelationships(selectedTerm.loincNum);
			selectedTerm = {
				...selectedTerm,
				mapTo: graph.mapTo ?? graph.outgoingMapTo ?? [],
				parts: graph.parts ?? [],
				answerLists: graph.answerLists ?? [],
				panels: [...(graph.panelMemberships ?? []), ...(graph.panelItems ?? [])],
				groups: graph.groups ?? [],
				hierarchy: graph.hierarchy ?? [],
			};
			relationshipGraph = {
				...graph,
				outgoingMapTo: graph.mapTo ?? graph.outgoingMapTo ?? [],
				incomingMapTo: graph.mappedFrom ?? graph.incomingMapTo ?? [],
				sharedConcepts: graph.sharedConcepts ?? [],
			};
			relationshipsLoaded = true;
			cacheStats = await getCacheStats();
		} catch (err) {
			error = errorMessage(err);
		} finally {
			relationshipsLoading = false;
		}
	}

	async function openGraphViewer() {
		await loadRelationshipDetails();
		if (selectedTerm && relationshipGraph) {
			graphViewerOpen = true;
		}
	}

	async function toggleSharedConcepts() {
		if (!relationshipsLoaded) {
			await loadRelationshipDetails();
			sharedConceptsOpen = true;
			return;
		}
		sharedConceptsOpen = !sharedConceptsOpen;
	}

	function closeTerm() {
		selectedTerm = null;
		relationshipGraph = null;
		relationshipsLoaded = false;
		detailOpen = false;
		graphViewerOpen = false;
		updateURL();
	}

	async function importSelectedZip() {
		if (!selectedZip) {
			importError = 'Choose a LOINC release zip first.';
			return;
		}
		importLoading = true;
		importError = '';
		importMessage = '';
		try {
			const response = await uploadReleaseZip(selectedZip);
			importMessage = `Imported ${response.termCount.toLocaleString()} terms.`;
			selectedZip = null;
			selectedTerm = null;
			relationshipGraph = null;
			relationshipsLoaded = false;
			detailOpen = false;
			offset = 0;
			await Promise.all([loadFacets(), runSearch(0)]);
			activeView = 'loader';
		} catch (err) {
			importError = errorMessage(err);
		} finally {
			importLoading = false;
		}
	}

	function clearFilters() {
		selectedClass = '';
		system = '';
		property = '';
		statuses = [];
		timeAspects = [];
		scales = [];
		methods = [];
		orderObsValues = [];
		rankedOnly = false;
		hierarchyNodeId = '';
		hierarchyLabel = '';
		runSearch(0);
	}

	function browseByRank() {
		query = '';
		selectedClass = '';
		system = '';
		property = '';
		statuses = [];
		timeAspects = [];
		scales = [];
		methods = [];
		orderObsValues = [];
		hierarchyNodeId = '';
		hierarchyLabel = '';
		rankedOnly = true;
		activeView = 'browse';
		runSearch(0);
	}

	function openFacetBrowser() {
		rankedOnly = false;
		hierarchyNodeId = '';
		hierarchyLabel = '';
		activeView = 'browse';
		resultsFullscreen = false;
		runSearch(0);
	}

	function activeBrowseMode() {
		if (activeView === 'hierarchy') return 'hierarchy';
		if (activeView === 'accessories') return 'relationships';
		if (hierarchyNodeId && activeView === 'browse') return 'hierarchy';
		if (rankedOnly && activeView === 'browse') return 'rank';
		return 'facets';
	}

	function modeButtonClass(mode: BrowseMode, activeMode: BrowseMode) {
		const active = activeMode === mode;
		return [
			'inline-flex h-8 shrink-0 items-center justify-center gap-2 rounded-md px-3 text-xs font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-400',
			active ? 'bg-zinc-950 text-white shadow-sm hover:bg-zinc-800' : 'border border-zinc-200 bg-white text-zinc-900 hover:bg-zinc-50',
		].join(' ');
	}

	function sidePanelTitle() {
		const mode = activeBrowseMode();
		if (mode === 'hierarchy') return 'Browse hierarchy';
		if (mode === 'rank') return 'Browse rank';
		if (mode === 'relationships') return 'Browse relationships';
		return 'Browse facets';
	}

	function resetAll() {
		query = '';
		clearFilters();
		selectedTerm = null;
		relationshipGraph = null;
		relationshipsLoaded = false;
		detailOpen = false;
	}

	function goHome() {
		selectedTerm = null;
		relationshipGraph = null;
		relationshipsLoaded = false;
		detailOpen = false;
		browseDrawerOpen = false;
		openHierarchyHome(false);
	}

	function chooseFacet(kind: 'status' | 'class' | 'system' | 'scale' | 'property' | 'orderObs', value: string) {
		if (kind === 'class') selectedClass = selectedClass === value ? '' : value;
		if (kind === 'system') system = system === value ? '' : value;
		if (kind === 'property') property = property === value ? '' : value;
		runSearch(0);
	}

	function clearFacet(kind: 'status' | 'class' | 'system' | 'timeAspect' | 'scale' | 'method' | 'property' | 'orderObs' | 'hierarchy', value = '') {
		if (kind === 'class') selectedClass = '';
		if (kind === 'system') system = '';
		if (kind === 'property') property = '';
		if (kind === 'hierarchy') {
			hierarchyNodeId = '';
			hierarchyLabel = '';
		}
		if (kind === 'status') statuses = value ? statuses.filter((item) => item !== value) : [];
		if (kind === 'timeAspect') timeAspects = value ? timeAspects.filter((item) => item !== value) : [];
		if (kind === 'scale') scales = value ? scales.filter((item) => item !== value) : [];
		if (kind === 'method') methods = value ? methods.filter((item) => item !== value) : [];
		if (kind === 'orderObs') orderObsValues = value ? orderObsValues.filter((item) => item !== value) : [];
		runSearch(0);
	}

	function toggleMulti(kind: 'status' | 'timeAspect' | 'scale' | 'method' | 'orderObs', value: string) {
		if (kind === 'status') statuses = toggleValue(statuses, value);
		if (kind === 'timeAspect') timeAspects = toggleValue(timeAspects, value);
		if (kind === 'scale') scales = toggleValue(scales, value);
		if (kind === 'method') methods = toggleValue(methods, value);
		if (kind === 'orderObs') orderObsValues = toggleValue(orderObsValues, value);
		runSearch(0);
	}

	function toggleValue(values: string[], value: string) {
		return values.includes(value) ? values.filter((item) => item !== value) : [...values, value];
	}

	function activeFilterCount() {
		return [selectedClass, system, property, rankedOnly ? 'rankedOnly' : '', hierarchyNodeId, ...statuses, ...timeAspects, ...scales, ...methods, ...orderObsValues].filter(Boolean).length;
	}

	function cachedEntryCount() {
		if (!cacheStats) return 0;
		return cacheStats.termEntries + cacheStats.relationshipEntries + cacheStats.accessoryEntries + cacheStats.facetEntries;
	}

	function resultsTableWidth() {
		return columnWidths.loinc + columnWidths.name + columnWidths.status + columnWidths.rank + columnWidths.axes;
	}

	function facetEntries(values: Record<string, number> | null | undefined) {
		return Object.entries(values ?? {});
	}

	function hasFacetChoices(values: Record<string, number> | null | undefined, selected: string[] = []) {
		return Object.keys(values ?? {}).length > 0 || selected.length > 0;
	}

	function statusVariant(value: string) {
		if (value === 'ACTIVE') return 'default';
		if (value === 'DISCOURAGED') return 'warning';
		return 'secondary';
	}

	function errorMessage(err: unknown) {
		return err instanceof Error ? err.message : String(err);
	}

	function startColumnResize(column: keyof typeof columnWidths, event: PointerEvent) {
		event.preventDefault();
		event.stopPropagation();
		resizingColumn = column;
		tableResizeStartX = event.clientX;
		tableResizeStartWidth = columnWidths[column];
	}

	function handleTableResize(event: PointerEvent) {
		if (!resizingColumn) return;
		const minimums: Record<keyof typeof columnWidths, number> = {
			loinc: 78,
			name: 220,
			status: 88,
			rank: 74,
			axes: 140,
		};
		const nextWidth = Math.max(minimums[resizingColumn], tableResizeStartWidth + event.clientX - tableResizeStartX);
		columnWidths = { ...columnWidths, [resizingColumn]: nextWidth };
	}

	function stopTableResize() {
		resizingColumn = null;
	}

	function openLoader() {
		activeView = 'loader';
		resultsFullscreen = false;
		updateURL(false);
	}

	async function openAccessoryBrowser(kind = accessoryKind, q = '', replaceURL = false) {
		accessoryKind = kind;
		accessoryQuery = q;
		accessoryOffset = 0;
		activeView = 'accessories';
		resultsFullscreen = false;
		await loadAccessories(0, replaceURL);
	}

	async function openHierarchyBrowser(q = '', replaceURL = false) {
		if (!q.trim()) {
			openHierarchyHome(replaceURL);
			return;
		}
		accessoryKind = 'hierarchy';
		accessoryQuery = q;
		accessoryOffset = 0;
		activeView = 'hierarchy';
		resultsFullscreen = false;
		await loadAccessories(0, replaceURL);
	}

	function openHierarchyHome(replaceURL = false) {
		query = '';
		selectedClass = '';
		system = '';
		property = '';
		statuses = [];
		timeAspects = [];
		scales = [];
		methods = [];
		orderObsValues = [];
		hierarchyNodeId = hierarchyHomeNodeId;
		hierarchyLabel = hierarchyHomeLabel;
		rankedOnly = false;
		activeView = 'browse';
		resultsFullscreen = false;
		void runSearch(0, replaceURL, false);
	}

	async function loadAccessories(nextOffset = accessoryOffset, replaceURL = true) {
		accessoryLoading = true;
		error = '';
		accessoryOffset = nextOffset;
		try {
			const response: AccessoryBrowseResponse = await browseAccessories({
				kind: accessoryKind,
				q: accessoryQuery.trim(),
				limit: accessoryLimit,
				offset: accessoryOffset,
			});
			accessoryResults = response.results;
			accessoryTotal = response.total;
			updateURL(replaceURL);
		} catch (err) {
			error = errorMessage(err);
		} finally {
			accessoryLoading = false;
		}
	}

	function accessoryTitle(item: TermAccessory) {
		return item.title || item.code || item.kind;
	}

	function hasAccessories(term: Term) {
		return Boolean(
			term.mapTo?.length ||
				term.parts?.length ||
				term.answerLists?.length ||
				term.panels?.length ||
				term.groups?.length ||
				term.hierarchy?.length,
		);
	}

	function hasRelationshipGraph(graph: TermRelationshipGraph | null) {
		return Boolean(graph?.outgoingMapTo?.length || graph?.incomingMapTo?.length || graph?.sharedConcepts?.length);
	}

	function accessorySections(term: Term): { title: string; kind: string; items: TermAccessory[] }[] {
		return [
			{ title: 'Parts', kind: 'part-primary', items: term.parts ?? [] },
			{ title: 'Answer lists', kind: 'answer-list', items: term.answerLists ?? [] },
			{ title: 'Panels and forms', kind: 'panel-membership', items: term.panels ?? [] },
			{ title: 'Groups', kind: 'group', items: term.groups ?? [] },
			{ title: 'Hierarchy', kind: 'hierarchy', items: term.hierarchy ?? [] },
		].filter((section) => section.items.length > 0);
	}

	function browseAccessoryForSelected(kind: string) {
		if (!selectedTerm) return;
		if (kind === 'hierarchy') {
			void openHierarchyBrowser(selectedTerm.loincNum);
			return;
		}
		void openAccessoryBrowser(kind, selectedTerm.loincNum);
	}

	function browseConcept(concept: { kind: string; code: string }) {
		void openAccessoryBrowser(concept.kind, concept.code);
	}

	function isLoincNumber(value: string) {
		return /^\d+-\d$/.test(value);
	}

	function openAccessoryRow(item: AccessoryRecord) {
		if (isLoincNumber(item.loincNum)) {
			void openTerm(item.loincNum);
			return;
		}
		accessoryQuery = item.loincNum || item.code || accessoryTitle(item);
		void loadAccessories(0, false);
	}

	function browseHierarchyNode(node: HierarchyNode) {
		query = '';
		hierarchyNodeId = node.nodeId;
		hierarchyLabel = node.label || node.code;
		rankedOnly = false;
		activeView = 'browse';
		void runSearch(0, false, false);
	}

	async function loadHierarchyPath(nodeId: string) {
		const requestId = ++hierarchyPathRequest;
		hierarchyPathNodeId = nodeId;
		hierarchyPath = hierarchyLabel || nodeId;
		try {
			const [node, parents] = await Promise.all([getHierarchyNode(nodeId), getHierarchyParents(nodeId)]);
			if (requestId !== hierarchyPathRequest) return;
			const pathNodes = [...(parents.results ?? []), node];
			const labels = pathNodes.map(hierarchyPathNodeLabel).filter(Boolean);
			hierarchyPath = labels.length ? labels.join(' / ') : hierarchyLabel || nodeId;
			hierarchyLabel = hierarchyPathNodeLabel(node) || hierarchyLabel;
		} catch {
			if (requestId === hierarchyPathRequest) {
				hierarchyPath = hierarchyLabel || nodeId;
			}
		}
	}

	function hierarchyPathNodeLabel(node: HierarchyNode) {
		return node.label || node.code || node.nodeId;
	}

	function chooseMobileBrowseMode(mode: 'hierarchy' | 'facets' | 'rank' | 'relationships') {
		mobileBrowseMenuOpen = false;
		if (mode === 'hierarchy') {
			void openHierarchyBrowser();
		} else if (mode === 'facets') {
			openFacetBrowser();
		} else if (mode === 'rank') {
			browseByRank();
		} else {
			void openAccessoryBrowser();
		}
	}

	function termSummaryLabel(term: TermSummary) {
		return term.longCommonName || term.shortName || term.loincNum;
	}

	function sharedConcepts() {
		return relationshipGraph?.sharedConcepts ?? [];
	}

	function openBrowseDrawer() {
		facetsCollapsed = false;
		browseDrawerOpen = true;
	}

	function closeBrowseDrawer() {
		browseDrawerOpen = false;
	}

	function collapseFacetPane() {
		facetsCollapsed = true;
		browsePane?.collapse();
	}

	function expandFacetPane() {
		facetsCollapsed = false;
		browsePane?.expand();
	}

	function handleFacetPaneCollapse() {
		facetsCollapsed = true;
	}

	function handleFacetPaneExpand() {
		facetsCollapsed = false;
	}
</script>

<main class="min-h-screen bg-zinc-50 pb-12 text-zinc-950 lg:flex lg:h-screen lg:flex-col lg:overflow-hidden">
	<header class="border-b border-zinc-200 bg-white lg:shrink-0">
		<div class="mx-auto flex max-w-[1500px] flex-wrap items-center justify-between gap-4 px-5 py-4">
			<button type="button" class="flex items-center gap-3 rounded-md text-left hover:bg-zinc-50 focus:outline-none focus:ring-2 focus:ring-zinc-200" aria-label="Go to hierarchy home" on:click={goHome}>
				<div class="flex size-10 items-center justify-center rounded-md bg-zinc-950 text-white">
					<BookOpen size={20} />
				</div>
				<div>
					<h1 class="text-lg font-semibold tracking-normal">LOINC Browser</h1>
				</div>
			</button>
			<div class="relative md:hidden">
				<Button variant="outline" size="sm" ariaLabel="Open browse mode menu" on:click={() => (mobileBrowseMenuOpen = !mobileBrowseMenuOpen)}>
					<Menu size={14} />
					Modes
				</Button>
				{#if mobileBrowseMenuOpen}
					<div class="absolute right-0 top-10 z-50 flex w-52 max-w-[calc(100vw-1.5rem)] flex-col gap-1 rounded-md border border-zinc-200 bg-white p-1.5 shadow-lg" role="menu" aria-label="Browse mode menu">
						<button type="button" class={`flex items-center gap-2 whitespace-nowrap rounded px-2 py-1.5 text-left text-xs ${currentBrowseMode === 'hierarchy' ? 'bg-zinc-950 text-white' : 'text-zinc-700 hover:bg-zinc-100'}`} on:click={() => chooseMobileBrowseMode('hierarchy')}>
							<Network size={14} />
							Hierarchy
						</button>
						<button type="button" class={`flex items-center gap-2 whitespace-nowrap rounded px-2 py-1.5 text-left text-xs ${currentBrowseMode === 'facets' ? 'bg-zinc-950 text-white' : 'text-zinc-700 hover:bg-zinc-100'}`} on:click={() => chooseMobileBrowseMode('facets')}>
							<FilterX size={14} />
							Facets
						</button>
						<button type="button" class={`flex items-center gap-2 whitespace-nowrap rounded px-2 py-1.5 text-left text-xs ${currentBrowseMode === 'rank' ? 'bg-zinc-950 text-white' : 'text-zinc-700 hover:bg-zinc-100'}`} on:click={() => chooseMobileBrowseMode('rank')}>
							<Database size={14} />
							Rank
						</button>
						<button type="button" class={`flex items-center gap-2 whitespace-nowrap rounded px-2 py-1.5 text-left text-xs ${currentBrowseMode === 'relationships' ? 'bg-zinc-950 text-white' : 'text-zinc-700 hover:bg-zinc-100'}`} on:click={() => chooseMobileBrowseMode('relationships')}>
							<Database size={14} />
							Relationships
						</button>
					</div>
				{/if}
			</div>
			<div class="hidden flex-wrap gap-2 md:flex" role="tablist" aria-label="Browse mode">
				<button type="button" class={modeButtonClass('hierarchy', currentBrowseMode)} on:click={() => { void openHierarchyBrowser(); }}>
					<Network size={14} />
					Hierarchy
				</button>
				<button type="button" class={modeButtonClass('facets', currentBrowseMode)} on:click={openFacetBrowser}>
					<FilterX size={14} />
					Facets
				</button>
				<button type="button" class={modeButtonClass('rank', currentBrowseMode)} on:click={browseByRank}>
					<Database size={14} />
					Rank
				</button>
				<button type="button" class={modeButtonClass('relationships', currentBrowseMode)} on:click={() => { void openAccessoryBrowser(); }}>
					<Database size={14} />
					Relationships
				</button>
			</div>
			<div class="hidden items-center gap-3 text-sm text-zinc-500 md:flex">
				<div class="flex items-center gap-2 rounded-md border border-zinc-200 px-3 py-2">
					<Database size={16} />
					<span>{total.toLocaleString()} matches</span>
				</div>
				<div class="flex items-center gap-2 rounded-md border border-zinc-200 px-3 py-2">
					<Server size={16} />
					<span>{cacheStats ? `${cachedEntryCount()} cached` : 'cache ready'}</span>
				</div>
			</div>
		</div>
	</header>

	<div class="mx-auto flex w-full max-w-[1500px] flex-col gap-5 px-5 py-5 lg:min-h-0 lg:flex-1 lg:gap-0 lg:overflow-hidden">
		<Button variant="outline" size="sm" className="fixed left-3 top-[77px] z-40 w-fit shadow-lg lg:hidden" ariaLabel="Open browse drawer" on:click={openBrowseDrawer}>
			<PanelLeftOpen size={14} />
			Browse
		</Button>

		{#if browseDrawerOpen}
			<button
				type="button"
				class="fixed inset-0 z-[60] bg-zinc-950/30 lg:hidden"
				aria-label="Close browse drawer"
				data-testid="browse-drawer-backdrop"
				on:click={closeBrowseDrawer}
			></button>
		{/if}

		<Resizable.PaneGroup
			direction="horizontal"
			keyboardResizeBy={4}
			class="w-full lg:min-h-0 lg:flex-1"
			data-loinc-shell-pane-group
			data-testid="desktop-resizable-pane-group"
		>
			<Resizable.Pane
				id="browse-pane"
				bind:this={browsePane}
				defaultSize={25}
				minSize={16}
				maxSize={45}
				collapsible
				collapsedSize={4}
				order={1}
				onCollapse={handleFacetPaneCollapse}
				onExpand={handleFacetPaneExpand}
				class="lg:min-h-0"
				data-loinc-shell-pane
				data-testid="browse-resizable-pane"
			>
				<aside
					class={`facet-pane fixed inset-y-0 left-0 z-[70] flex flex-col gap-4 bg-zinc-50 p-3 shadow-xl transition-transform duration-200 ease-out lg:static lg:z-auto lg:h-full lg:min-h-0 lg:w-full lg:translate-x-0 lg:bg-transparent lg:p-0 lg:shadow-none ${browseDrawerOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'}`}
				>
					<section class="flex h-full min-h-0 flex-col overflow-hidden rounded-lg border border-zinc-200 bg-white">
				<div class="flex shrink-0 items-center justify-between border-b border-zinc-200 px-4 py-3">
					{#if facetsCollapsed}
						<button
							type="button"
							class="inline-flex size-8 items-center justify-center rounded-md text-zinc-700 hover:bg-zinc-100 hover:text-zinc-950"
							aria-label="Expand facets"
							on:click={expandFacetPane}
						>
							<PanelLeftOpen size={16} />
						</button>
					{:else}
						<h2 class="text-sm font-semibold">{sidePanelHeading}</h2>
						<div class="flex items-center gap-1">
							{#if activeFilterCount() > 0}
								<Button variant="ghost" size="sm" on:click={clearFilters}>
									<FilterX size={14} />
									Clear
								</Button>
							{/if}
							<button
								type="button"
								class="inline-flex size-8 items-center justify-center rounded-md text-zinc-700 hover:bg-zinc-100 hover:text-zinc-950 lg:hidden"
								aria-label="Close browse drawer"
								on:click={closeBrowseDrawer}
							>
								<X size={16} />
							</button>
							<button
							type="button"
							class="hidden size-8 items-center justify-center rounded-md text-zinc-700 hover:bg-zinc-100 hover:text-zinc-950 lg:inline-flex"
							aria-label="Collapse facets"
							on:click={collapseFacetPane}
						>
							<PanelLeftClose size={16} />
						</button>
					</div>
				{/if}
				</div>
				{#if !facetsCollapsed}
					<div class="flex min-h-0 flex-1 flex-col gap-4 overflow-auto p-4" data-testid="facet-scroll-panel">
						{#if currentBrowseMode === 'hierarchy'}
							<HierarchyTree onOpenTerm={(loincNum) => openTerm(loincNum)} onBrowseNode={browseHierarchyNode} />
						{:else if currentBrowseMode === 'relationships'}
							<section class="rounded-md border border-zinc-200 bg-white">
								<div class="border-b border-zinc-100 px-2.5 py-2 text-[11px] font-semibold uppercase tracking-wide text-zinc-600">Relationship types</div>
								<div class="flex flex-col gap-1 p-2">
									{#each accessoryKinds as item}
										<button
											type="button"
											class={`rounded-md px-2 py-1.5 text-left text-xs leading-4 ${accessoryKind === item.value ? 'bg-zinc-950 text-white' : 'text-zinc-700 hover:bg-zinc-100'}`}
											on:click={() => openAccessoryBrowser(item.value)}
										>
											{item.label}
										</button>
									{/each}
								</div>
							</section>
						{:else}
							{#if currentBrowseMode === 'rank'}
								<section class="rounded-md border border-zinc-200 bg-white p-3 text-xs leading-5 text-zinc-600">
									<div class="font-semibold uppercase tracking-wide text-zinc-500">Rank mode</div>
									<p class="mt-1">Showing ranked LOINC terms first. Use facets below to narrow ranked results.</p>
								</section>
							{/if}
							{#if facetsLoading}
								<p class="text-sm text-zinc-500">Loading facets...</p>
							{:else}
								<FacetGroup title="System" entries={facetEntries(facets.systems)} active={system} kind="system" onPick={chooseFacet} />
								<FacetGroup title="Class" entries={facetEntries(facets.classes)} active={selectedClass} kind="class" onPick={chooseFacet} />
								<FacetGroup title="Property" entries={facetEntries(facets.properties)} active={property} kind="property" onPick={chooseFacet} />
							{/if}
						{/if}
					</div>
					<div class="shrink-0 border-t border-zinc-200 p-3">
						<Button variant={activeView === 'loader' ? 'default' : 'outline'} className="w-full justify-start" on:click={openLoader}>
							<Upload size={15} />
							Load release zip
						</Button>
					</div>
				{/if}
					</section>
				</aside>
			</Resizable.Pane>

			<Resizable.Handle
				withHandle
				class="hidden bg-transparent data-[direction=horizontal]:w-4 lg:flex"
				aria-label="Resize browse panel"
				title="Drag to resize browse panel"
				data-testid="facet-resize-handle"
			/>

			<Resizable.Pane
				id="results-shell-pane"
				defaultSize={75}
				minSize={55}
				order={2}
				class="lg:min-h-0"
				data-loinc-shell-pane
				data-testid="results-resizable-pane"
			>
				<section
					class={`results-pane order-1 min-w-0 rounded-lg border border-zinc-200 bg-white lg:order-2 lg:flex lg:min-h-0 lg:flex-col lg:overflow-hidden ${resultsFullscreen ? 'fixed inset-0 z-40 h-screen rounded-none' : 'relative lg:h-full'}`}
					data-testid="results-pane"
				>
					<Button
						type="button"
						variant="ghost"
						size="icon"
						className="absolute right-3 top-3 z-20 bg-white/95 shadow-sm ring-1 ring-zinc-200 hover:bg-zinc-50"
						ariaLabel={resultsFullscreen ? 'Exit fullscreen' : 'Expand results'}
						on:click={() => (resultsFullscreen = !resultsFullscreen)}
					>
						{#if resultsFullscreen}<Minimize2 size={16} />{:else}<Maximize2 size={16} />{/if}
					</Button>
			{#if activeView === 'loader'}
				<div class="flex items-center justify-between gap-4 border-b border-zinc-200 px-4 py-3 pr-14 lg:shrink-0">
					<div class="min-w-0">
						<h2 class="text-sm font-semibold">Release loader</h2>
						<p class="mt-1 text-xs text-zinc-500">Upload a licensed LOINC release zip and ingest it into the local SQLite database.</p>
					</div>
					<div class="flex items-center gap-2">
						<Button variant="outline" size="sm" on:click={openFacetBrowser}>Back to browse</Button>
					</div>
				</div>
				<div class="lg:min-h-0 lg:flex-1 lg:overflow-auto">
					<div class="mx-auto flex max-w-3xl flex-col gap-4 p-5">
						<section class="rounded-lg border border-zinc-200 bg-white p-4">
							<h3 class="text-sm font-semibold">Current data status</h3>
							<div class="mt-3 grid gap-3 text-sm sm:grid-cols-3">
								<div class="rounded-md border border-zinc-200 p-3">
									<div class="text-xs uppercase tracking-wide text-zinc-500">Searchable terms</div>
									<div class="mt-1 text-lg font-semibold">{total.toLocaleString()}</div>
								</div>
								<div class="rounded-md border border-zinc-200 p-3">
									<div class="text-xs uppercase tracking-wide text-zinc-500">Cached terms</div>
									<div class="mt-1 text-lg font-semibold">{cachedEntryCount().toLocaleString()}</div>
								</div>
								<div class="rounded-md border border-zinc-200 p-3">
									<div class="text-xs uppercase tracking-wide text-zinc-500">Import state</div>
									<div class="mt-1 text-lg font-semibold">{importLoading ? 'Ingesting' : importMessage ? 'Complete' : 'Ready'}</div>
								</div>
							</div>
						</section>

						<section class="rounded-lg border border-zinc-200 bg-white p-4">
							<form class="flex flex-col gap-4" on:submit|preventDefault={importSelectedZip}>
								<div>
									<label class="text-xs font-semibold uppercase tracking-wide text-zinc-500" for="releaseZipPage">LOINC release zip</label>
									<input
										id="releaseZipPage"
										class="mt-2 block w-full rounded-md border border-zinc-200 bg-white text-sm text-zinc-700 file:mr-3 file:border-0 file:bg-zinc-100 file:px-3 file:py-2 file:text-sm file:font-medium file:text-zinc-700"
										type="file"
										accept=".zip,application/zip"
										disabled={importLoading}
										on:change={(event) => {
											selectedZip = event.currentTarget.files?.[0] ?? null;
											importError = '';
											importMessage = '';
										}}
									/>
								</div>
								<div class="flex flex-wrap items-center gap-3">
									<Button type="submit" disabled={importLoading || !selectedZip}>
										<Upload size={16} />
										{importLoading ? 'Ingesting release...' : 'Upload and ingest'}
									</Button>
									{#if selectedZip && !importLoading}
										<span class="max-w-full truncate text-sm text-zinc-500">{selectedZip.name}</span>
									{/if}
								</div>
								{#if importMessage}
									<p class="rounded-md bg-emerald-50 px-3 py-2 text-sm text-emerald-800">{importMessage}</p>
								{/if}
								{#if importError}
									<p class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-800">{importError}</p>
								{/if}
								<p class="text-xs leading-5 text-zinc-500">
									The uploaded release is unpacked into local data storage and indexed with SQLite FTS. Existing search data is replaced only after import succeeds.
								</p>
							</form>
						</section>
					</div>
				</div>
			{:else if activeView === 'accessories' || activeView === 'hierarchy'}
				<div class="flex items-center justify-between gap-4 border-b border-zinc-200 px-4 py-3 pr-14 lg:shrink-0">
					<div class="min-w-0">
						<h2 class="text-sm font-semibold">{activeView === 'hierarchy' ? 'Hierarchy browser' : 'Relationship browser'}</h2>
						<p class="mt-1 text-xs text-zinc-500">
							{activeView === 'hierarchy'
								? 'Browse imported LOINC hierarchy paths and open linked terms.'
								: 'Browse imported LOINC accessory rows and open linked terms.'}
						</p>
					</div>
					<div class="flex items-center gap-2">
						<Button variant="outline" size="sm" on:click={openFacetBrowser}>Back to search</Button>
					</div>
				</div>
				<form class="border-b border-zinc-200 p-4 lg:shrink-0" on:submit|preventDefault={() => loadAccessories(0)}>
					<div class="grid gap-3 md:grid-cols-[220px_minmax(0,1fr)_auto]">
						<label class="text-xs font-semibold uppercase tracking-wide text-zinc-500">
							{activeView === 'hierarchy' ? 'Hierarchy' : 'Type'}
							<select class="mt-1 h-10 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm normal-case tracking-normal text-zinc-800" bind:value={accessoryKind} on:change={() => loadAccessories(0)}>
								{#if activeView === 'hierarchy'}
									<option value="hierarchy">Component hierarchy</option>
								{:else}
									{#each accessoryKinds as item}
										<option value={item.value}>{item.label}</option>
									{/each}
								{/if}
							</select>
						</label>
						<label class="text-xs font-semibold uppercase tracking-wide text-zinc-500">
							{activeView === 'hierarchy' ? 'Search hierarchy' : 'Search relationships'}
							<input
								class="mt-1 h-10 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm normal-case tracking-normal text-zinc-800 outline-none focus:border-zinc-400 focus:ring-2 focus:ring-zinc-100"
								bind:value={accessoryQuery}
								placeholder={activeView === 'hierarchy' ? 'Hierarchy label, LP code, path, or LOINC number' : 'Part, group, panel, answer list, or LOINC number'}
							/>
						</label>
						<div class="flex items-end">
							<Button type="submit" className="h-10">
								<Search size={15} />
								Search
							</Button>
						</div>
					</div>
				</form>
				{#if error}
					<div class="border-b border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800 lg:shrink-0">{error}</div>
				{/if}
				<div class="flex items-center justify-between border-b border-zinc-200 px-4 py-3 lg:shrink-0">
					<p class="text-sm text-zinc-500">
						{#if accessoryLoading}
							{activeView === 'hierarchy' ? 'Loading hierarchy...' : 'Loading relationships...'}
						{:else}
							Showing {accessoryResults.length.toLocaleString()} of {accessoryTotal.toLocaleString()} {activeView === 'hierarchy' ? 'hierarchy rows' : 'relationships'}
						{/if}
					</p>
					<div class="flex items-center gap-2">
						<Button variant="outline" size="sm" disabled={accessoryOffset === 0 || accessoryLoading} on:click={() => loadAccessories(Math.max(0, accessoryOffset - accessoryLimit))}>Previous</Button>
						<Button variant="outline" size="sm" disabled={accessoryOffset + accessoryLimit >= accessoryTotal || accessoryLoading} on:click={() => loadAccessories(accessoryOffset + accessoryLimit)}>Next</Button>
					</div>
				</div>
				<div class="lg:min-h-0 lg:flex-1 lg:overflow-auto">
					{#if accessoryLoading}
						<div class="space-y-3 p-4">
							{#each Array(8) as _}<div class="h-16 animate-pulse rounded-md bg-zinc-100"></div>{/each}
						</div>
					{:else if accessoryResults.length === 0}
						<div class="p-4">
							<EmptyState
								title={activeView === 'hierarchy' ? 'No hierarchy rows' : 'No relationships'}
								body={activeView === 'hierarchy' ? 'Try another hierarchy search term.' : 'Try another relationship type or search term.'}
							/>
						</div>
					{:else}
						<div class="divide-y divide-zinc-100">
							{#each accessoryResults as item}
								<button type="button" class="grid w-full cursor-pointer gap-3 px-4 py-3 text-left text-sm hover:bg-zinc-50 focus:bg-zinc-50 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-zinc-300 md:grid-cols-[110px_minmax(0,1fr)_120px]" on:click={() => openAccessoryRow(item)}>
									<div class="font-mono font-semibold text-zinc-950">{item.loincNum}</div>
									<div class="min-w-0">
										<div class="break-words font-medium text-zinc-950">{item.longCommonName || item.shortName || accessoryTitle(item)}</div>
										<div class="mt-1 break-words text-xs text-zinc-500">
											{#if activeView === 'hierarchy'}
												{#if item.subtitle}<span>{item.subtitle}</span>{:else}<span>{accessoryTitle(item)}</span>{/if}
											{:else}
												<span>{accessoryTitle(item)}</span>
												{#if item.subtitle}<span> · {item.subtitle}</span>{/if}
											{/if}
										</div>
									</div>
									<div class="font-mono text-xs text-zinc-500">{item.code}</div>
								</button>
							{/each}
						</div>
					{/if}
				</div>
			{:else}
				<form class="border-b border-zinc-200 px-4 py-3 pr-14 lg:shrink-0" on:submit|preventDefault={() => runSearch(0)}>
					<div class="flex flex-col gap-2 md:flex-row">
						<label class="relative min-w-0 flex-1">
							<Search class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-zinc-400" size={18} />
							<input
								class="h-10 w-full rounded-md border border-zinc-200 bg-white pl-10 pr-3 text-sm outline-none transition focus:border-zinc-400 focus:ring-2 focus:ring-zinc-100 md:h-11"
								bind:value={query}
								placeholder="Search LOINC number, long name, component, related names..."
							/>
						</label>
						<div class="grid grid-cols-2 gap-2 md:flex">
							<Button type="submit" className="h-9 md:h-11">
								<Search size={16} />
								Search
							</Button>
							<Button type="button" variant="outline" className="h-9 md:h-11" on:click={resetAll}>
								<RefreshCcw size={16} />
								Reset
							</Button>
						</div>
					</div>
					<button
						type="button"
						class="mt-2 flex w-full items-center justify-between gap-3 rounded-md border border-zinc-200 bg-zinc-50 px-3 py-2 text-left text-xs text-zinc-700 md:hidden"
						aria-expanded={mobileFiltersOpen}
						on:click={() => (mobileFiltersOpen = !mobileFiltersOpen)}
					>
						<span class="flex min-w-0 items-center gap-2">
							{#if mobileFiltersOpen}<ChevronDown size={14} class="shrink-0 text-zinc-500" />{:else}<ChevronRight size={14} class="shrink-0 text-zinc-500" />{/if}
							<span class="font-semibold">Filters</span>
						</span>
						<span class="shrink-0 rounded bg-white px-1.5 py-0.5 text-[11px] text-zinc-500">{activeFilterCount().toLocaleString()}</span>
					</button>
					<div class={`${mobileFiltersOpen ? 'mt-2 flex' : 'hidden'} flex-col gap-2 md:mt-2 md:flex`}>
					<div class="flex flex-wrap items-center gap-1.5">
						<span class="text-[11px] font-semibold uppercase tracking-wide text-zinc-500">Filters</span>
						{#if hasFacetChoices(facets.statuses, statuses)}
							<MultiSelectDropdown label="Status" emptyLabel="Not inactive" options={facetEntries(facets.statuses)} selected={statuses} onToggle={(value) => toggleMulti('status', value)} onClear={() => clearFacet('status')} />
						{/if}
						{#if hasFacetChoices(facets.timeAspects, timeAspects)}
							<MultiSelectDropdown label="Time" options={facetEntries(facets.timeAspects)} selected={timeAspects} onToggle={(value) => toggleMulti('timeAspect', value)} onClear={() => clearFacet('timeAspect')} />
						{/if}
						{#if hasFacetChoices(facets.scales, scales)}
							<MultiSelectDropdown label="Scale" options={facetEntries(facets.scales)} selected={scales} onToggle={(value) => toggleMulti('scale', value)} onClear={() => clearFacet('scale')} />
						{/if}
						{#if hasFacetChoices(facets.methods, methods)}
							<MultiSelectDropdown label="Method" options={facetEntries(facets.methods)} selected={methods} onToggle={(value) => toggleMulti('method', value)} onClear={() => clearFacet('method')} />
						{/if}
						{#if hasFacetChoices(facets.orderObs, orderObsValues)}
							<MultiSelectDropdown label="Order/Obs" options={facetEntries(facets.orderObs)} selected={orderObsValues} onToggle={(value) => toggleMulti('orderObs', value)} onClear={() => clearFacet('orderObs')} />
						{/if}
					</div>
					{#if statuses.length || timeAspects.length || scales.length || methods.length || orderObsValues.length}
						<div class="mt-2 flex flex-wrap items-center gap-1.5" data-testid="selected-facet-summary">
							{#each statuses as status}
								<FilterChip label="Status" value={status} onRemove={() => clearFacet('status', status)} />
							{/each}
							{#each timeAspects as timeAspect}
								<FilterChip label="Time" value={timeAspect} onRemove={() => clearFacet('timeAspect', timeAspect)} />
							{/each}
							{#each scales as scale}
								<FilterChip label="Scale" value={scale} onRemove={() => clearFacet('scale', scale)} />
							{/each}
							{#each methods as method}
								<FilterChip label="Method" value={method} onRemove={() => clearFacet('method', method)} />
							{/each}
							{#each orderObsValues as orderObs}
								<FilterChip label="Order/Obs" value={orderObs} onRemove={() => clearFacet('orderObs', orderObs)} />
							{/each}
						</div>
					{/if}
					{#if hierarchyNodeId}
						<div class="mt-1.5 flex min-w-0 items-center gap-1.5 text-[11px] leading-5" data-testid="browse-path-summary">
							<span class="shrink-0 font-semibold uppercase tracking-wide text-zinc-500">Path</span>
							<span class="min-w-0 break-words text-zinc-700 [overflow-wrap:anywhere]">{hierarchyPath || hierarchyLabel || hierarchyNodeId}</span>
						</div>
					{/if}
					{#if system || selectedClass || property || rankedOnly}
						<div class="mt-1.5 flex flex-wrap items-center gap-1.5" data-testid="browse-scope-summary">
							<span class="text-[11px] font-semibold uppercase tracking-wide text-zinc-500">Scope</span>
							{#if rankedOnly}<FilterChip label="Rank" value="Top 20,000 common terms" onRemove={() => { rankedOnly = false; runSearch(0); }} />{/if}
							{#if system}<FilterChip label="System" value={system} onRemove={() => clearFacet('system')} />{/if}
							{#if selectedClass}<FilterChip label="Class" value={selectedClass} onRemove={() => clearFacet('class')} />{/if}
							{#if property}<FilterChip label="Property" value={property} onRemove={() => clearFacet('property')} />{/if}
						</div>
					{/if}
					</div>
				</form>

				{#if error}
					<div class="border-b border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800 lg:shrink-0">{error}</div>
				{/if}

				<div class="flex items-center justify-between border-b border-zinc-200 px-4 py-3 lg:shrink-0">
					<p class="text-sm text-zinc-500">
						{#if loading}Searching...{:else}Showing {results.length.toLocaleString()} of {total.toLocaleString()} terms{/if}
					</p>
					<div class="flex items-center gap-2">
						<Button variant="outline" size="sm" disabled={offset === 0 || loading} on:click={() => runSearch(Math.max(0, offset - limit))}>Previous</Button>
						<Button variant="outline" size="sm" disabled={offset + limit >= total || loading} on:click={() => runSearch(offset + limit)}>Next</Button>
					</div>
				</div>

				<div class="lg:min-h-0 lg:flex-1 lg:overflow-auto" data-testid="results-scroll-panel">
					{#if loading}
						<div class="space-y-3 p-4">
							{#each Array(8) as _}
								<div class="h-16 animate-pulse rounded-md bg-zinc-100"></div>
							{/each}
						</div>
					{:else if results.length === 0}
						<div class="p-4"><EmptyState /></div>
					{:else}
						<div class="divide-y divide-zinc-100 md:hidden">
							{#each results as result}
								<button type="button" class="w-full px-4 py-3 text-left hover:bg-zinc-50 focus:bg-zinc-50 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-zinc-300" on:click={() => openTerm(result.loincNum)}>
									<div class="flex min-w-0 items-center justify-between gap-2">
										<div class="min-w-0">
											<div class="flex items-center gap-2">
												<span class="font-mono text-xs font-semibold text-zinc-950">{result.loincNum}</span>
												<Badge variant={statusVariant(result.status)}>{result.status || 'UNKNOWN'}</Badge>
											</div>
											<div class="mt-1 break-words text-sm font-medium leading-5 text-zinc-950 [overflow-wrap:anywhere]">{result.longCommonName}</div>
										</div>
										{#if result.commonTestRank > 0}
											<span class="shrink-0 rounded bg-zinc-100 px-1.5 py-0.5 text-[11px] text-zinc-600">#{result.commonTestRank.toLocaleString()}</span>
										{/if}
									</div>
									<div class="mt-1 break-words text-xs leading-5 text-zinc-500 [overflow-wrap:anywhere]">{result.shortName || result.component}</div>
									<div class="mt-2 grid grid-cols-2 gap-x-3 gap-y-1 text-[11px] leading-4 text-zinc-500">
										<div class="min-w-0">
											<span class="font-semibold uppercase tracking-wide">Class</span>
											<span class="ml-1 break-words text-zinc-700 [overflow-wrap:anywhere]">{result.class || '-'}</span>
										</div>
										<div class="min-w-0">
											<span class="font-semibold uppercase tracking-wide">System</span>
											<span class="ml-1 break-words text-zinc-700 [overflow-wrap:anywhere]">{result.system || '-'}</span>
										</div>
										<div class="min-w-0">
											<span class="font-semibold uppercase tracking-wide">Property</span>
											<span class="ml-1 break-words text-zinc-700 [overflow-wrap:anywhere]">{result.property || '-'}</span>
										</div>
										<div class="min-w-0">
											<span class="font-semibold uppercase tracking-wide">Scale</span>
											<span class="ml-1 break-words text-zinc-700 [overflow-wrap:anywhere]">{result.scale || '-'}</span>
										</div>
									</div>
								</button>
							{/each}
						</div>
						<div class="hidden overflow-x-auto md:block">
							<table class="w-full table-fixed border-collapse text-left text-sm" style={`min-width: ${resultsTableWidth()}px`}>
								<colgroup>
									<col style={`width: ${columnWidths.loinc}px`} />
									<col style={`width: ${columnWidths.name}px`} />
									<col style={`width: ${columnWidths.status}px`} />
									<col style={`width: ${columnWidths.rank}px`} />
									<col style={`width: ${columnWidths.axes}px`} />
								</colgroup>
								<thead class="bg-zinc-50 text-xs uppercase tracking-wide text-zinc-500">
									<tr>
										<th class="relative px-4 py-3 font-medium">
											LOINC
											<button
												type="button"
												class="absolute right-0 top-0 h-full w-2 cursor-col-resize border-r border-transparent hover:border-zinc-400 focus:border-zinc-500 focus:outline-none"
												aria-label="Resize LOINC column"
												on:pointerdown={(event) => startColumnResize('loinc', event)}
											></button>
										</th>
										<th class="relative px-4 py-3 font-medium">
											Name
											<button
												type="button"
												class="absolute right-0 top-0 h-full w-2 cursor-col-resize border-r border-transparent hover:border-zinc-400 focus:border-zinc-500 focus:outline-none"
												aria-label="Resize Name column"
												on:pointerdown={(event) => startColumnResize('name', event)}
											></button>
										</th>
										<th class="relative px-4 py-3 font-medium">
											Status
											<button
												type="button"
												class="absolute right-0 top-0 h-full w-2 cursor-col-resize border-r border-transparent hover:border-zinc-400 focus:border-zinc-500 focus:outline-none"
												aria-label="Resize Status column"
												on:pointerdown={(event) => startColumnResize('status', event)}
											></button>
										</th>
										<th class="relative px-4 py-3 font-medium">
											Rank
											<button
												type="button"
												class="absolute right-0 top-0 h-full w-2 cursor-col-resize border-r border-transparent hover:border-zinc-400 focus:border-zinc-500 focus:outline-none"
												aria-label="Resize Rank column"
												on:pointerdown={(event) => startColumnResize('rank', event)}
											></button>
										</th>
										<th class="relative px-4 py-3 font-medium">
											Axes
											<button
												type="button"
												class="absolute right-0 top-0 h-full w-2 cursor-col-resize border-r border-transparent hover:border-zinc-400 focus:border-zinc-500 focus:outline-none"
												aria-label="Resize Axes column"
												on:pointerdown={(event) => startColumnResize('axes', event)}
											></button>
										</th>
									</tr>
								</thead>
								<tbody>
									{#each results as result}
										<tr class="cursor-pointer border-t border-zinc-100 hover:bg-zinc-50" on:click={() => openTerm(result.loincNum)}>
											<td class="px-4 py-3 font-mono text-sm text-zinc-950">{result.loincNum}</td>
											<td class="min-w-0 px-4 py-3">
												<div class="break-words font-medium leading-5 text-zinc-950 [overflow-wrap:anywhere]">{result.longCommonName}</div>
												<div class="mt-1 break-words text-xs text-zinc-500 [overflow-wrap:anywhere]">{result.shortName || result.component}</div>
											</td>
											<td class="px-4 py-3"><Badge variant={statusVariant(result.status)}>{result.status || 'UNKNOWN'}</Badge></td>
											<td class="px-4 py-3 text-sm text-zinc-600">{result.commonTestRank > 0 ? result.commonTestRank.toLocaleString() : '-'}</td>
											<td class="px-4 py-3 text-xs text-zinc-600">
												<div class="break-words [overflow-wrap:anywhere]">{result.class || 'No class'} / {result.system || 'No system'}</div>
												<div class="mt-1 break-words [overflow-wrap:anywhere]">{result.property || 'No property'} / {result.scale || 'No scale'}</div>
											</td>
										</tr>
									{/each}
								</tbody>
							</table>
						</div>
					{/if}
				</div>
			{/if}
				</section>
			</Resizable.Pane>
		</Resizable.PaneGroup>
	</div>

	{#if detailOpen || termLoading}
		<div class="pointer-events-none fixed inset-y-0 right-0 z-50 flex w-full justify-end" role="presentation">
			<aside class="pointer-events-auto flex h-full w-full max-w-[560px] flex-col border-l border-zinc-200 bg-white shadow-2xl" data-testid="detail-drawer">
				<div class="flex items-center justify-between border-b border-zinc-200 px-4 py-3 lg:shrink-0">
					<h2 class="text-sm font-semibold">Term detail</h2>
					<Button variant="ghost" size="icon" ariaLabel="Close term detail" on:click={closeTerm}><X size={16} /></Button>
				</div>
				<div class="min-h-0 flex-1 overflow-auto" data-testid="detail-scroll-panel">
					{#if termLoading}
						<div class="space-y-3 p-4">
							<div class="h-8 animate-pulse rounded-md bg-zinc-100"></div>
							<div class="h-24 animate-pulse rounded-md bg-zinc-100"></div>
							<div class="h-48 animate-pulse rounded-md bg-zinc-100"></div>
						</div>
					{:else if !selectedTerm}
						<div class="p-4"><EmptyState title="Select a term" body="Open any search result to inspect all imported LOINC fields." /></div>
					{:else}
						<div class="flex flex-col gap-4 p-4">
							<div>
								<div class="flex items-center gap-2">
									<span class="font-mono text-sm font-semibold">{selectedTerm.loincNum}</span>
									<Badge variant={statusVariant(selectedTerm.status)}>{selectedTerm.status || 'UNKNOWN'}</Badge>
								</div>
								<h3 class="mt-2 text-lg font-semibold leading-snug">{selectedTerm.longCommonName}</h3>
								{#if selectedTerm.definition}
									<p class="mt-2 text-sm leading-6 text-zinc-600">{selectedTerm.definition}</p>
								{/if}
							</div>

							<div class="grid grid-cols-2 gap-2 text-sm">
								<DetailField label="Component" value={selectedTerm.component} />
								<DetailField label="Property" value={selectedTerm.property} />
								<DetailField label="Time" value={selectedTerm.timeAspect} />
								<DetailField label="System" value={selectedTerm.system} />
								<DetailField label="Scale" value={selectedTerm.scale} />
								<DetailField label="Method" value={selectedTerm.method} />
								<DetailField label="Class" value={selectedTerm.class} />
								<DetailField label="Order/Obs" value={selectedTerm.orderObs} />
								<DetailField label="Rank" value={selectedTerm.commonTestRank > 0 ? selectedTerm.commonTestRank.toLocaleString() : '-'} />
							</div>

							{#if relationshipsLoading}
								<section class="rounded-md border border-zinc-200 p-3">
									<div class="flex items-center gap-2 text-sm text-zinc-500">
										<Network size={15} />
										Loading relationship details...
									</div>
								</section>
							{:else if relationshipsLoaded && hasRelationshipGraph(relationshipGraph)}
								<div class="flex flex-col gap-3">
									<div class="flex items-center justify-between gap-3">
										<h4 class="text-xs font-semibold uppercase tracking-wide text-zinc-500">Relationship graph</h4>
										<Button variant="outline" size="sm" on:click={openGraphViewer}>
											<Network size={14} />
											View graph
										</Button>
									</div>
									{#if relationshipGraph?.outgoingMapTo?.length || relationshipGraph?.incomingMapTo?.length}
										<section class="rounded-md border border-zinc-200">
											<div class="border-b border-zinc-100 bg-zinc-50 px-3 py-2 text-xs font-semibold uppercase tracking-wide text-zinc-500">Direct term links</div>
											<div class="divide-y divide-zinc-100">
												{#each relationshipGraph?.outgoingMapTo ?? [] as item}
													<div class="px-3 py-2 text-sm">
														<div class="text-xs font-medium uppercase tracking-wide text-zinc-500">This term maps to</div>
														<button type="button" class="mt-1 font-mono font-semibold text-zinc-950 hover:underline" on:click={() => openTerm(item.mapTo)}>{item.mapTo}</button>
														{#if item.comment}<div class="mt-1 text-xs text-zinc-500">{item.comment}</div>{/if}
													</div>
												{/each}
												{#each relationshipGraph?.incomingMapTo ?? [] as item}
													<div class="px-3 py-2 text-sm">
														<div class="text-xs font-medium uppercase tracking-wide text-zinc-500">Other term maps here</div>
														<button type="button" class="mt-1 font-mono font-semibold text-zinc-950 hover:underline" on:click={() => openTerm(item.loinc)}>{item.loinc}</button>
														{#if item.comment}<div class="mt-1 text-xs text-zinc-500">{item.comment}</div>{/if}
													</div>
												{/each}
											</div>
										</section>
									{/if}
								</div>
							{:else}
								<section class="rounded-md border border-zinc-200 p-3">
									<div class="flex items-center justify-between gap-3">
										<div class="min-w-0">
											<h4 class="text-xs font-semibold uppercase tracking-wide text-zinc-500">Relationship graph</h4>
											<p class="mt-1 text-xs text-zinc-500">Relationship details are loaded only when needed.</p>
										</div>
										<Button variant="outline" size="sm" on:click={openGraphViewer}>
											<Network size={14} />
											View graph
										</Button>
									</div>
								</section>
							{/if}

							{#if relationshipsLoaded && hasAccessories(selectedTerm)}
								<div class="flex flex-col gap-3">
									<h4 class="text-xs font-semibold uppercase tracking-wide text-zinc-500">Relationships and accessories</h4>
									{#if selectedTerm.mapTo?.length}
										<section class="rounded-md border border-zinc-200">
											<div class="border-b border-zinc-100 bg-zinc-50 px-3 py-2 text-xs font-semibold uppercase tracking-wide text-zinc-500">Maps to</div>
											<div class="divide-y divide-zinc-100">
												{#each selectedTerm.mapTo as item}
													<div class="px-3 py-2 text-sm">
														<button type="button" class="font-mono font-semibold text-zinc-950 hover:underline" on:click={() => openTerm(item.mapTo)}>{item.mapTo}</button>
														{#if item.comment}<div class="mt-1 text-xs text-zinc-500">{item.comment}</div>{/if}
													</div>
												{/each}
											</div>
										</section>
									{/if}
									{#each accessorySections(selectedTerm) as section}
										<section class="rounded-md border border-zinc-200">
											<div class="flex items-center justify-between gap-2 border-b border-zinc-100 bg-zinc-50 px-3 py-2">
												<span class="text-xs font-semibold uppercase tracking-wide text-zinc-500">{section.title} ({section.items.length})</span>
												<button type="button" class="text-xs font-medium text-zinc-700 hover:underline" on:click={() => browseAccessoryForSelected(section.kind)}>Open browser</button>
											</div>
											<div class="max-h-72 divide-y divide-zinc-100 overflow-auto">
												{#each section.items.slice(0, 50) as item}
													<div class="px-3 py-2 text-sm">
														<div class="flex items-start justify-between gap-2">
															<div class="min-w-0">
																<div class="break-words font-medium text-zinc-950">{accessoryTitle(item)}</div>
																{#if item.subtitle}<div class="mt-0.5 break-words text-xs text-zinc-500">{item.subtitle}</div>{/if}
															</div>
															{#if item.code}<span class="shrink-0 rounded bg-zinc-100 px-1.5 py-0.5 font-mono text-[11px] text-zinc-600">{item.code}</span>{/if}
														</div>
													</div>
												{/each}
											</div>
										</section>
									{/each}
								</div>
							{/if}

							<div>
								<h4 class="mb-2 text-xs font-semibold uppercase tracking-wide text-zinc-500">All fields</h4>
								<div class="rounded-md border border-zinc-200">
									{#each Object.entries(selectedTerm.fields ?? {}) as [key, value]}
										<div class="grid grid-cols-[150px_minmax(0,1fr)] border-b border-zinc-100 text-xs last:border-b-0">
											<div class="break-words bg-zinc-50 px-3 py-2 font-medium text-zinc-600">{key}</div>
											<div class="break-words px-3 py-2 text-zinc-800">{value || ' '}</div>
										</div>
									{/each}
								</div>
							</div>

							{#if !relationshipsLoaded}
								<section class="rounded-md border border-zinc-200">
									<button
										type="button"
										class="flex w-full items-center justify-between gap-3 bg-zinc-50 px-3 py-2 text-left"
										aria-expanded="false"
										on:click={toggleSharedConcepts}
									>
										<span class="min-w-0">
											<span class="block text-xs font-semibold uppercase tracking-wide text-zinc-500">Shared concept neighborhoods</span>
											<span class="mt-0.5 block text-xs text-zinc-500">Load shared concepts only when you need broader exploration.</span>
										</span>
										<ChevronRight class="shrink-0 text-zinc-500" size={16} />
									</button>
								</section>
							{:else if sharedConcepts().length}
								<section class="rounded-md border border-zinc-200">
									<button
										type="button"
										class="flex w-full items-center justify-between gap-3 bg-zinc-50 px-3 py-2 text-left"
										aria-expanded={sharedConceptsOpen}
										on:click={toggleSharedConcepts}
									>
										<span class="min-w-0">
											<span class="block text-xs font-semibold uppercase tracking-wide text-zinc-500">Shared concept neighborhoods</span>
											<span class="mt-0.5 block text-xs text-zinc-500">{sharedConcepts().length.toLocaleString()} concepts connect this term to related terms</span>
										</span>
										{#if sharedConceptsOpen}<ChevronDown class="shrink-0 text-zinc-500" size={16} />{:else}<ChevronRight class="shrink-0 text-zinc-500" size={16} />{/if}
									</button>
									{#if sharedConceptsOpen}
										<div class="divide-y divide-zinc-100">
											{#each sharedConcepts().slice(0, 20) as concept}
												<div class="px-3 py-3 text-sm">
													<div class="flex items-start justify-between gap-3">
														<div class="min-w-0">
															<div class="break-words font-medium text-zinc-950">{accessoryTitle(concept)}</div>
															<div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-zinc-500">
																<span>{concept.kind}</span>
																{#if concept.code}<span class="font-mono">{concept.code}</span>{/if}
																<span>{concept.relatedTotal.toLocaleString()} other terms</span>
															</div>
														</div>
														<button type="button" class="shrink-0 text-xs font-medium text-zinc-700 hover:underline" on:click={() => browseConcept(concept)}>Browse</button>
													</div>
													{#if concept.relatedTerms?.length}
														<div class="mt-2 flex flex-col gap-1.5">
															{#each (concept.relatedTerms ?? []).slice(0, 6) as related}
																<button type="button" class="rounded-md border border-zinc-100 px-2 py-1.5 text-left hover:border-zinc-300 hover:bg-zinc-50" on:click={() => openTerm(related.loincNum)}>
																	<div class="flex items-center gap-2">
																		<span class="font-mono text-xs font-semibold text-zinc-800">{related.loincNum}</span>
																		<Badge variant={statusVariant(related.status)}>{related.status || 'UNKNOWN'}</Badge>
																	</div>
																	<div class="mt-1 break-words text-xs text-zinc-600">{termSummaryLabel(related)}</div>
																</button>
															{/each}
														</div>
													{/if}
												</div>
											{/each}
										</div>
									{/if}
								</section>
							{/if}
						</div>
					{/if}
				</div>
			</aside>
		</div>
	{/if}

	{#if graphViewerOpen && selectedTerm && relationshipGraph}
		<div class="fixed inset-0 z-[70] flex items-center justify-center bg-zinc-950/35 p-4">
			<section class="flex h-[94vh] w-full max-w-7xl flex-col overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-2xl" data-testid="relationship-graph-viewer">
				<div class="flex items-center justify-between gap-4 border-b border-zinc-200 px-4 py-3">
					<div class="min-w-0">
						<h2 class="text-sm font-semibold">Connection graph</h2>
						<p class="mt-1 truncate text-xs text-zinc-500">{selectedTerm.loincNum} · {selectedTerm.longCommonName}</p>
					</div>
					<div class="flex items-center gap-2">
						<Button variant="outline" size="sm" disabled={graphVisibleConceptLimit <= 8} on:click={() => (graphVisibleConceptLimit = Math.max(8, graphVisibleConceptLimit - 4))}>Fewer</Button>
						<Button variant="outline" size="sm" disabled={graphVisibleConceptLimit >= sharedConcepts().length} on:click={() => (graphVisibleConceptLimit = Math.min(sharedConcepts().length, graphVisibleConceptLimit + 4))}>More</Button>
						<Button variant="ghost" size="icon" ariaLabel="Close relationship graph" on:click={() => (graphViewerOpen = false)}><X size={16} /></Button>
					</div>
				</div>
				<RelationshipGraph
					term={selectedTerm}
					concepts={sharedConcepts()}
					maxConcepts={graphVisibleConceptLimit}
					maxTermsPerConcept={3}
					onOpenTerm={(loincNum) => openTerm(loincNum)}
					onBrowseConcept={browseConcept}
				/>
			</section>
		</div>
	{/if}
	<footer class="fixed inset-x-0 bottom-0 z-50 border-t border-zinc-200 bg-white shadow-[0_-1px_3px_rgba(24,24,27,0.04)]">
		<div class="mx-auto flex max-w-[1500px] flex-col gap-2 px-5 py-3 text-[11px] leading-4 text-zinc-500 md:flex-row md:items-center md:justify-between">
			<p>
				{#if versionInfo}<span class="font-mono text-zinc-700">v{versionInfo.version}</span><span class="mx-2 text-zinc-300">|</span>{/if}
				LOINC is Copyright © Regenstrief Institute, Inc. and the LOINC Committee.
			</p>
			<nav class="flex flex-wrap gap-x-3 gap-y-1" aria-label="Footer links">
				<a class="font-medium underline underline-offset-2 hover:text-zinc-900" href="/api/docs">Swagger API</a>
				<a class="font-medium underline underline-offset-2 hover:text-zinc-900" href="/openapi.json">OpenAPI</a>
				<a class="font-medium underline underline-offset-2 hover:text-zinc-900" href="/mcp">MCP endpoint</a>
				<a class="font-medium underline underline-offset-2 hover:text-zinc-900" href="/docs/mcp">MCP guide</a>
				<a class="font-medium underline underline-offset-2 hover:text-zinc-900" href="/docs/concepts">LOINC concepts</a>
				<a class="font-medium underline underline-offset-2 hover:text-zinc-900" href="/docs/agent-guide">Agent guide</a>
				<a class="font-medium underline underline-offset-2 hover:text-zinc-900" href="https://loinc.org/kb/license/" target="_blank" rel="noreferrer">
					LOINC license
				</a>
				<a class="font-medium underline underline-offset-2 hover:text-zinc-900" href="https://creativecommons.org/licenses/by/4.0/" target="_blank" rel="noreferrer">
					CC BY attribution
				</a>
				<a class="font-medium underline underline-offset-2 hover:text-zinc-900" href="https://github.com/drguptavivek/loinc-browser" target="_blank" rel="noreferrer">
					GitHub
				</a>
			</nav>
		</div>
	</footer>
</main>
