<script lang="ts">
	import { onMount } from 'svelte';
	import {
		BookOpen,
		ChevronDown,
		ChevronRight,
		Database,
		FilterX,
		Maximize2,
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
	import MultiSelectDropdown from '$lib/components/MultiSelectDropdown.svelte';
	import {
		browseAccessories,
		getCacheStats,
		getFacets,
		getTerm,
		getTermRelationships,
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
		type TermSummary,
	} from '$lib/api';

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
	let results: SearchResult[] = [];
	let total = 0;
	let facets: Facets = emptyFacets;
	let cacheStats: CacheStats | null = null;
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
	let activeView: 'browse' | 'loader' | 'accessories' = 'browse';
	let detailOpen = false;
	let sharedConceptsOpen = false;
	let graphViewerOpen = false;
	let activeGraphConceptCode = '';
	let graphVisibleConceptLimit = 8;
	let graphZoom = 1;
	let graphPanX = 0;
	let graphPanY = 0;
	let graphPanning = false;
	let graphPanStartX = 0;
	let graphPanStartY = 0;
	let graphPanOriginX = 0;
	let graphPanOriginY = 0;
	let facetsCollapsed = false;
	let resultsFullscreen = false;
	let resizingFacets = false;
	let facetWidth = 280;
	let layoutElement: HTMLDivElement;
	let columnWidths = {
		loinc: 96,
		name: 520,
		status: 112,
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
	const accessoryKinds = [
		{ value: 'part-primary', label: 'Primary parts' },
		{ value: 'part-supplementary', label: 'Supplementary parts' },
		{ value: 'answer-list', label: 'Answer lists' },
		{ value: 'panel-membership', label: 'Panel membership' },
		{ value: 'panel-child', label: 'Panel children' },
		{ value: 'group', label: 'Groups' },
		{ value: 'hierarchy', label: 'Hierarchy' },
	];
	const minFacetWidth = 220;
	const maxFacetWidth = 520;

	onMount(() => {
		void (async () => {
			applyURLState();
			await Promise.all([loadFacets(), runSearch(offset, true)]);
			if (initialTerm) {
				await openTerm(initialTerm, true);
			}
		})();

		const handlePopState = async () => {
			applyURLState();
			await runSearch(offset, true);
			if (initialTerm) {
				await openTerm(initialTerm, true);
			} else {
				selectedTerm = null;
				detailOpen = false;
			}
		};

		window.addEventListener('popstate', handlePopState);
		window.addEventListener('pointermove', handleFacetResize);
		window.addEventListener('pointermove', handleTableResize);
		window.addEventListener('pointermove', handleGraphPan);
		window.addEventListener('pointerup', stopFacetResize);
		window.addEventListener('pointerup', stopTableResize);
		window.addEventListener('pointerup', stopGraphPan);
		return () => {
			window.removeEventListener('popstate', handlePopState);
			window.removeEventListener('pointermove', handleFacetResize);
			window.removeEventListener('pointermove', handleTableResize);
			window.removeEventListener('pointermove', handleGraphPan);
			window.removeEventListener('pointerup', stopFacetResize);
			window.removeEventListener('pointerup', stopTableResize);
			window.removeEventListener('pointerup', stopGraphPan);
		};
	});

	function applyURLState() {
		const params = new URLSearchParams(window.location.search);
		query = params.get('q') ?? '';
		selectedClass = params.get('class') ?? '';
		system = params.get('system') ?? '';
		property = params.get('property') ?? '';
		statuses = params.getAll('status');
		timeAspects = params.getAll('timeAspect');
		scales = params.getAll('scale');
		methods = params.getAll('method');
		orderObsValues = params.getAll('orderObs');
		offset = Number(params.get('offset') ?? '0') || 0;
		initialTerm = params.get('term') ?? '';
	}

	function updateURL(replace = true) {
		const params = new URLSearchParams();
		if (query.trim()) params.set('q', query.trim());
		if (selectedClass) params.set('class', selectedClass);
		if (system) params.set('system', system);
		if (property) params.set('property', property);
		for (const value of statuses) params.append('status', value);
		for (const value of timeAspects) params.append('timeAspect', value);
		for (const value of scales) params.append('scale', value);
		for (const value of methods) params.append('method', value);
		for (const value of orderObsValues) params.append('orderObs', value);
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

	async function runSearch(nextOffset = 0, replaceURL = false) {
		loading = true;
		error = '';
		activeView = 'browse';
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
		activeGraphConceptCode = '';
		graphVisibleConceptLimit = 8;
		resetGraphViewport();
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
			const [term, graph] = await Promise.all([getTerm(selectedTerm.loincNum, true), getTermRelationships(selectedTerm.loincNum)]);
			selectedTerm = term;
			relationshipGraph = graph;
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
		runSearch(0);
	}

	function resetAll() {
		query = '';
		clearFilters();
		selectedTerm = null;
		relationshipGraph = null;
		relationshipsLoaded = false;
		detailOpen = false;
	}

	function chooseFacet(kind: 'status' | 'class' | 'system' | 'scale' | 'property' | 'orderObs', value: string) {
		if (kind === 'class') selectedClass = selectedClass === value ? '' : value;
		if (kind === 'system') system = system === value ? '' : value;
		if (kind === 'property') property = property === value ? '' : value;
		runSearch(0);
	}

	function clearFacet(kind: 'status' | 'class' | 'system' | 'timeAspect' | 'scale' | 'method' | 'property' | 'orderObs', value = '') {
		if (kind === 'class') selectedClass = '';
		if (kind === 'system') system = '';
		if (kind === 'property') property = '';
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
		return [selectedClass, system, property, ...statuses, ...timeAspects, ...scales, ...methods, ...orderObsValues].filter(Boolean).length;
	}

	function resultsTableWidth() {
		return columnWidths.loinc + columnWidths.name + columnWidths.status + columnWidths.axes;
	}

	function facetEntries(values: Record<string, number> | null | undefined) {
		return Object.entries(values ?? {});
	}

	function statusVariant(value: string) {
		if (value === 'ACTIVE') return 'default';
		if (value === 'DISCOURAGED') return 'warning';
		return 'secondary';
	}

	function errorMessage(err: unknown) {
		return err instanceof Error ? err.message : String(err);
	}

	function startFacetResize(event: PointerEvent) {
		if (facetsCollapsed || !layoutElement) return;
		event.preventDefault();
		resizingFacets = true;
		handleFacetResize(event);
	}

	function handleFacetResize(event: PointerEvent) {
		if (!resizingFacets || !layoutElement) return;
		const left = layoutElement.getBoundingClientRect().left;
		facetWidth = Math.min(maxFacetWidth, Math.max(minFacetWidth, event.clientX - left));
	}

	function stopFacetResize() {
		resizingFacets = false;
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
	}

	async function openAccessoryBrowser(kind = accessoryKind, q = '') {
		accessoryKind = kind;
		accessoryQuery = q;
		accessoryOffset = 0;
		activeView = 'accessories';
		resultsFullscreen = false;
		await loadAccessories(0);
	}

	async function loadAccessories(nextOffset = accessoryOffset) {
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
		void openAccessoryBrowser(kind, selectedTerm.loincNum);
	}

	function browseConcept(concept: { kind: string; code: string }) {
		void openAccessoryBrowser(concept.kind, concept.code);
	}

	function termSummaryLabel(term: TermSummary) {
		return term.longCommonName || term.shortName || term.loincNum;
	}

	function sharedConcepts() {
		return relationshipGraph?.sharedConcepts ?? [];
	}

	function graphConcepts() {
		return sharedConcepts()
			.filter((concept) => concept.relatedTotal > 0 || (concept.relatedTerms?.length ?? 0) > 0)
			.slice(0, graphVisibleConceptLimit);
	}

	function graphRelatedTerms(concept: RelationshipConcept) {
		const expanded = graphConceptKey(concept) === activeGraphConceptCode;
		return (concept.relatedTerms ?? []).slice(0, expanded ? 6 : 2);
	}

	function graphConceptKey(concept: RelationshipConcept) {
		return `${concept.kind}:${concept.code || concept.title}`;
	}

	function toggleGraphConcept(concept: RelationshipConcept) {
		const key = graphConceptKey(concept);
		activeGraphConceptCode = activeGraphConceptCode === key ? '' : key;
	}

	function graphAngle(index: number, total: number) {
		return (Math.PI * 2 * index) / Math.max(total, 1) - Math.PI / 2;
	}

	function graphX(index: number, total: number, radius: number) {
		return 320 + Math.cos(graphAngle(index, total)) * radius;
	}

	function graphY(index: number, total: number, radius: number) {
		return 230 + Math.sin(graphAngle(index, total)) * radius;
	}

	function graphRelatedX(index: number, total: number, relatedIndex: number, relatedTotal: number) {
		const angle = graphAngle(index, total);
		const spread = (relatedIndex - (relatedTotal - 1) / 2) * 24;
		return 320 + Math.cos(angle) * 260 + Math.cos(angle + Math.PI / 2) * spread;
	}

	function graphRelatedY(index: number, total: number, relatedIndex: number, relatedTotal: number) {
		const angle = graphAngle(index, total);
		const spread = (relatedIndex - (relatedTotal - 1) / 2) * 24;
		return 230 + Math.sin(angle) * 260 + Math.sin(angle + Math.PI / 2) * spread;
	}

	function handleGraphKey(event: KeyboardEvent, action: () => void) {
		if (event.key === 'Enter' || event.key === ' ') {
			event.preventDefault();
			action();
		}
	}

	function zoomGraph(delta: number) {
		graphZoom = Math.min(2.4, Math.max(0.55, Number((graphZoom + delta).toFixed(2))));
	}

	function resetGraphViewport() {
		graphZoom = 1;
		graphPanX = 0;
		graphPanY = 0;
		graphPanning = false;
	}

	function startGraphPan(event: PointerEvent) {
		if (event.button !== 0) return;
		graphPanning = true;
		graphPanStartX = event.clientX;
		graphPanStartY = event.clientY;
		graphPanOriginX = graphPanX;
		graphPanOriginY = graphPanY;
	}

	function handleGraphPan(event: PointerEvent) {
		if (!graphPanning) return;
		graphPanX = graphPanOriginX + (event.clientX - graphPanStartX) / graphZoom;
		graphPanY = graphPanOriginY + (event.clientY - graphPanStartY) / graphZoom;
	}

	function stopGraphPan() {
		graphPanning = false;
	}
</script>

<main class="min-h-screen bg-zinc-50 text-zinc-950 lg:flex lg:h-screen lg:flex-col lg:overflow-hidden">
	<header class="border-b border-zinc-200 bg-white lg:shrink-0">
		<div class="mx-auto flex max-w-[1500px] items-center justify-between gap-4 px-5 py-4">
			<div class="flex items-center gap-3">
				<div class="flex size-10 items-center justify-center rounded-md bg-zinc-950 text-white">
					<BookOpen size={20} />
				</div>
				<div>
					<h1 class="text-lg font-semibold tracking-normal">LOINC Browser</h1>
					<p class="text-sm text-zinc-500">Local release search with SQLite FTS5</p>
				</div>
			</div>
			<div class="hidden items-center gap-3 text-sm text-zinc-500 md:flex">
				<div class="flex items-center gap-2 rounded-md border border-zinc-200 px-3 py-2">
					<Database size={16} />
					<span>{total.toLocaleString()} matches</span>
				</div>
				<div class="flex items-center gap-2 rounded-md border border-zinc-200 px-3 py-2">
					<Server size={16} />
					<span>{cacheStats ? `${cacheStats.termEntries} cached` : 'cache ready'}</span>
				</div>
			</div>
		</div>
	</header>

	<div
		bind:this={layoutElement}
		class="mx-auto flex w-full max-w-[1500px] flex-col gap-5 px-5 py-5 lg:min-h-0 lg:flex-1 lg:flex-row lg:gap-0 lg:overflow-hidden"
		style={`--facet-width: ${facetsCollapsed ? 52 : facetWidth}px`}
	>
		<aside class="facet-pane order-2 flex flex-col gap-4 lg:order-1 lg:min-h-0 lg:shrink-0">
			<section class="rounded-lg border border-zinc-200 bg-white lg:flex lg:h-full lg:min-h-0 lg:flex-col lg:overflow-hidden">
				<div class="flex items-center justify-between border-b border-zinc-200 px-4 py-3 lg:shrink-0">
					{#if facetsCollapsed}
						<button
							type="button"
							class="inline-flex size-8 items-center justify-center rounded-md text-zinc-700 hover:bg-zinc-100 hover:text-zinc-950"
							aria-label="Expand facets"
							on:click={() => (facetsCollapsed = false)}
						>
							<PanelLeftOpen size={16} />
						</button>
					{:else}
						<h2 class="text-sm font-semibold">Browse facets</h2>
						<div class="flex items-center gap-1">
							{#if activeFilterCount() > 0}
								<Button variant="ghost" size="sm" on:click={clearFilters}>
									<FilterX size={14} />
									Clear
								</Button>
							{/if}
							<button
								type="button"
								class="inline-flex size-8 items-center justify-center rounded-md text-zinc-700 hover:bg-zinc-100 hover:text-zinc-950"
								aria-label="Collapse facets"
								on:click={() => (facetsCollapsed = true)}
							>
								<PanelLeftClose size={16} />
							</button>
						</div>
					{/if}
				</div>
				{#if !facetsCollapsed}
					<div class="flex flex-col gap-4 p-4 lg:min-h-0 lg:flex-1 lg:overflow-auto" data-testid="facet-scroll-panel">
						{#if facetsLoading}
							<p class="text-sm text-zinc-500">Loading facets...</p>
						{:else}
							<FacetGroup title="System" entries={facetEntries(facets.systems)} active={system} kind="system" onPick={chooseFacet} />
							<FacetGroup title="Class" entries={facetEntries(facets.classes)} active={selectedClass} kind="class" onPick={chooseFacet} />
							<FacetGroup title="Property" entries={facetEntries(facets.properties)} active={property} kind="property" onPick={chooseFacet} />
						{/if}
					</div>
					<div class="border-t border-zinc-200 p-3 lg:shrink-0">
						<Button variant={activeView === 'loader' ? 'default' : 'outline'} className="w-full justify-start" on:click={openLoader}>
							<Upload size={15} />
							Load release zip
						</Button>
						<Button variant={activeView === 'accessories' ? 'default' : 'outline'} className="mt-2 w-full justify-start" on:click={() => openAccessoryBrowser()}>
							<Database size={15} />
							Browse relationships
						</Button>
						<p class="mt-2 text-[11px] leading-4 text-zinc-500">Open loader page and ingest status.</p>
					</div>
				{/if}
			</section>
		</aside>

		<button
			type="button"
			class={`group hidden w-4 shrink-0 cursor-col-resize items-stretch justify-center rounded-md lg:order-1 lg:flex ${resizingFacets ? 'bg-zinc-100' : 'hover:bg-zinc-100'}`}
			aria-label="Resize facets panel"
			title="Drag to resize facets panel"
			on:pointerdown={startFacetResize}
		>
			<span class={`my-2 w-1 rounded-full ${resizingFacets ? 'bg-zinc-500' : 'bg-zinc-300 group-hover:bg-zinc-500'}`}></span>
		</button>

		<section
			class={`results-pane order-1 min-w-0 rounded-lg border border-zinc-200 bg-white lg:order-2 lg:flex lg:min-h-0 lg:flex-1 lg:flex-col lg:overflow-hidden ${resultsFullscreen ? 'fixed inset-0 z-40 h-screen rounded-none' : 'lg:h-full'}`}
			data-testid="results-pane"
		>
			{#if activeView === 'loader'}
				<div class="flex items-center justify-between gap-4 border-b border-zinc-200 px-4 py-3 lg:shrink-0">
					<div class="min-w-0">
						<h2 class="text-sm font-semibold">Release loader</h2>
						<p class="mt-1 text-xs text-zinc-500">Upload a licensed LOINC release zip and ingest it into the local SQLite database.</p>
					</div>
					<div class="flex items-center gap-2">
						<Button variant="outline" size="sm" on:click={() => (activeView = 'browse')}>Back to browse</Button>
						<Button variant="ghost" size="icon" ariaLabel={resultsFullscreen ? 'Exit fullscreen' : 'Expand results'} on:click={() => (resultsFullscreen = !resultsFullscreen)}>
							{#if resultsFullscreen}<Minimize2 size={16} />{:else}<Maximize2 size={16} />{/if}
						</Button>
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
									<div class="mt-1 text-lg font-semibold">{cacheStats ? cacheStats.termEntries.toLocaleString() : '0'}</div>
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
			{:else if activeView === 'accessories'}
				<div class="flex items-center justify-between gap-4 border-b border-zinc-200 px-4 py-3 lg:shrink-0">
					<div class="min-w-0">
						<h2 class="text-sm font-semibold">Relationship browser</h2>
						<p class="mt-1 text-xs text-zinc-500">Browse imported LOINC accessory rows and open linked terms.</p>
					</div>
					<div class="flex items-center gap-2">
						<Button variant="outline" size="sm" on:click={() => (activeView = 'browse')}>Back to search</Button>
						<Button variant="ghost" size="icon" ariaLabel={resultsFullscreen ? 'Exit fullscreen' : 'Expand results'} on:click={() => (resultsFullscreen = !resultsFullscreen)}>
							{#if resultsFullscreen}<Minimize2 size={16} />{:else}<Maximize2 size={16} />{/if}
						</Button>
					</div>
				</div>
				<form class="border-b border-zinc-200 p-4 lg:shrink-0" on:submit|preventDefault={() => loadAccessories(0)}>
					<div class="grid gap-3 md:grid-cols-[220px_minmax(0,1fr)_auto]">
						<label class="text-xs font-semibold uppercase tracking-wide text-zinc-500">
							Type
							<select class="mt-1 h-10 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm normal-case tracking-normal text-zinc-800" bind:value={accessoryKind} on:change={() => loadAccessories(0)}>
								{#each accessoryKinds as item}
									<option value={item.value}>{item.label}</option>
								{/each}
							</select>
						</label>
						<label class="text-xs font-semibold uppercase tracking-wide text-zinc-500">
							Search relationships
							<input class="mt-1 h-10 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm normal-case tracking-normal text-zinc-800 outline-none focus:border-zinc-400 focus:ring-2 focus:ring-zinc-100" bind:value={accessoryQuery} placeholder="Part, group, panel, answer list, or LOINC number" />
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
					<p class="text-sm text-zinc-500">{accessoryLoading ? 'Loading relationships...' : `Showing ${accessoryResults.length.toLocaleString()} of ${accessoryTotal.toLocaleString()} relationships`}</p>
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
						<div class="p-4"><EmptyState title="No relationships" body="Try another relationship type or search term." /></div>
					{:else}
						<div class="divide-y divide-zinc-100">
							{#each accessoryResults as item}
								<button type="button" class="grid w-full cursor-pointer gap-3 px-4 py-3 text-left text-sm hover:bg-zinc-50 focus:bg-zinc-50 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-zinc-300 md:grid-cols-[110px_minmax(0,1fr)_120px]" on:click={() => openTerm(item.loincNum)}>
									<div class="font-mono font-semibold text-zinc-950">{item.loincNum}</div>
									<div class="min-w-0">
										<div class="break-words font-medium text-zinc-950">{item.longCommonName || item.shortName || accessoryTitle(item)}</div>
										<div class="mt-1 break-words text-xs text-zinc-500">
											{accessoryTitle(item)}
											{#if item.subtitle}<span> · {item.subtitle}</span>{/if}
										</div>
									</div>
									<div class="font-mono text-xs text-zinc-500">{item.code}</div>
								</button>
							{/each}
						</div>
					{/if}
				</div>
			{:else}
				<form class="border-b border-zinc-200 p-4 lg:shrink-0" on:submit|preventDefault={() => runSearch(0)}>
					<div class="flex flex-col gap-3 md:flex-row">
						<label class="relative min-w-0 flex-1">
							<Search class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-zinc-400" size={18} />
							<input
								class="h-11 w-full rounded-md border border-zinc-200 bg-white pl-10 pr-3 text-sm outline-none transition focus:border-zinc-400 focus:ring-2 focus:ring-zinc-100"
								bind:value={query}
								placeholder="Search LOINC number, long name, component, related names..."
							/>
						</label>
						<Button type="submit" className="h-11">
							<Search size={16} />
							Search
						</Button>
						<Button type="button" variant="outline" className="h-11" on:click={resetAll}>
							<RefreshCcw size={16} />
							Reset
						</Button>
						<Button type="button" variant="ghost" size="icon" ariaLabel={resultsFullscreen ? 'Exit fullscreen' : 'Expand results'} on:click={() => (resultsFullscreen = !resultsFullscreen)}>
							{#if resultsFullscreen}<Minimize2 size={16} />{:else}<Maximize2 size={16} />{/if}
						</Button>
					</div>
					<div class="mt-3 flex flex-wrap items-center gap-2">
						<span class="text-xs font-medium uppercase tracking-wide text-zinc-500">Filtering facets</span>
						<MultiSelectDropdown label="Status" options={facetEntries(facets.statuses)} selected={statuses} onToggle={(value) => toggleMulti('status', value)} onClear={() => clearFacet('status')} />
						<MultiSelectDropdown label="Time" options={facetEntries(facets.timeAspects)} selected={timeAspects} onToggle={(value) => toggleMulti('timeAspect', value)} onClear={() => clearFacet('timeAspect')} />
						<MultiSelectDropdown label="Scale" options={facetEntries(facets.scales)} selected={scales} onToggle={(value) => toggleMulti('scale', value)} onClear={() => clearFacet('scale')} />
						<MultiSelectDropdown label="Method" options={facetEntries(facets.methods)} selected={methods} onToggle={(value) => toggleMulti('method', value)} onClear={() => clearFacet('method')} />
						<MultiSelectDropdown label="Order/Obs" options={facetEntries(facets.orderObs)} selected={orderObsValues} onToggle={(value) => toggleMulti('orderObs', value)} onClear={() => clearFacet('orderObs')} />
					</div>
					{#if system || selectedClass || property}
						<div class="mt-3 flex flex-col gap-2 rounded-md border border-zinc-200 bg-zinc-50 p-2">
							<span class="text-[11px] font-semibold uppercase tracking-wide text-zinc-500">Browse path filters</span>
							<div class="flex flex-wrap gap-2">
								{#if system}<FilterChip label="System" value={system} truncate={false} onRemove={() => clearFacet('system')} />{/if}
								{#if selectedClass}<FilterChip label="Class" value={selectedClass} truncate={false} onRemove={() => clearFacet('class')} />{/if}
								{#if property}<FilterChip label="Property" value={property} truncate={false} onRemove={() => clearFacet('property')} />{/if}
							</div>
						</div>
					{/if}
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
						<div class="overflow-x-auto">
							<table class="w-full table-fixed border-collapse text-left text-sm" style={`min-width: ${resultsTableWidth()}px`}>
								<colgroup>
									<col style={`width: ${columnWidths.loinc}px`} />
									<col style={`width: ${columnWidths.name}px`} />
									<col style={`width: ${columnWidths.status}px`} />
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
						<Button variant="outline" size="sm" on:click={() => zoomGraph(-0.15)}>Zoom out</Button>
						<span class="w-12 text-center text-xs tabular-nums text-zinc-500">{Math.round(graphZoom * 100)}%</span>
						<Button variant="outline" size="sm" on:click={() => zoomGraph(0.15)}>Zoom in</Button>
						<Button variant="outline" size="sm" on:click={resetGraphViewport}>Reset view</Button>
						<Button variant="ghost" size="icon" ariaLabel="Close relationship graph" on:click={() => (graphViewerOpen = false)}><X size={16} /></Button>
					</div>
				</div>
				<div class="grid min-h-0 flex-1 gap-0 overflow-auto lg:grid-cols-[minmax(0,2fr)_360px]">
					<div class="min-h-[620px] overflow-auto bg-zinc-50 p-4">
						<svg
							class={`h-[680px] min-w-[920px] rounded-md border border-zinc-200 bg-white ${graphPanning ? 'cursor-grabbing' : 'cursor-grab'}`}
							viewBox="0 0 640 460"
							role="img"
							aria-label="LOINC relationship graph"
							on:pointerdown={startGraphPan}
						>
							<defs>
								<marker id="graph-arrow" markerHeight="8" markerWidth="8" orient="auto" refX="7" refY="3">
									<path d="M0,0 L0,6 L8,3 z" fill="#a1a1aa"></path>
								</marker>
							</defs>
							<g transform={`translate(${graphPanX} ${graphPanY}) scale(${graphZoom})`}>
							{#each graphConcepts() as concept, i}
								<line x1="320" y1="230" x2={graphX(i, graphConcepts().length, 150)} y2={graphY(i, graphConcepts().length, 150)} stroke="#d4d4d8" stroke-width="1.5" marker-end="url(#graph-arrow)" />
								{#each graphRelatedTerms(concept) as related, j}
									<line x1={graphX(i, graphConcepts().length, 150)} y1={graphY(i, graphConcepts().length, 150)} x2={graphRelatedX(i, graphConcepts().length, j, graphRelatedTerms(concept).length)} y2={graphRelatedY(i, graphConcepts().length, j, graphRelatedTerms(concept).length)} stroke="#e4e4e7" stroke-width="1" />
								{/each}
							{/each}
							<circle cx="320" cy="230" r="48" fill="#18181b"></circle>
							<text x="320" y="225" text-anchor="middle" class="fill-white font-mono text-[14px] font-semibold">{selectedTerm.loincNum}</text>
							<text x="320" y="244" text-anchor="middle" class="fill-zinc-300 text-[10px]">selected term</text>
							{#each graphConcepts() as concept, i}
								<g
									role="button"
									tabindex="0"
									class="cursor-pointer outline-none"
									on:click={(event) => {
										event.stopPropagation();
										toggleGraphConcept(concept);
									}}
									on:keydown={(event) => handleGraphKey(event, () => toggleGraphConcept(concept))}
								>
									<circle
										cx={graphX(i, graphConcepts().length, 150)}
										cy={graphY(i, graphConcepts().length, 150)}
										r={graphConceptKey(concept) === activeGraphConceptCode ? 43 : 35}
										fill={graphConceptKey(concept) === activeGraphConceptCode ? '#e4e4e7' : '#f4f4f5'}
										stroke={graphConceptKey(concept) === activeGraphConceptCode ? '#18181b' : '#a1a1aa'}
										stroke-width={graphConceptKey(concept) === activeGraphConceptCode ? 2 : 1}
									></circle>
									<text x={graphX(i, graphConcepts().length, 150)} y={graphY(i, graphConcepts().length, 150) - 4} text-anchor="middle" class="pointer-events-none fill-zinc-800 text-[10px] font-semibold">{concept.kind}</text>
									<text x={graphX(i, graphConcepts().length, 150)} y={graphY(i, graphConcepts().length, 150) + 11} text-anchor="middle" class="pointer-events-none fill-zinc-500 text-[9px]">{concept.relatedTotal}</text>
								</g>
								{#each graphRelatedTerms(concept) as related, j}
									<g
										role="button"
										tabindex="0"
										class="cursor-pointer outline-none"
										on:click={(event) => {
											event.stopPropagation();
											openTerm(related.loincNum);
										}}
										on:keydown={(event) => handleGraphKey(event, () => openTerm(related.loincNum))}
									>
										<circle cx={graphRelatedX(i, graphConcepts().length, j, graphRelatedTerms(concept).length)} cy={graphRelatedY(i, graphConcepts().length, j, graphRelatedTerms(concept).length)} r="24" fill="#ffffff" stroke="#d4d4d8"></circle>
										<text x={graphRelatedX(i, graphConcepts().length, j, graphRelatedTerms(concept).length)} y={graphRelatedY(i, graphConcepts().length, j, graphRelatedTerms(concept).length) + 3} text-anchor="middle" class="pointer-events-none fill-zinc-700 font-mono text-[8px]">{related.loincNum}</text>
									</g>
								{/each}
							{/each}
							</g>
						</svg>
					</div>
					<aside class="min-h-0 overflow-auto border-t border-zinc-200 p-4 lg:border-l lg:border-t-0">
						<div class="mb-3 flex items-center justify-between gap-2">
							<h3 class="text-xs font-semibold uppercase tracking-wide text-zinc-500">Visible concepts</h3>
							<span class="text-xs text-zinc-500">{graphConcepts().length} of {sharedConcepts().length}</span>
						</div>
						<div class="flex flex-col gap-3">
							{#if graphConcepts().length === 0}
								<EmptyState title="No graph links" body="This term has no shared concept neighborhoods to draw." />
							{:else}
								{#each graphConcepts() as concept}
									<section class={`rounded-md border p-3 ${graphConceptKey(concept) === activeGraphConceptCode ? 'border-zinc-900 bg-zinc-50' : 'border-zinc-200'}`}>
										<div class="break-words text-sm font-medium text-zinc-950">{accessoryTitle(concept)}</div>
										<div class="mt-1 flex flex-wrap gap-2 text-xs text-zinc-500">
											<span>{concept.kind}</span>
											{#if concept.code}<span class="font-mono">{concept.code}</span>{/if}
											<span>{concept.relatedTotal.toLocaleString()} other terms</span>
										</div>
										<div class="mt-2 flex flex-wrap gap-3">
											<button type="button" class="text-xs font-medium text-zinc-700 hover:underline" on:click={() => toggleGraphConcept(concept)}>{graphConceptKey(concept) === activeGraphConceptCode ? 'Collapse node' : 'Expand node'}</button>
											<button type="button" class="text-xs font-medium text-zinc-700 hover:underline" on:click={() => browseConcept(concept)}>Browse concept</button>
										</div>
										{#if graphRelatedTerms(concept).length}
											<div class="mt-2 flex flex-col gap-1.5">
												{#each graphRelatedTerms(concept) as related}
													<button type="button" class="rounded-md bg-zinc-50 px-2 py-1.5 text-left text-xs hover:bg-zinc-100" on:click={() => openTerm(related.loincNum)}>
														<span class="font-mono font-semibold text-zinc-900">{related.loincNum}</span>
														<span class="mt-0.5 block break-words text-zinc-600">{termSummaryLabel(related)}</span>
													</button>
												{/each}
											</div>
										{/if}
									</section>
								{/each}
							{/if}
						</div>
					</aside>
				</div>
			</section>
		</div>
	{/if}
</main>
