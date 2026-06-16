<script lang="ts">
	import { onMount } from 'svelte';
	import {
		BookOpen,
		ChevronDown,
		ChevronRight,
		Database,
		FilterX,
		HelpCircle,
		KeyRound,
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
	import Checkbox from '$lib/components/Checkbox.svelte';
	import DetailField from '$lib/components/DetailField.svelte';
	import EmptyState from '$lib/components/EmptyState.svelte';
	import FacetGroup from '$lib/components/FacetGroup.svelte';
	import Field from '$lib/components/Field.svelte';
	import FilterChip from '$lib/components/FilterChip.svelte';
	import HierarchyTree from '$lib/components/HierarchyTree.svelte';
	import Input from '$lib/components/Input.svelte';
	import MultiSelectDropdown from '$lib/components/MultiSelectDropdown.svelte';
	import Select from '$lib/components/Select.svelte';
	import ClinicalRelationshipLanes from '$lib/components/ClinicalRelationshipLanes.svelte';
	import RelationshipGraph from '$lib/components/RelationshipGraph.svelte';
	import * as Resizable from '$lib/components/ui/resizable';
	import type { PaneAPI } from 'paneforge';
	import {
		browseAccessories,
		getCacheStats,
		getFacets,
		getHierarchyNode,
		getHierarchyParents,
		getLocalSearchStatus,
		getOfficialCredentialStatus,
		getTerm,
		getTermRelationships,
		getVersion,
		localLuceneSearch,
		officialSearch,
		rebuildLocalSearch,
		searchTerms,
		deleteOfficialCredentials as deleteOfficialCredentialsRequest,
		uploadReleaseZip,
		type AccessoryBrowseResponse,
		type AccessoryRecord,
		type CacheStats,
		type Facets,
		type OfficialCredentialStatus,
		type OfficialSearchResponse,
		type SearchResult,
		type Term,
		type TermAccessory,
		type RelationshipConcept,
		type TermRelationshipGraph,
		type HierarchyNode,
		type LocalSearchResponse,
		type LocalSearchScope,
		type LocalSearchStatus,
		type TermSummary,
		type VersionInfo,
	} from '$lib/api';

	type BrowseMode = 'hierarchy' | 'facets' | 'rank' | 'relationships' | 'official' | 'advanced';
	type ResultsColumnKey = 'loinc' | 'name' | 'status' | 'rank' | 'axes';

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
	let searchSort: 'relevance' | 'usage' = 'relevance';
	let hierarchyNodeId = '';
	let hierarchyLabel = '';
	let results: SearchResult[] = [];
	let total = 0;
	let facets: Facets = emptyFacets;
	let cacheStats: CacheStats | null = null;
	let versionInfo: VersionInfo | null = null;
	let footerTotal: number | null = null;
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
	let activeView: 'browse' | 'loader' | 'accessories' | 'hierarchy' | 'official' | 'advanced' = 'browse';
	let detailOpen = false;
	let sharedConceptsOpen = false;
	let graphViewerOpen = false;
	let relationshipViewMode: 'clinical' | 'explore' = 'clinical';
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
	let columnWidths: Record<ResultsColumnKey, number> = {
		loinc: 96,
		name: 520,
		status: 112,
		rank: 96,
		axes: 220,
	};
	let advancedColumnWidths: Record<string, number> = {
		key: 128,
		longCommonName: 420,
		component: 220,
		system: 150,
		class: 160,
		status: 120,
		partName: 360,
		partTypeName: 180,
		answerListName: 360,
		answerCount: 120,
		loincCount: 120,
		groupName: 360,
		archetype: 220,
	};
	let resizingColumn: string | null = null;
	let resizingTable: 'results' | 'advanced' | null = null;
	let tableResizeStartX = 0;
	let tableResizeStartWidth = 0;
	let accessoryKind = 'part-primary';
	let accessoryQuery = '';
	let accessoryOffset = 0;
	let accessoryLoading = false;
	let accessoryResults: AccessoryRecord[] = [];
	let accessoryTotal = 0;
	let officialScope: 'loincs' | 'answerlists' | 'parts' | 'groups' = 'loincs';
	let officialQuery = '';
	let officialRows = 10;
	let officialOffset = 0;
	let officialSortOrder = '';
	let officialLanguage = 0;
	let officialIncludeFilterCounts = true;
	let officialUsername = '';
	let officialPassword = '';
	let officialRemember = false;
	let officialUseSavedCredentials = false;
	let officialQueryField = '';
	let officialQueryOperator = 'field';
	let officialQueryValue = '';
	let officialQuerySecondValue = '';
	let officialLoading = false;
	let officialCredentialLoading = false;
	let officialCredentialStatus: OfficialCredentialStatus | null = null;
	let officialResult: OfficialSearchResponse | null = null;
	let officialRawOpen = false;
	let localLuceneScope: LocalSearchScope = 'loincs';
	let localLuceneQuery = '';
	let localLuceneLimit = 25;
	let localLuceneOffset = 0;
	let localLuceneLoading = false;
	let localLuceneRebuilding = false;
	let localLuceneStatus: LocalSearchStatus | null = null;
	let localLuceneResult: LocalSearchResponse | null = null;
	let advancedSearchHelpOpen = false;
	let advancedFiltersOpen = false;
	let advancedQueryField = 'Component';
	let advancedQueryOperator = 'field';
	let advancedQueryValue = '';
	let advancedQuerySecondValue = '';

	const limit = 50;
	const accessoryLimit = 50;
	const officialScopes = [
		{ value: 'loincs', label: 'LOINCs' },
		{ value: 'answerlists', label: 'Answer lists' },
		{ value: 'parts', label: 'Parts' },
		{ value: 'groups', label: 'Groups' },
	] as const;
	const officialLoincFields = [
		'LOINC',
		'Component',
		'Property',
		'Timing',
		'System',
		'Scale',
		'Method',
		'Class',
		'AllowMethodSpecific',
		'AnswerList',
		'AnswerListId',
		'AnswerListName',
		'AnswerListType',
		'AskAtOrderEntry',
		'AssociatedObservations',
		'AttachmentUnitsRequired',
		'Categorization',
		'ChangeReasonPublic',
		'ChngType',
		'ClassHierarchy',
		'CommonOrder',
		'CommonLabResult',
		'ComponentHierarchy',
		'ComponentWordCount',
		'CoreComponent',
		'Description',
		'DisplayName',
		'ExUCUMunits',
		'ExUnits',
		'Formula',
		'HL7AttachmentStructure',
		'HL7FieldSubId',
		'LabTest',
		'LForms',
		'LongName',
		'MapToLOINC',
		'MassProperty',
		'MethodHierarchy',
		'Methodless',
		'MultiAxialHierarchy',
		'NonroutineChallenge',
		'OrderObs',
		'OrderRank',
		'OtherCopyright',
		'PanelType',
		'Pharma',
		'Punctuation',
		'Rank',
		'Ranked',
		'RelatedCodes',
		'ShortName',
		'Status',
		'StatusReason',
		'StatusText',
		'SubstanceProperty',
		'SuperSystem',
		'SurveyQuestionSource',
		'SurveyQuestionText',
		'SystemHierarchy',
		'TimeModifier',
		'Type',
		'TypeName',
		'UniversalLabOrders',
		'ValidHL7AttachmentRequest',
		'VersionLastChanged',
	];
	const officialPartFields = [
		'Partnumber',
		'Part',
		'Abbreviation',
		'Article',
		'Book',
		'Citation',
		'ClassList',
		'CreatedOn',
		'Description',
		'DisplayName',
		'Image',
		'MolecularWeight',
		'OriginalForm',
		'PackageInsert',
		'Synonyms',
		'TechnicalBrief',
		'Type',
		'WebContent',
	];
	const officialAnswerListFields = [
		'AnswerList',
		'Name',
		'Description',
		'AnswerCode',
		'AnswerCodeSystem',
		'LOINCAnswerListOID',
		'AnswerCount',
		'AnswerDisplayText',
		'AnswerScore',
		'AnswerSequenceNum',
		'AnswerString',
		'AnswerStringDescription',
		'CodeSystem',
		'ExternalAnswerListOID',
		'ExternalListURL',
		'ExternallyDefined',
		'LoincCount',
		'SourceName',
	];
	const officialGroupFields = ['Group', 'GroupId', 'Name', 'Archetype', 'ParentGroup', 'Status', 'VersionFirstReleased', 'LoincCount'];
	const officialOperators = [
		{ value: 'field', label: 'Field contains' },
		{ value: 'required', label: 'Required +' },
		{ value: 'excluded', label: 'Exclude -' },
		{ value: 'phrase', label: 'Exact phrase' },
		{ value: 'wildcard', label: 'Wildcard *' },
		{ value: 'fuzzy', label: 'Fuzzy ~' },
		{ value: 'proximity', label: 'Proximity "~N"' },
		{ value: 'range-inclusive', label: 'Range [A TO B]' },
		{ value: 'range-exclusive', label: 'Range {A TO B}' },
	];
	const officialResultKeyPriority = [
		'LOINC_NUM',
		'LOINC',
		'LoincNumber',
		'loincNum',
		'LongCommonName',
		'LONG_COMMON_NAME',
		'DisplayName',
		'DISPLAY_NAME',
		'SHORTNAME',
		'ShortName',
		'COMPONENT',
		'Component',
		'PROPERTY',
		'Property',
		'TIME_ASPCT',
		'Timing',
		'SYSTEM',
		'System',
		'SCALE_TYP',
		'Scale',
		'METHOD_TYP',
		'Method',
		'CLASS',
		'Class',
		'STATUS',
		'Status',
		'ORDER_OBS',
		'OrderObs',
		'RANK',
		'Rank',
		'COMMON_TEST_RANK',
		'COMMON_ORDER_RANK',
	];
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
			void loadFooterStats();
			void loadOfficialCredentialStatus();
			void loadLocalLuceneStatus();
			await loadFacets();
			if (activeView === 'accessories' || activeView === 'hierarchy') {
				await loadAccessories(accessoryOffset, true);
			} else if (activeView === 'official' || activeView === 'advanced') {
				updateURL(true);
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
			} else if (activeView === 'official' || activeView === 'advanced') {
				updateURL(true);
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
			window.addEventListener('pointerdown', handleAdvancedResizeStart, true);
			window.addEventListener('mousedown', handleAdvancedResizeStart, true);
			window.addEventListener('pointermove', handleTableResize);
			window.addEventListener('pointerup', stopTableResize);
			window.addEventListener('mousemove', handleTableResize);
			window.addEventListener('mouseup', stopTableResize);
			return () => {
				window.removeEventListener('popstate', handlePopState);
				window.removeEventListener('pointerdown', handleAdvancedResizeStart, true);
				window.removeEventListener('mousedown', handleAdvancedResizeStart, true);
				window.removeEventListener('pointermove', handleTableResize);
				window.removeEventListener('pointerup', stopTableResize);
				window.removeEventListener('mousemove', handleTableResize);
				window.removeEventListener('mouseup', stopTableResize);
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
		activeView === 'advanced'
			? 'advanced'
			: activeView === 'official'
			? 'official'
			: activeView === 'hierarchy'
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
					: currentBrowseMode === 'advanced'
						? 'Advanced Search'
						: currentBrowseMode === 'official'
						? 'Official API'
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
		searchSort = params.get('sort') === 'usage' ? 'usage' : 'relevance';
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
		} else if (mode === 'official') {
			activeView = 'official';
		} else if (mode === 'advanced') {
			activeView = 'advanced';
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
		else if (activeView === 'official' || activeView === 'advanced') {
			params.set('mode', activeView === 'advanced' ? 'advanced' : 'official');
			const nextURL = `${window.location.pathname}?${params.toString()}`;
			if (replace) {
				window.history.replaceState(null, '', nextURL);
			} else {
				window.history.pushState(null, '', nextURL);
			}
			return;
		}
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
		if (searchSort === 'usage') params.set('sort', 'usage');
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

	async function loadFooterStats() {
		try {
			const response = await searchTerms({ limit: 1, offset: 0 });
			footerTotal = response.total;
			cacheStats = await getCacheStats();
		} catch {
			footerTotal = null;
		}
	}

	async function loadOfficialCredentialStatus() {
		officialCredentialLoading = true;
		try {
			officialCredentialStatus = await getOfficialCredentialStatus();
		} catch (err) {
			officialCredentialStatus = { saved: false, usable: false, message: errorMessage(err) };
		} finally {
			officialCredentialLoading = false;
		}
	}

	async function deleteOfficialCredentials() {
		officialCredentialLoading = true;
		error = '';
		try {
			officialCredentialStatus = await deleteOfficialCredentialsRequest();
			officialUseSavedCredentials = false;
		} catch (err) {
			error = errorMessage(err);
		} finally {
			officialCredentialLoading = false;
		}
	}

	async function runOfficialSearch() {
		officialLoading = true;
		error = '';
		officialRawOpen = false;
		try {
			officialResult = await officialSearch({
				scope: officialScope,
				query: officialQuery.trim(),
				rows: officialRows,
				offset: officialOffset,
				sortorder: officialSortOrder.trim(),
				language: officialLanguage,
				includefiltercounts: officialIncludeFilterCounts,
				username: officialUseSavedCredentials ? undefined : officialUsername.trim(),
				password: officialUseSavedCredentials ? undefined : officialPassword,
				remember: officialUseSavedCredentials ? false : officialRemember,
				useSavedCredentials: officialUseSavedCredentials,
			});
			if (officialRemember && !officialUseSavedCredentials) {
				officialPassword = '';
				await loadOfficialCredentialStatus();
			}
			updateURL();
		} catch (err) {
			error = errorMessage(err);
		} finally {
			officialLoading = false;
		}
	}

	async function loadLocalLuceneStatus() {
		try {
			localLuceneStatus = await getLocalSearchStatus();
		} catch (err) {
			localLuceneStatus = {
				state: 'error',
				indexPath: '',
				docCount: 0,
				message: errorMessage(err),
			};
		}
	}

	async function rebuildLocalLuceneIndex() {
		localLuceneRebuilding = true;
		error = '';
		try {
			localLuceneStatus = await rebuildLocalSearch();
		} catch (err) {
			error = errorMessage(err);
			await loadLocalLuceneStatus();
		} finally {
			localLuceneRebuilding = false;
		}
	}

	async function runLocalLuceneSearch() {
		localLuceneLoading = true;
		error = '';
		try {
			localLuceneResult = await localLuceneSearch({
				scope: localLuceneScope,
				query: localLuceneQuery.trim(),
				limit: localLuceneLimit,
				offset: localLuceneOffset,
			});
			await loadLocalLuceneStatus();
			updateURL();
		} catch (err) {
			error = errorMessage(err);
			await loadLocalLuceneStatus();
		} finally {
			localLuceneLoading = false;
		}
	}

	function runAdvancedSearchFromStart() {
		localLuceneOffset = 0;
		void runLocalLuceneSearch();
	}

	function advancedSearchPreviousPage() {
		localLuceneOffset = Math.max(0, localLuceneOffset - localLuceneLimit);
		void runLocalLuceneSearch();
	}

	function advancedSearchNextPage() {
		localLuceneOffset = localLuceneOffset + localLuceneLimit;
		void runLocalLuceneSearch();
	}

	async function runSearch(nextOffset = 0, replaceURL = false, switchToBrowse = true, preserveSelectedTerm = false) {
		loading = true;
		error = '';
		if (switchToBrowse) activeView = 'browse';
		offset = nextOffset;
		if (nextOffset === 0 && selectedTerm && !preserveSelectedTerm) {
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
				sort: searchSort,
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
		relationshipViewMode = 'clinical';
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
		searchSort = 'relevance';
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
		searchSort = 'usage';
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
		if (activeView === 'advanced') return 'advanced';
		if (activeView === 'official') return 'official';
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
		if (mode === 'advanced') return 'Advanced Search';
		if (mode === 'official') return 'Official API';
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

	function setSearchSort(sort: 'relevance' | 'usage') {
		searchSort = sort;
		runSearch(0);
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

	function advancedColumnDefaultWidth(column: string) {
		if (column === 'key') return 128;
		if (column === 'longCommonName') return 420;
		if (column === 'component' || column === 'archetype') return 220;
		if (column === 'partName' || column === 'answerListName' || column === 'groupName') return 360;
		if (column === 'system' || column === 'class' || column === 'partTypeName') return 160;
		return 120;
	}

	function advancedColumnMinimumWidth(column: string) {
		if (column === 'longCommonName') return 240;
		if (column === 'partName' || column === 'answerListName' || column === 'groupName') return 220;
		if (column === 'component' || column === 'archetype') return 160;
		if (column === 'key') return 92;
		return 88;
	}

	function advancedColumnWidth(column: string) {
		return advancedColumnWidths[column] ?? advancedColumnDefaultWidth(column);
	}

	function advancedResultsTableWidth() {
		return localLuceneColumns().reduce((width, column) => width + advancedColumnWidth(column), 0);
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

	function officialPayloadRows(payload: unknown): Record<string, unknown>[] {
		if (Array.isArray(payload)) return payload.filter(isRecord);
		if (isRecord(payload)) {
			for (const key of ['results', 'Results', 'items', 'Items', 'data', 'Data', 'docs', 'Docs']) {
				const value = payload[key];
				if (Array.isArray(value)) return value.filter(isRecord);
			}
			return [payload];
		}
		return [];
	}

	function officialPayloadKeys(rows: Record<string, unknown>[]) {
		const sampledRows = rows.slice(0, 25);
		const availableKeys: string[] = [];
		for (const row of sampledRows) {
			for (const key of Object.keys(row)) {
				if (!availableKeys.includes(key)) availableKeys.push(key);
			}
		}
		const keys: string[] = [];
		const addKey = (key: string) => {
			if (keys.includes(key)) return;
			if (!availableKeys.includes(key)) return;
			if (!officialColumnHasValue(sampledRows, key)) return;
			keys.push(key);
		};
		for (const preferred of officialResultKeyPriority) {
			addKey(preferred);
		}
		for (const key of availableKeys) {
			addKey(key);
			if (keys.length >= 12) break;
		}
		return keys.length > 0 ? keys : availableKeys.slice(0, 8);
	}

	function officialColumnHasValue(rows: Record<string, unknown>[], key: string) {
		return rows.some((row) => {
			const value = row[key];
			if (value === null || value === undefined || value === '') return false;
			if (Array.isArray(value)) return value.length > 0;
			if (typeof value === 'object') return Object.keys(value as Record<string, unknown>).length > 0;
			return true;
		});
	}

	function officialColumnClass(key: string) {
		const normalized = key.toLowerCase().replace(/[_\-\s]/g, '');
		if (normalized.includes('longcommonname')) return 'min-w-[22rem]';
		if (normalized.includes('displayname')) return 'min-w-[16rem]';
		if (normalized.includes('shortname') || normalized === 'component') return 'min-w-[14rem]';
		if (normalized.includes('loinc')) return 'min-w-[7rem]';
		if (normalized === 'methodtyp' || normalized === 'method') return 'min-w-[8rem]';
		if (normalized === 'class' || normalized === 'system' || normalized === 'status') return 'min-w-[7rem]';
		return 'min-w-[9rem]';
	}

	function officialHeaderLabel(key: string) {
		return key.replace(/_/g, ' ');
	}

	function officialValue(value: unknown) {
		if (value === null || value === undefined || value === '') return '-';
		if (typeof value === 'object') return JSON.stringify(value);
		return String(value);
	}

	function localLuceneColumns() {
		if (localLuceneScope === 'parts') return ['key', 'partName', 'partTypeName', 'status'];
		if (localLuceneScope === 'answerlists') return ['key', 'answerListName', 'answerCount', 'loincCount'];
		if (localLuceneScope === 'groups') return ['key', 'groupName', 'archetype', 'status'];
		return ['key', 'longCommonName', 'component', 'system', 'class', 'status'];
	}

	function localLuceneColumnLabel(key: string) {
		const labels: Record<string, string> = {
			key: 'ID',
			longCommonName: 'Long name',
			partName: 'Part',
			partTypeName: 'Type',
			answerListName: 'Answer list',
			answerCount: 'Answers',
			loincCount: 'LOINCs',
			groupName: 'Group',
			archetype: 'Archetype',
		};
		return labels[key] ?? key.replace(/([A-Z])/g, ' $1').replace(/^./, (value) => value.toUpperCase());
	}

	function localLuceneValue(row: { key: string; score: number; result: Record<string, unknown> }, key: string) {
		if (key === 'key') return row.key;
		return officialValue(row.result?.[key]);
	}

	function localLucenePrimaryLOINC(row: { key: string; result: Record<string, unknown> }) {
		if (localLuceneScope !== 'loincs') return '';
		const value = row.result?.loincNum;
		return typeof value === 'string' ? value : row.key;
	}

	function officialRowLOINC(row: Record<string, unknown>) {
		for (const key of Object.keys(row)) {
			if (isOfficialLOINCField(key)) {
				const loincNum = officialLOINCFromValue(row[key]);
				if (loincNum) return loincNum;
			}
		}
		for (const value of Object.values(row)) {
			const loincNum = officialLOINCFromValue(value);
			if (loincNum) return loincNum;
		}
		return '';
	}

	function isOfficialLOINCField(key: string) {
		const normalized = key.toLowerCase().replace(/[_\-\s]/g, '');
		return normalized === 'loinc' || normalized === 'loincnum' || normalized === 'loincnumber' || normalized === 'loincode' || normalized === 'code' || normalized.includes('loinc');
	}

	function officialLOINCFromValue(value: unknown): string {
		if (typeof value === 'string') {
			return value.match(/\b\d{1,7}-\d\b/i)?.[0]?.toUpperCase() ?? '';
		}
		if (Array.isArray(value)) {
			for (const item of value) {
				const loincNum = officialLOINCFromValue(item);
				if (loincNum) return loincNum;
			}
		}
		if (isRecord(value)) {
			for (const child of Object.values(value)) {
				const loincNum = officialLOINCFromValue(child);
				if (loincNum) return loincNum;
			}
		}
		return '';
	}

	function officialLocalMatch(loincNum: string) {
		if (!loincNum) return null;
		return officialResult?.local?.matches?.[loincNum] ?? null;
	}

	function officialLocalSummary() {
		const local = officialResult?.local;
		if (!local) return '';
		if (local.loincNums.length === 0) return local.message || 'No LOINC numbers found in the official payload.';
		if (!local.available) return local.message || 'Local database matching is unavailable.';
		return `${local.matched} of ${local.loincNums.length} official LOINC codes found in the local database.`;
	}

	function officialRawJSON() {
		return JSON.stringify(officialResult?.payload ?? {}, null, 2);
	}

	function isRecord(value: unknown): value is Record<string, unknown> {
		return typeof value === 'object' && value !== null && !Array.isArray(value);
	}

	function officialSavedLabel() {
		if (officialCredentialLoading) return 'Checking saved credentials...';
		if (!officialCredentialStatus) return 'Saved credential status unknown';
		if (officialCredentialStatus.saved && officialCredentialStatus.usable) {
			return `Saved credentials ready${officialCredentialStatus.maskedUsername ? ` for ${officialCredentialStatus.maskedUsername}` : ''}`;
		}
		return officialCredentialStatus.message || 'No saved credentials';
	}

	function officialFieldsForScope() {
		return Array.from(new Set([...officialLoincFields, ...officialPartFields, ...officialAnswerListFields]));
	}

	function advancedFieldsForScope() {
		if (localLuceneScope === 'parts') return ['Part', 'Partnumber', 'Name', 'DisplayName', 'Type', 'Abbreviation', 'Synonyms', 'Description'];
		if (localLuceneScope === 'answerlists') return ['AnswerList', 'Name', 'Description', 'AnswerCode', 'AnswerDisplayText', 'AnswerString', 'SourceName'];
		if (localLuceneScope === 'groups') return officialGroupFields;
		return ['LOINC', 'Component', 'Property', 'Timing', 'System', 'Scale', 'Method', 'Class', 'LongName', 'ShortName', 'DisplayName', 'Status'];
	}

	function advancedFieldExamples() {
		if (localLuceneScope === 'parts') return ['Part:glucose', 'Type:COMPONENT', 'Synonyms:serum'];
		if (localLuceneScope === 'answerlists') return ['AnswerDisplayText:positive', 'AnswerCode:LA*', 'Name:ordinal'];
		if (localLuceneScope === 'groups') return ['Name:chemistry', 'Status:ACTIVE', 'Archetype:panel'];
		return ['Component:morphine', 'System:urine', 'Class:DRUG/TOX', 'LOINC:80619-?'];
	}

	function appendAdvancedQueryText(text: string) {
		const next = text.trim();
		if (!next) return;
		localLuceneQuery = [localLuceneQuery.trim(), next].filter(Boolean).join(' ');
	}

	function appendAdvancedQueryToken(token: string) {
		appendAdvancedQueryText(token);
	}

	function clearAdvancedBuilder() {
		advancedQueryValue = '';
		advancedQuerySecondValue = '';
	}

	function appendAdvancedQueryClause() {
		const clause = buildAdvancedQueryClause();
		if (!clause) return;
		appendAdvancedQueryText(clause);
		clearAdvancedBuilder();
	}

	function buildAdvancedQueryClause() {
		const field = advancedQueryField.trim();
		const value = advancedQueryValue.trim();
		const second = advancedQuerySecondValue.trim();
		if (!value) return '';
		const fieldPrefix = field ? `${field}:` : '';
		if (advancedQueryOperator === 'required') return `+${fieldPrefix}${value}`;
		if (advancedQueryOperator === 'excluded') return `-${fieldPrefix}${value}`;
		if (advancedQueryOperator === 'phrase') return `${fieldPrefix}"${value}"`;
		if (advancedQueryOperator === 'wildcard') return `${fieldPrefix}${value.includes('*') || value.includes('?') ? value : `${value}*`}`;
		if (advancedQueryOperator === 'fuzzy') return `${fieldPrefix}${value.endsWith('~') ? value : `${value}~`}`;
		if (advancedQueryOperator === 'proximity') return `${fieldPrefix}"${value}"~${second || '1'}`;
		if (advancedQueryOperator === 'range-inclusive') return `${fieldPrefix}[${value} TO ${second || value}]`;
		if (advancedQueryOperator === 'range-exclusive') return `${fieldPrefix}{${value} TO ${second || value}}`;
		return `${fieldPrefix}${value}`;
	}

	function appendOfficialQueryOption() {
		const clause = buildOfficialQueryClause();
		if (!clause) return;
		officialQuery = [officialQuery.trim(), clause].filter(Boolean).join(' ');
		officialQueryValue = '';
		officialQuerySecondValue = '';
	}

	function buildOfficialQueryClause() {
		const field = officialQueryField.trim();
		const value = officialQueryValue.trim();
		const second = officialQuerySecondValue.trim();
		if (!value) return '';
		const fieldPrefix = field ? `${field}:` : '';
		if (officialQueryOperator === 'required') return `+${fieldPrefix}${value}`;
		if (officialQueryOperator === 'excluded') return `-${fieldPrefix}${value}`;
		if (officialQueryOperator === 'phrase') return `${fieldPrefix}"${value}"`;
		if (officialQueryOperator === 'wildcard') return `${fieldPrefix}${value.includes('*') || value.includes('?') ? value : `${value}*`}`;
		if (officialQueryOperator === 'fuzzy') return `${fieldPrefix}${value.endsWith('~') ? value : `${value}~`}`;
		if (officialQueryOperator === 'proximity') return `${fieldPrefix}"${value}"~${second || '1'}`;
		if (officialQueryOperator === 'range-inclusive') return `${fieldPrefix}[${value} TO ${second || value}]`;
		if (officialQueryOperator === 'range-exclusive') return `${fieldPrefix}{${value} TO ${second || value}}`;
		return `${fieldPrefix}${value}`;
	}

	function startColumnResize(column: ResultsColumnKey, event: PointerEvent | MouseEvent) {
		event.preventDefault();
		event.stopPropagation();
		resizingTable = 'results';
		resizingColumn = column;
		tableResizeStartX = event.clientX;
		tableResizeStartWidth = columnWidths[column];
	}

	function handleAdvancedResizeStart(event: PointerEvent | MouseEvent) {
		const target = event.target instanceof Element ? event.target : null;
		const handle = target?.closest('[data-advanced-resize-column]') as HTMLElement | null;
		const column = handle?.dataset.advancedResizeColumn;
		if (!column) return;
		event.preventDefault();
		event.stopPropagation();
		const table = handle.closest('table') as HTMLTableElement | null;
		const columns = localLuceneColumns();
		const columnIndex = columns.indexOf(column);
		const startX = event.clientX;
		const startWidth = advancedColumnWidth(column);
		const updateWidth = (moveEvent: PointerEvent | MouseEvent) => {
			const nextWidth = Math.max(advancedColumnMinimumWidth(column), startWidth + moveEvent.clientX - startX);
			advancedColumnWidths = { ...advancedColumnWidths, [column]: nextWidth };
			const col = table?.querySelectorAll('col')[columnIndex] as HTMLTableColElement | undefined;
			if (col) col.style.width = `${nextWidth}px`;
			if (table) {
				const nextTableWidth = columns.reduce((width, item) => width + (item === column ? nextWidth : advancedColumnWidth(item)), 0);
				table.style.minWidth = `${nextTableWidth}px`;
			}
		};
		const stopResize = () => {
			window.removeEventListener('pointermove', updateWidth);
			window.removeEventListener('mousemove', updateWidth);
			window.removeEventListener('pointerup', stopResize);
			window.removeEventListener('mouseup', stopResize);
		};
		window.addEventListener('pointermove', updateWidth);
		window.addEventListener('mousemove', updateWidth);
		window.addEventListener('pointerup', stopResize, { once: true });
		window.addEventListener('mouseup', stopResize, { once: true });
	}

	function handleTableResize(event: PointerEvent | MouseEvent) {
		if (!resizingTable || !resizingColumn) return;
		if (resizingTable === 'advanced') {
			const nextWidth = Math.max(advancedColumnMinimumWidth(resizingColumn), tableResizeStartWidth + event.clientX - tableResizeStartX);
			advancedColumnWidths = { ...advancedColumnWidths, [resizingColumn]: nextWidth };
			return;
		}
		const column = resizingColumn as ResultsColumnKey;
		const minimums: Record<ResultsColumnKey, number> = {
			loinc: 78,
			name: 220,
			status: 88,
			rank: 74,
			axes: 140,
		};
		const nextWidth = Math.max(minimums[column], tableResizeStartWidth + event.clientX - tableResizeStartX);
		columnWidths = { ...columnWidths, [column]: nextWidth };
	}

	function stopTableResize() {
		resizingTable = null;
		resizingColumn = null;
	}

	function openLoader() {
		activeView = 'loader';
		resultsFullscreen = false;
		updateURL(false);
	}

	function openOfficialAPI() {
		activeView = 'official';
		resultsFullscreen = false;
		mobileBrowseMenuOpen = false;
		void loadOfficialCredentialStatus();
		updateURL(false);
	}

	function openAdvancedSearch() {
		activeView = 'advanced';
		resultsFullscreen = false;
		mobileBrowseMenuOpen = false;
		void loadLocalLuceneStatus();
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

	function accessoryMeta(item: TermAccessory) {
		const sequence = item.fields?.sequence ? `Seq ${item.fields.sequence}` : '';
		const entryType = item.fields?.entryType || item.subtitle || '';
		return [sequence, entryType].filter(Boolean).join(' · ');
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
		return Boolean(
			graph?.outgoingMapTo?.length ||
				graph?.incomingMapTo?.length ||
				graph?.sharedConcepts?.length ||
				graph?.panelMemberships?.length ||
				graph?.panelItems?.length ||
				graph?.hierarchy?.length,
		);
	}

	function accessorySections(term: Term): { title: string; kind: string; items: TermAccessory[] }[] {
		const panelMemberships = (term.panels ?? []).filter((item) => item.kind === 'panel-membership');
		const panelItems = dedupePanelItems((term.panels ?? []).filter((item) => item.kind === 'panel-child'));
		return [
			{ title: 'Parent panels / scales / orders', kind: 'panel-membership', items: panelMemberships },
			{ title: panelItemsTitle(term), kind: 'panel-child', items: panelItems },
			{ title: 'Parts', kind: 'part-primary', items: term.parts ?? [] },
			{ title: 'Answer lists', kind: 'answer-list', items: term.answerLists ?? [] },
			{ title: 'Groups', kind: 'group', items: term.groups ?? [] },
			{ title: 'Hierarchy', kind: 'hierarchy', items: term.hierarchy ?? [] },
		].filter((section) => section.items.length > 0);
	}

	function panelItemsTitle(term: Term) {
		const text = `${term.class} ${term.method} ${term.longCommonName}`.toLowerCase();
		if (text.includes('survey') || text.includes('questionnaire') || text.includes('phq')) return 'Scale / survey items';
		if (term.orderObs?.toLowerCase() === 'order' || term.class?.toLowerCase().includes('panel')) return 'Panel observations';
		return 'Panel children';
	}

	function clinicalRoleBadges(term: Term) {
		const panels = term.panels ?? [];
		const badges: string[] = [];
		if (term.orderObs) badges.push(term.orderObs);
		if (panels.some((item) => item.kind === 'panel-membership')) badges.push('Contained item');
		if (panels.some((item) => item.kind === 'panel-child')) badges.push(panelItemsTitle(term).replace(/s$/, ''));
		if (term.answerLists?.length) badges.push('Answer list');
		if (`${term.class} ${term.method} ${term.longCommonName}`.toLowerCase().match(/survey|questionnaire|phq/)) badges.push('Survey');
		return [...new Set(badges.filter(Boolean))];
	}

	function dedupePanelItems(items: TermAccessory[]) {
		const seen = new Map<string, TermAccessory>();
		for (const item of items) {
			const key = item.code || accessoryTitle(item);
			const existing = seen.get(key);
			if (!existing) {
				seen.set(key, item);
				continue;
			}
			const count = Number(existing.fields?.duplicateCount ?? 1);
			seen.set(key, {
				...existing,
				fields: {
					...existing.fields,
					duplicateCount: String(count + 1),
				},
			});
		}
		return [...seen.values()];
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

	function browseHierarchyFromRelationship(nodeId: string, label: string) {
		query = '';
		hierarchyNodeId = nodeId;
		hierarchyLabel = label;
		rankedOnly = false;
		activeView = 'browse';
		graphViewerOpen = false;
		void runSearch(0, false, false, true);
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

	function chooseMobileBrowseMode(mode: 'hierarchy' | 'facets' | 'rank' | 'relationships' | 'official' | 'advanced') {
		mobileBrowseMenuOpen = false;
		if (mode === 'hierarchy') {
			void openHierarchyBrowser();
		} else if (mode === 'facets') {
			openFacetBrowser();
		} else if (mode === 'rank') {
			browseByRank();
		} else if (mode === 'official') {
			openOfficialAPI();
		} else if (mode === 'advanced') {
			openAdvancedSearch();
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

	function directRelationshipCount() {
		if (!selectedTerm) return 0;
		return (
			(selectedTerm.parts?.length ?? 0) +
			(selectedTerm.answerLists?.length ?? 0) +
			(selectedTerm.panels?.length ?? 0) +
			(selectedTerm.groups?.length ?? 0) +
			(selectedTerm.hierarchy?.length ?? 0) +
			(selectedTerm.mapTo?.length ?? 0) +
			(relationshipGraph?.incomingMapTo?.length ?? 0)
		);
	}

	function graphVisibleLimitMax() {
		return Math.max(sharedConcepts().length, Math.ceil(directRelationshipCount() / 2), 8);
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
						<button type="button" class={`flex items-center gap-2 whitespace-nowrap rounded px-2 py-1.5 text-left text-xs ${currentBrowseMode === 'official' ? 'bg-zinc-950 text-white' : 'text-zinc-700 hover:bg-zinc-100'}`} on:click={() => chooseMobileBrowseMode('official')}>
							<KeyRound size={14} />
							Official API
						</button>
						<button type="button" class={`flex items-center gap-2 whitespace-nowrap rounded px-2 py-1.5 text-left text-xs ${currentBrowseMode === 'advanced' ? 'bg-zinc-950 text-white' : 'text-zinc-700 hover:bg-zinc-100'}`} on:click={() => chooseMobileBrowseMode('advanced')}>
							<Search size={14} />
							Advanced Search
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
				<button type="button" class={modeButtonClass('official', currentBrowseMode)} on:click={openOfficialAPI}>
					<KeyRound size={14} />
					Official API
				</button>
				<button type="button" class={modeButtonClass('advanced', currentBrowseMode)} on:click={openAdvancedSearch}>
					<Search size={14} />
					Advanced Search
				</button>
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
						{:else if currentBrowseMode === 'official'}
							<section class="rounded-md border border-zinc-200 bg-white">
								<div class="border-b border-zinc-100 px-2.5 py-2 text-[11px] font-semibold uppercase tracking-wide text-zinc-600">Official scopes</div>
								<div class="flex flex-col gap-1 p-2">
									{#each officialScopes as item}
										<button
											type="button"
											class={`rounded-md px-2 py-1.5 text-left text-xs leading-4 ${officialScope === item.value ? 'bg-zinc-950 text-white' : 'text-zinc-700 hover:bg-zinc-100'}`}
											on:click={() => (officialScope = item.value)}
										>
											{item.label}
										</button>
									{/each}
								</div>
							</section>
							<section class="rounded-md border border-zinc-200 bg-white p-3 text-xs leading-5 text-zinc-600">
								<div class="font-semibold uppercase tracking-wide text-zinc-500">Official syntax</div>
								<p class="mt-1">Use the options builder for all official fielded queries, required or excluded clauses, quoted phrases, wildcards, fuzzy terms, proximity, ranges, and documented LOINC, part, or answer-list fields.</p>
							</section>
						{:else if currentBrowseMode === 'advanced'}
							{#key localLuceneScope}
							<section class="rounded-md border border-zinc-200 bg-white" data-testid="advanced_query_adder">
								<div class="border-b border-zinc-100 px-2.5 py-2 text-[11px] font-semibold uppercase tracking-wide text-zinc-600">Add search term</div>
								<div class="flex flex-col gap-3 p-3">
									<Field label="Field">
										<Select bind:value={advancedQueryField} on:valueChange={(event) => (advancedQueryField = event.detail)}>
											<option value="">Any field</option>
											{#each advancedFieldsForScope() as field}
												<option value={field}>{field}</option>
											{/each}
										</Select>
									</Field>
									<Field label="Operator">
										<Select bind:value={advancedQueryOperator} on:valueChange={(event) => (advancedQueryOperator = event.detail)}>
											{#each officialOperators as operator}
												<option value={operator.value}>{operator.label}</option>
											{/each}
										</Select>
									</Field>
									<Field label="Value">
										<Input bind:value={advancedQueryValue} placeholder={advancedQueryOperator === 'phrase' || advancedQueryOperator === 'proximity' ? 'exact phrase' : 'value'} />
									</Field>
									{#if ['proximity', 'range-inclusive', 'range-exclusive'].includes(advancedQueryOperator)}
										<Field label={advancedQueryOperator === 'proximity' ? 'Distance' : 'Range end'}>
											<Input bind:value={advancedQuerySecondValue} placeholder={advancedQueryOperator === 'proximity' ? '1' : 'end'} />
										</Field>
									{/if}
									<div class="grid grid-cols-2 gap-2">
										<Button type="button" size="sm" on:click={appendAdvancedQueryClause}>Add</Button>
										<Button type="button" variant="outline" size="sm" on:click={clearAdvancedBuilder}>Clear</Button>
									</div>
								</div>
							</section>
							<section class="rounded-md border border-zinc-200 bg-white">
								<div class="border-b border-zinc-100 px-2.5 py-2 text-[11px] font-semibold uppercase tracking-wide text-zinc-600">Logic</div>
								<div class="grid grid-cols-3 gap-2 p-3">
									{#each ['AND', 'OR', 'NOT', '+', '-', '(', ')', '" "', '*'] as token}
										<Button type="button" variant="outline" size="sm" on:click={() => appendAdvancedQueryToken(token)}>{token}</Button>
									{/each}
								</div>
							</section>
							<section class="rounded-md border border-zinc-200 bg-white">
								<div class="border-b border-zinc-100 px-2.5 py-2 text-[11px] font-semibold uppercase tracking-wide text-zinc-600">Examples</div>
								<div class="flex flex-col gap-1 p-2">
									{#each advancedFieldExamples() as example}
										<button
											type="button"
											data-testid="advanced_query_example"
											class="rounded-md px-2 py-1.5 text-left font-mono text-xs leading-4 text-zinc-700 hover:bg-zinc-100"
											on:click={() => appendAdvancedQueryText(example)}
										>
											{example}
										</button>
									{/each}
								</div>
							</section>
							{/key}
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
			{#if activeView === 'advanced'}
				<div class="flex items-center justify-between gap-4 border-b border-zinc-200 px-4 py-3 pr-14 lg:shrink-0" data-testid="advanced_search_header">
					<div class="min-w-0">
						<h2 class="text-sm font-semibold">Advanced Search</h2>
					</div>
					<div class="flex items-center gap-2">
						<Button type="button" variant="outline" size="sm" on:click={() => (advancedSearchHelpOpen = true)}>
							<HelpCircle size={14} />
							Help
						</Button>
						{#if localLuceneStatus?.state !== 'ready'}
							<Button type="button" size="sm" on:click={rebuildLocalLuceneIndex} disabled={localLuceneRebuilding}>
								<RefreshCcw size={14} />
								{localLuceneRebuilding ? 'Building...' : 'Build index'}
							</Button>
						{/if}
					</div>
				</div>
				<div class="lg:min-h-0 lg:flex-1 lg:overflow-auto">
					<div class="flex flex-col gap-4 p-4">
						<section class="rounded-lg border border-zinc-200 bg-white p-4" data-testid="local_lucene_form_card">
							<form class="flex flex-col gap-4" on:submit|preventDefault={runAdvancedSearchFromStart}>
								<div class="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto] md:items-end">
									<Input ariaLabel="Search query" className="h-11 text-base" bind:value={localLuceneQuery} placeholder="morphine AND cutoff" />
									<Button type="submit" className="h-11" disabled={localLuceneLoading || localLuceneStatus?.state !== 'ready'}>
										<Search size={16} />
										{localLuceneLoading ? 'Searching...' : 'Search'}
									</Button>
								</div>
							</form>
						</section>
						<section class="rounded-lg border border-zinc-200 bg-white" data-testid="advanced_filter_row">
							<button
								type="button"
								data-testid="advanced_filter_toggle"
								class="flex w-full items-center justify-between gap-3 px-4 py-3 text-left"
								aria-expanded={advancedFiltersOpen}
								on:click={() => (advancedFiltersOpen = !advancedFiltersOpen)}
							>
								<div class="min-w-0 truncate text-sm">
									<span class="font-semibold">Search options</span>
									<span class="text-xs text-zinc-500"> · {officialScopes.find((item) => item.value === localLuceneScope)?.label ?? 'LOINCs'} · {localLuceneLimit} per page</span>
								</div>
								{#if advancedFiltersOpen}<ChevronDown size={16} />{:else}<ChevronRight size={16} />{/if}
							</button>
							{#if advancedFiltersOpen}
								<div class="grid gap-3 border-t border-zinc-200 px-4 py-3 md:grid-cols-[220px_180px] md:items-end">
									<Field label="Scope">
										<Select
											bind:value={localLuceneScope}
											on:valueChange={(event) => {
												localLuceneScope = event.detail as LocalSearchScope;
												localLuceneOffset = 0;
												localLuceneResult = null;
												advancedQueryField = advancedFieldsForScope()[0] ?? '';
											}}
										>
											{#each officialScopes as item}
												<option value={item.value}>{item.label}</option>
											{/each}
										</Select>
									</Field>
									<Field label="Per page">
										<Select
											value={String(localLuceneLimit)}
											on:valueChange={(event) => {
												localLuceneLimit = Number(event.detail);
												localLuceneOffset = 0;
											}}
										>
											<option value="10">10</option>
											<option value="25">25</option>
											<option value="50">50</option>
											<option value="100">100</option>
										</Select>
									</Field>
								</div>
							{/if}
						</section>

						<section class="rounded-lg border border-zinc-200 bg-white" data-testid="local_lucene_results_window">
							<div class="flex flex-wrap items-center justify-between gap-3 border-b border-zinc-200 px-4 py-3">
								<div>
									<h3 class="text-sm font-semibold">Results</h3>
									<p class="mt-1 text-xs text-zinc-500">
										{#if localLuceneLoading}
											Searching...
										{:else if localLuceneResult}
											{localLuceneResult.total.toLocaleString()} matches; showing {localLuceneOffset + 1}-{Math.min(localLuceneOffset + localLuceneResult.results.length, localLuceneResult.total)}
										{:else if localLuceneStatus?.state !== 'ready'}
											Build the search index before running a query.
										{:else}
											Enter a query to search.
										{/if}
									</p>
								</div>
								{#if localLuceneResult}
									<div class="flex items-center gap-2">
										<Button type="button" variant="outline" size="sm" disabled={localLuceneOffset === 0 || localLuceneLoading} on:click={advancedSearchPreviousPage}>Previous</Button>
										<Button type="button" variant="outline" size="sm" disabled={localLuceneOffset + localLuceneLimit >= localLuceneResult.total || localLuceneLoading} on:click={advancedSearchNextPage}>Next</Button>
									</div>
								{/if}
							</div>
							{#if error && activeView === 'advanced'}
								<div class="border-b border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800">{error}</div>
							{/if}
							{#if localLuceneResult?.warnings?.length}
								<div class="border-b border-amber-200 bg-amber-50 px-4 py-3 text-xs leading-5 text-amber-900">
									{#each localLuceneResult.warnings as warning}
										<div>{warning}</div>
									{/each}
								</div>
							{/if}
							<div class="min-h-[420px] overflow-auto">
								{#if localLuceneLoading}
									<div class="space-y-3 p-4">
										{#each Array(5) as _}<div class="h-14 animate-pulse rounded-md bg-zinc-100"></div>{/each}
									</div>
								{:else if !localLuceneResult}
									<div class="p-4"><EmptyState title="No query yet" body="Enter a search query above." /></div>
								{:else if localLuceneResult.results.length === 0}
									<div class="p-4"><EmptyState title="No matches" body="Try a broader term or another scope." /></div>
								{:else}
									<div class="overflow-x-auto">
										<table class="w-full table-fixed border-collapse text-left text-sm" style={`min-width: ${advancedResultsTableWidth()}px`}>
											<colgroup>
												{#each localLuceneColumns() as column}
													<col style={`width: ${advancedColumnWidth(column)}px`} />
												{/each}
											</colgroup>
											<thead class="bg-zinc-50 text-xs uppercase tracking-wide text-zinc-500">
												<tr>
													{#each localLuceneColumns() as column}
														<th class="relative px-4 py-3 font-medium leading-4">
															<span class="block whitespace-normal break-words pr-2">{localLuceneColumnLabel(column)}</span>
															<button
																type="button"
																class="absolute right-0 top-0 h-full w-2 cursor-col-resize border-r border-transparent hover:border-zinc-400 focus:border-zinc-500 focus:outline-none"
																aria-label={`Resize ${localLuceneColumnLabel(column)} column`}
																data-advanced-resize-column={column}
															></button>
														</th>
													{/each}
												</tr>
											</thead>
											<tbody>
												{#each localLuceneResult.results as row}
													<tr
														class={`border-t border-zinc-100 ${localLuceneScope === 'loincs' ? 'cursor-pointer hover:bg-zinc-50' : ''}`}
														on:click={() => {
															const loincNum = localLucenePrimaryLOINC(row);
															if (loincNum) void openTerm(loincNum);
														}}
													>
														{#each localLuceneColumns() as column}
															<td class="px-4 py-3 align-top text-xs leading-5 text-zinc-700">
																<div class="whitespace-normal break-words [overflow-wrap:anywhere]">{localLuceneValue(row, column)}</div>
															</td>
														{/each}
													</tr>
												{/each}
											</tbody>
										</table>
									</div>
								{/if}
							</div>
						</section>
					</div>
				</div>
			{:else if activeView === 'official'}
				<div class="flex items-center justify-between gap-4 border-b border-zinc-200 px-4 py-3 pr-14 lg:shrink-0" data-testid="official_api_header">
					<div class="min-w-0">
						<h2 class="text-sm font-semibold">Official API</h2>
						<p class="mt-1 text-xs text-zinc-500">Query Regenstrief's official LOINC Search API through the local credential-safe proxy.</p>
					</div>
					<div class="flex items-center gap-2">
						<Button variant="outline" size="sm" on:click={openFacetBrowser}>Back to local search</Button>
					</div>
				</div>
					<div class="lg:min-h-0 lg:flex-1 lg:overflow-auto">
						<div class="flex flex-col gap-4 p-4">
							<section class="rounded-lg border border-zinc-200 bg-white p-4" data-testid="official_api_form_card">
								<form class="flex flex-col gap-4" on:submit|preventDefault={runOfficialSearch}>
									<div class="grid gap-3 md:grid-cols-[180px_minmax(0,1fr)]">
										<Field label="Scope">
											<Select
												bind:value={officialScope}
												on:valueChange={(event) => {
													officialScope = event.detail as typeof officialScope;
													officialQueryField = '';
												}}
											>
												{#each officialScopes as item}
													<option value={item.value}>{item.label}</option>
												{/each}
											</Select>
										</Field>
										<Field label="Query">
											<Input bind:value={officialQuery} placeholder="Component:glucose System:blood" />
										</Field>
									</div>
									<div class="rounded-md border border-zinc-200 bg-zinc-50 p-3" data-testid="official_api_query_options">
										<div class="flex flex-wrap items-center justify-between gap-2">
											<div>
												<div class="text-xs font-semibold uppercase tracking-wide text-zinc-500">Official search options</div>
												<div class="mt-1 text-xs text-zinc-600">Build fielded, boolean, phrase, wildcard, fuzzy, proximity, and range clauses supported by the official search syntax.</div>
											</div>
											<Badge variant="secondary">{officialFieldsForScope().length ? `${officialFieldsForScope().length} fields` : 'Free text'}</Badge>
										</div>
										<div class="mt-3 grid gap-3 lg:grid-cols-[minmax(0,1fr)_180px_minmax(0,1fr)_minmax(0,1fr)_auto]">
											<Field label="Field">
												<Select bind:value={officialQueryField} on:valueChange={(event) => (officialQueryField = event.detail)}>
													<option value="">Any field</option>
													{#each officialFieldsForScope() as field}
														<option value={field}>{field}</option>
													{/each}
												</Select>
											</Field>
											<Field label="Operator">
												<Select bind:value={officialQueryOperator} on:valueChange={(event) => (officialQueryOperator = event.detail)}>
													{#each officialOperators as operator}
														<option value={operator.value}>{operator.label}</option>
													{/each}
												</Select>
											</Field>
											<Field label="Value">
												<Input bind:value={officialQueryValue} placeholder={officialQueryOperator === 'phrase' || officialQueryOperator === 'proximity' ? 'exact phrase' : 'value'} />
											</Field>
											<Field label="Second value">
												<Input bind:value={officialQuerySecondValue} placeholder={officialQueryOperator === 'proximity' ? 'distance' : 'range end'} disabled={!['proximity', 'range-inclusive', 'range-exclusive'].includes(officialQueryOperator)} />
											</Field>
											<div class="flex items-end">
												<Button type="button" variant="outline" className="h-10 w-full" on:click={appendOfficialQueryOption}>Add</Button>
											</div>
										</div>
									</div>
									<div class="grid gap-3 md:grid-cols-4">
										<Field label="Rows">
											<Input type="number" min="1" max="100" bind:value={officialRows} />
										</Field>
										<Field label="Offset">
											<Input type="number" min="0" bind:value={officialOffset} />
										</Field>
										<Field label="Language">
											<Input type="number" min="0" bind:value={officialLanguage} />
										</Field>
										<Field label="Sort order">
											<Input bind:value={officialSortOrder} placeholder="Optional" />
										</Field>
									</div>
									<label class="flex items-center gap-2 text-sm text-zinc-700">
										<Checkbox bind:checked={officialIncludeFilterCounts} />
										Include upstream filter counts
									</label>

								<div class="rounded-md border border-zinc-200 bg-zinc-50 p-3">
									<div class="flex flex-wrap items-center justify-between gap-2">
										<div>
											<div class="text-xs font-semibold uppercase tracking-wide text-zinc-500">Credentials</div>
											<div class="mt-1 text-xs text-zinc-600">{officialSavedLabel()}</div>
										</div>
										<div class="flex items-center gap-2">
											<Button type="button" variant="outline" size="sm" on:click={loadOfficialCredentialStatus} disabled={officialCredentialLoading}>Refresh</Button>
											<Button type="button" variant="outline" size="sm" on:click={deleteOfficialCredentials} disabled={officialCredentialLoading || !officialCredentialStatus?.saved}>Delete saved</Button>
										</div>
										</div>
										<label class="mt-3 flex items-center gap-2 text-sm text-zinc-700">
											<Checkbox bind:checked={officialUseSavedCredentials} disabled={!officialCredentialStatus?.usable} />
											Use saved credentials
										</label>
										{#if !officialUseSavedCredentials}
											<div class="mt-3 grid gap-3 md:grid-cols-2">
												<Field label="Username">
													<Input bind:value={officialUsername} autocomplete="username" />
												</Field>
												<Field label="Password">
													<Input type="password" bind:value={officialPassword} autocomplete="current-password" />
												</Field>
											</div>
											<label class="mt-3 flex items-center gap-2 text-sm text-zinc-700">
												<Checkbox bind:checked={officialRemember} />
												Remember locally with encrypted file KV
											</label>
									{/if}
								</div>

								<div class="flex flex-wrap items-center gap-3">
									<Button type="submit" disabled={officialLoading}>
										<Search size={16} />
										{officialLoading ? 'Searching official API...' : 'Search official API'}
									</Button>
									<span class="text-xs text-zinc-500">Credentials are sent in this local POST body, never as URL query parameters.</span>
								</div>
							</form>
						</section>

							<section class="rounded-lg border border-zinc-200 bg-white p-4" data-testid="official_api_syntax_band">
								<div class="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
									<h3 class="shrink-0 text-sm font-semibold">Query syntax</h3>
									<div class="grid flex-1 gap-3 text-xs leading-5 text-zinc-600 md:grid-cols-3">
										<div>
										<div class="font-semibold uppercase tracking-wide text-zinc-500">Basic</div>
										<div class="mt-1 font-mono text-zinc-800">glucose blood</div>
										<div class="font-mono text-zinc-800">"glucose blood" OR Component:glucose</div>
									</div>
									<div>
									<div class="font-semibold uppercase tracking-wide text-zinc-500">Advanced</div>
									<div class="mt-1 font-mono text-zinc-800">+Component:glucose -System:urine</div>
									<div class="font-mono text-zinc-800">(Component:glucose OR Component:fructose) System:blood</div>
								</div>
								<div>
									<div class="font-semibold uppercase tracking-wide text-zinc-500">Parts and answer lists</div>
										<div class="mt-1 font-mono text-zinc-800">Part:glucose Abbreviation:glu*</div>
										<div class="font-mono text-zinc-800">AnswerDisplayText:positive AnswerCode:LA*</div>
									</div>
									</div>
								</div>
								<p class="mt-3 text-xs leading-5 text-zinc-500">The options builder above includes the complete official advanced LOINC field catalog, part-search fields, answer-list fields, and operators documented by LOINC.</p>
							</section>

							<section class="rounded-lg border border-zinc-200 bg-white" data-testid="official_api_results_window">
							<div class="flex items-center justify-between gap-3 border-b border-zinc-200 px-4 py-3">
								<div>
									<h3 class="text-sm font-semibold">Official results</h3>
									<p class="mt-1 text-xs text-zinc-500">
										{#if officialLoading}
											Searching...
										{:else if officialResult}
											Upstream status {officialResult.upstreamStatus} for {officialResult.scope}
											{#if officialLocalSummary()}
												<span class="ml-2">{officialLocalSummary()}</span>
											{/if}
										{:else}
											Run an official API query to inspect upstream payloads.
										{/if}
									</p>
								</div>
								<Button type="button" variant="outline" size="sm" disabled={!officialResult} on:click={() => (officialRawOpen = !officialRawOpen)}>
									{officialRawOpen ? 'Hide JSON' : 'Raw JSON'}
								</Button>
							</div>
							{#if error && activeView === 'official'}
								<div class="border-b border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800">{error}</div>
							{/if}
							<div class="min-h-[260px] overflow-auto">
								{#if officialLoading}
									<div class="space-y-3 p-4">
										{#each Array(5) as _}<div class="h-14 animate-pulse rounded-md bg-zinc-100"></div>{/each}
									</div>
								{:else if !officialResult}
									<div class="p-4"><EmptyState title="No official API query yet" body="Choose a scope, enter credentials or use saved credentials, then search." /></div>
								{:else if officialRawOpen}
									<pre class="whitespace-pre-wrap break-words p-4 text-xs leading-5 text-zinc-700">{officialRawJSON()}</pre>
								{:else if officialPayloadRows(officialResult.payload).length === 0}
									<pre class="whitespace-pre-wrap break-words p-4 text-xs leading-5 text-zinc-700">{officialRawJSON()}</pre>
								{:else}
									<div class="overflow-x-auto">
										<table class="w-max min-w-full table-auto border-collapse text-left text-sm">
											<thead class="bg-zinc-50 text-xs uppercase tracking-wide text-zinc-500">
												<tr>
													{#if officialResult.local}
														<th class="min-w-[10rem] px-4 py-3 font-medium leading-4">Offline</th>
													{/if}
													{#each officialPayloadKeys(officialPayloadRows(officialResult.payload)) as key}
														<th class={`${officialColumnClass(key)} px-4 py-3 font-medium leading-4`}>
															<span class="block whitespace-normal break-words">{officialHeaderLabel(key)}</span>
														</th>
													{/each}
												</tr>
											</thead>
											<tbody>
												{#each officialPayloadRows(officialResult.payload) as row}
													{@const rowLOINC = officialRowLOINC(row)}
													{@const localMatch = officialLocalMatch(rowLOINC)}
													<tr class="border-t border-zinc-100">
														{#if officialResult.local}
															<td class="px-4 py-3 align-top text-xs">
																{#if localMatch?.found}
																	<div class="flex flex-col items-start gap-2">
																		<Badge variant="default">Local match</Badge>
																		<Button type="button" variant="outline" size="sm" on:click={() => openTerm(localMatch.loincNum)}>Open local</Button>
																	</div>
																{:else if rowLOINC}
																	<div class="flex flex-col items-start gap-2">
																		<Badge variant="secondary">Not local</Badge>
																		<span class="font-mono text-[11px] text-zinc-500">{rowLOINC}</span>
																	</div>
																{:else}
																	<span class="text-zinc-400">No LOINC code</span>
																{/if}
															</td>
														{/if}
														{#each officialPayloadKeys(officialPayloadRows(officialResult.payload)) as key}
															<td class={`${officialColumnClass(key)} px-4 py-3 align-top text-xs leading-5 text-zinc-700`}>
																<div class="max-w-[28rem] whitespace-normal break-words">{officialValue(row[key])}</div>
															</td>
														{/each}
													</tr>
												{/each}
											</tbody>
										</table>
									</div>
								{/if}
							</div>
						</section>
					</div>
				</div>
			{:else if activeView === 'loader'}
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
					<div class="mt-2 flex flex-wrap items-center gap-2">
						<span class="text-[11px] font-semibold uppercase tracking-wide text-zinc-500">Sort</span>
						<div class="inline-flex rounded-md border border-zinc-200 bg-white p-0.5">
							<button
								type="button"
								class={`rounded px-2.5 py-1.5 text-xs font-medium ${searchSort === 'relevance' ? 'bg-zinc-950 text-white' : 'text-zinc-700 hover:bg-zinc-100'}`}
								aria-pressed={searchSort === 'relevance'}
								on:click={() => setSearchSort('relevance')}
							>
								Relevance
							</button>
							<button
								type="button"
								class={`rounded px-2.5 py-1.5 text-xs font-medium ${searchSort === 'usage' ? 'bg-zinc-950 text-white' : 'text-zinc-700 hover:bg-zinc-100'}`}
								aria-pressed={searchSort === 'usage'}
								on:click={() => setSearchSort('usage')}
							>
								Rank
							</button>
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
												on:mousedown={(event) => startColumnResize('loinc', event)}
											></button>
										</th>
										<th class="relative px-4 py-3 font-medium">
											Name
											<button
												type="button"
												class="absolute right-0 top-0 h-full w-2 cursor-col-resize border-r border-transparent hover:border-zinc-400 focus:border-zinc-500 focus:outline-none"
												aria-label="Resize Name column"
												on:pointerdown={(event) => startColumnResize('name', event)}
												on:mousedown={(event) => startColumnResize('name', event)}
											></button>
										</th>
										<th class="relative px-4 py-3 font-medium">
											Status
											<button
												type="button"
												class="absolute right-0 top-0 h-full w-2 cursor-col-resize border-r border-transparent hover:border-zinc-400 focus:border-zinc-500 focus:outline-none"
												aria-label="Resize Status column"
												on:pointerdown={(event) => startColumnResize('status', event)}
												on:mousedown={(event) => startColumnResize('status', event)}
											></button>
										</th>
										<th class="relative px-4 py-3 font-medium">
											Rank
											<button
												type="button"
												class="absolute right-0 top-0 h-full w-2 cursor-col-resize border-r border-transparent hover:border-zinc-400 focus:border-zinc-500 focus:outline-none"
												aria-label="Resize Rank column"
												on:pointerdown={(event) => startColumnResize('rank', event)}
												on:mousedown={(event) => startColumnResize('rank', event)}
											></button>
										</th>
										<th class="relative px-4 py-3 font-medium">
											Axes
											<button
												type="button"
												class="absolute right-0 top-0 h-full w-2 cursor-col-resize border-r border-transparent hover:border-zinc-400 focus:border-zinc-500 focus:outline-none"
												aria-label="Resize Axes column"
												on:pointerdown={(event) => startColumnResize('axes', event)}
												on:mousedown={(event) => startColumnResize('axes', event)}
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
									<h4 class="text-xs font-semibold uppercase tracking-wide text-zinc-500">Clinical relationship lanes</h4>
									<section class="rounded-md border border-zinc-200 p-3">
										<div class="text-xs font-semibold uppercase tracking-wide text-zinc-500">Clinical role</div>
										<div class="mt-2 flex flex-wrap gap-1.5">
											{#each clinicalRoleBadges(selectedTerm) as badge}
												<span class="rounded bg-zinc-100 px-2 py-1 text-xs font-medium text-zinc-700">{badge}</span>
											{/each}
										</div>
										<div class="mt-3 grid grid-cols-2 gap-2 text-xs text-zinc-600">
											<div><span class="font-medium text-zinc-500">Class</span><br />{selectedTerm.class || '-'}</div>
											<div><span class="font-medium text-zinc-500">System</span><br />{selectedTerm.system || '-'}</div>
											<div><span class="font-medium text-zinc-500">Scale</span><br />{selectedTerm.scale || '-'}</div>
											<div><span class="font-medium text-zinc-500">Method</span><br />{selectedTerm.method || '-'}</div>
										</div>
									</section>
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
												<span class="text-xs font-semibold uppercase tracking-wide text-zinc-500">{section.title}</span>
												<button type="button" class="text-xs font-medium text-zinc-700 hover:underline" on:click={() => browseAccessoryForSelected(section.kind)}>Open browser</button>
											</div>
											<div class="max-h-72 divide-y divide-zinc-100 overflow-auto">
												{#each section.items.slice(0, 50) as item}
													<div class="px-3 py-2 text-sm">
														<div class="flex items-start justify-between gap-2">
															<div class="min-w-0">
																<div class="break-words font-medium text-zinc-950">{accessoryTitle(item)}</div>
																{#if accessoryMeta(item)}<div class="mt-0.5 break-words text-xs text-zinc-500">{accessoryMeta(item)}</div>{/if}
																{#if Number(item.fields?.duplicateCount ?? 0) > 1}
																	<div class="mt-0.5 text-xs text-zinc-500">{item.fields.duplicateCount} source rows</div>
																{/if}
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
						<h2 class="text-sm font-semibold">Clinical relationship view</h2>
						<p class="mt-1 truncate text-xs text-zinc-500">{selectedTerm.loincNum} · {selectedTerm.longCommonName}</p>
					</div>
					<div class="flex flex-wrap items-center justify-end gap-2">
						{#if relationshipViewMode === 'explore'}
							<Button variant="outline" size="sm" disabled={graphVisibleConceptLimit <= 8} on:click={() => (graphVisibleConceptLimit = Math.max(8, graphVisibleConceptLimit - 4))}>Fewer</Button>
							<Button variant="outline" size="sm" disabled={graphVisibleConceptLimit >= graphVisibleLimitMax()} on:click={() => (graphVisibleConceptLimit = Math.min(graphVisibleLimitMax(), graphVisibleConceptLimit + 4))}>More</Button>
						{/if}
						<div class="inline-flex rounded-md border border-zinc-200 bg-white p-0.5">
							<button
								type="button"
								class={`rounded px-2.5 py-1.5 text-xs font-medium ${relationshipViewMode === 'clinical' ? 'bg-zinc-950 text-white' : 'text-zinc-700 hover:bg-zinc-100'}`}
								aria-pressed={relationshipViewMode === 'clinical'}
								on:click={() => (relationshipViewMode = 'clinical')}
							>
								Clinical lanes
							</button>
							<button
								type="button"
								class={`rounded px-2.5 py-1.5 text-xs font-medium ${relationshipViewMode === 'explore' ? 'bg-zinc-950 text-white' : 'text-zinc-700 hover:bg-zinc-100'}`}
								aria-pressed={relationshipViewMode === 'explore'}
								on:click={() => (relationshipViewMode = 'explore')}
							>
								Exploration graph
							</button>
						</div>
						<Button variant="ghost" size="icon" ariaLabel="Close relationship graph" on:click={() => (graphViewerOpen = false)}><X size={16} /></Button>
					</div>
				</div>
				{#if relationshipViewMode === 'clinical'}
					<ClinicalRelationshipLanes
						term={selectedTerm}
						graph={relationshipGraph}
						onOpenTerm={(loincNum) => openTerm(loincNum)}
						onBrowseHierarchy={browseHierarchyFromRelationship}
					/>
				{:else}
					<RelationshipGraph
						term={selectedTerm}
						graph={relationshipGraph}
						concepts={sharedConcepts()}
						maxConcepts={graphVisibleConceptLimit}
						maxDirectRelationships={graphVisibleConceptLimit * 2}
						maxTermsPerConcept={3}
						onOpenTerm={(loincNum) => openTerm(loincNum)}
						onBrowseConcept={browseConcept}
					/>
				{/if}
			</section>
		</div>
	{/if}
	{#if advancedSearchHelpOpen}
		<div class="fixed inset-0 z-[80] flex items-center justify-center bg-zinc-950/45 p-4" data-testid="advanced_search_help_modal">
			<section class="w-full max-w-2xl rounded-lg border border-zinc-200 bg-white shadow-2xl">
				<div class="flex items-center justify-between gap-4 border-b border-zinc-200 px-4 py-3">
					<div>
						<h2 class="text-sm font-semibold">Advanced search help</h2>
						<p class="mt-1 text-xs text-zinc-500">Common query patterns supported by advanced search.</p>
					</div>
					<Button variant="ghost" size="icon" ariaLabel="Close advanced search help" on:click={() => (advancedSearchHelpOpen = false)}><X size={16} /></Button>
				</div>
				<div class="grid gap-4 p-4 text-xs leading-5 text-zinc-700 md:grid-cols-2">
					<div>
						<div class="font-semibold uppercase tracking-wide text-zinc-500">Terms and phrases</div>
						<div class="mt-2 font-mono text-zinc-900">morphine cutoff</div>
						<div class="font-mono text-zinc-900">"virus A"</div>
						<div class="font-mono text-zinc-900">gluco*</div>
						<div class="font-mono text-zinc-900">80619-?</div>
					</div>
					<div>
						<div class="font-semibold uppercase tracking-wide text-zinc-500">Boolean logic</div>
						<div class="mt-2 font-mono text-zinc-900">morphine AND cutoff</div>
						<div class="font-mono text-zinc-900">influenza OR parainfluenza</div>
						<div class="font-mono text-zinc-900">influenza NOT equine</div>
						<div class="font-mono text-zinc-900">+morphine -serum</div>
					</div>
					<div>
						<div class="font-semibold uppercase tracking-wide text-zinc-500">LOINC fields</div>
						<div class="mt-2 font-mono text-zinc-900">Component:morphine</div>
						<div class="font-mono text-zinc-900">System:urine</div>
						<div class="font-mono text-zinc-900">Class:DRUG/TOX</div>
						<div class="font-mono text-zinc-900">LOINC:80619-?</div>
					</div>
					<div>
						<div class="font-semibold uppercase tracking-wide text-zinc-500">Other scopes</div>
						<div class="mt-2 font-mono text-zinc-900">Part:glucose</div>
						<div class="font-mono text-zinc-900">AnswerDisplayText:positive</div>
						<div class="font-mono text-zinc-900">AnswerCode:LA*</div>
						<div class="font-mono text-zinc-900">Name:chemistry</div>
					</div>
				</div>
			</section>
		</div>
	{/if}
	<footer class="fixed inset-x-0 bottom-0 z-50 border-t border-zinc-200 bg-white shadow-[0_-1px_3px_rgba(24,24,27,0.04)]">
		<div class="mx-auto flex max-w-[1500px] flex-col gap-2 px-5 py-2 text-[11px] leading-4 text-zinc-500 lg:flex-row lg:items-center lg:justify-between">
			<div class="flex flex-wrap items-center gap-2">
				{#if versionInfo}<span class="font-mono text-zinc-700">v{versionInfo.version}</span><span class="text-zinc-300">|</span>{/if}
				<span>LOINC is Copyright © Regenstrief Institute, Inc. and the LOINC Committee.</span>
				<span class="inline-flex items-center gap-1 rounded-md border border-zinc-200 px-2 py-1 text-zinc-700">
					<Database size={13} />
					{(footerTotal ?? total).toLocaleString()} matches
				</span>
				<span class="inline-flex items-center gap-1 rounded-md border border-zinc-200 px-2 py-1 text-zinc-700">
					<Server size={13} />
					{cacheStats ? `${cachedEntryCount().toLocaleString()} cached` : 'cache ready'}
				</span>
				<button
					type="button"
					class={`inline-flex items-center gap-1 rounded-md border px-2 py-1 font-medium transition-colors ${activeView === 'loader' ? 'border-zinc-950 bg-zinc-950 text-white' : 'border-zinc-200 text-zinc-700 hover:bg-zinc-50 hover:text-zinc-950'}`}
					on:click={openLoader}
				>
					<Upload size={13} />
					Load release zip
				</button>
			</div>
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
