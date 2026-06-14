<script lang="ts">
	import { onDestroy, onMount, tick } from 'svelte';
	import type cytoscape from 'cytoscape';
	import type { Core, ElementDefinition } from 'cytoscape';
	import type { Term, TermAccessory, TermRelationshipGraph } from '$lib/api';
	import EmptyState from '$lib/components/EmptyState.svelte';

	type Props = {
		term: Term;
		graph?: TermRelationshipGraph | null;
		onOpenTerm?: (loincNum: string) => void;
		onBrowseHierarchy?: (nodeId: string, label: string) => void;
	};

	let { term, graph = null, onOpenTerm = () => {}, onBrowseHierarchy = () => {} }: Props = $props();

	const parentContainers = $derived(dedupeParents(graph?.panelMemberships ?? []));
	const childItems = $derived(dedupePanelItems(graph?.panelItems ?? []));
	const answerLists = $derived(graph?.answerLists ?? term.answerLists ?? []);
	const hierarchyRows = $derived((graph?.hierarchy ?? term.hierarchy ?? []).slice(0, 6));
	const siblingContext = $derived(parentContainers.slice(0, 3));
	const hasAnyLane = $derived(parentContainers.length > 0 || childItems.length > 0 || answerLists.length > 0 || hierarchyRows.length > 0 || (graph?.sharedConcepts?.length ?? 0) > 0);
	const mapParents = $derived(parentContainers.slice(0, 8));
	const mapChildren = $derived(childItems.slice(0, 18));
	const mapAnswerLists = $derived(answerLists.slice(0, 4));
	const mapHierarchy = $derived(hierarchyRows.slice(0, 4));

	let container = $state<HTMLDivElement>();
	let cy: Core | null = null;
	let cytoscapeFactory: typeof cytoscape | null = null;
	let zoomPercent = $state(100);

	function sortParents(items: TermAccessory[]) {
		return [...items].sort((left, right) => {
			const rank = parentRank(left) - parentRank(right);
			if (rank !== 0) return rank;
			const type = parentTypePriority(left) - parentTypePriority(right);
			if (type !== 0) return type;
			const sequence = numericField(left, 'sequence') - numericField(right, 'sequence');
			if (sequence !== 0) return sequence;
			return title(left).localeCompare(title(right)) || left.code.localeCompare(right.code);
		});
	}

	function dedupeParents(items: TermAccessory[]) {
		const seen = new Map<string, TermAccessory>();
		for (const item of sortParents(items)) {
			const key = item.code || title(item);
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

	function dedupePanelItems(items: TermAccessory[]) {
		const seen = new Map<string, TermAccessory>();
		for (const item of [...items].sort(compareChildItems)) {
			const key = item.code || title(item);
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

	function compareChildItems(left: TermAccessory, right: TermAccessory) {
		const sequence = numericField(left, 'sequence') - numericField(right, 'sequence');
		if (sequence !== 0) return sequence;
		return title(left).localeCompare(title(right)) || left.code.localeCompare(right.code);
	}

	function title(item: TermAccessory) {
		return item.title || item.code || item.kind;
	}

	function parentRank(item: TermAccessory) {
		const rank = numericField(item, 'parentRank');
		return rank > 0 ? rank : Number.POSITIVE_INFINITY;
	}

	function numericField(item: TermAccessory, field: string) {
		const value = Number(item.fields?.[field] ?? 0);
		return Number.isFinite(value) ? value : 0;
	}

	function parentTypePriority(item: TermAccessory) {
		const text = `${item.title} ${item.subtitle} ${item.fields?.entryType ?? ''}`.toLowerCase();
		if (text.includes('questionnaire') || text.includes('survey') || text.includes('scale') || text.includes('phq')) return 1;
		if (text.includes('panel') || text.includes('order')) return 2;
		return 3;
	}

	function sequenceLabel(item: TermAccessory) {
		return item.fields?.sequence ? `Seq ${item.fields.sequence}` : '';
	}

	function duplicateLabel(item: TermAccessory) {
		const count = Number(item.fields?.duplicateCount ?? 0);
		return Number.isFinite(count) && count > 1 ? `${count} source rows` : '';
	}

	function rankLabel(item: TermAccessory) {
		const rank = parentRank(item);
		return Number.isFinite(rank) ? `Rank ${rank}` : '';
	}

	function isLoincNumber(value: string) {
		return /^\d+-\d$/.test(value);
	}

	function clinicalRoleBadges() {
		const badges: string[] = [];
		if (term.orderObs) badges.push(term.orderObs);
		if (parentContainers.length) badges.push('Contained item');
		if (childItems.length) badges.push(isSurveyTerm() ? 'Questionnaire / scale' : 'Panel / order');
		if (answerLists.length > 0) badges.push('Answer list');
		if (isSurveyTerm()) badges.push('Survey');
		return [...new Set(badges.filter(Boolean))];
	}

	function isSurveyTerm() {
		const text = `${term.class} ${term.method} ${term.longCommonName}`.toLowerCase();
		return text.includes('survey') || text.includes('questionnaire') || text.includes('phq');
	}

	function childLaneTitle() {
		return isSurveyTerm() ? 'Scale / survey items' : 'Panel observations';
	}

	function openAccessory(item: TermAccessory) {
		if (isLoincNumber(item.code)) onOpenTerm(item.code);
	}

	function browseHierarchy(item: TermAccessory) {
		const nodeId = item.fields?.nodeId;
		if (nodeId) onBrowseHierarchy(nodeId, item.title || item.code);
	}

	function nodeLabel(primary: string, secondary: string) {
		if (!secondary || secondary === primary) return primary;
		return `${primary}\n${secondary}`;
	}

	function accessoryNodeID(prefix: string, item: TermAccessory, index: number) {
		return `${prefix}:${item.kind}:${item.code || title(item)}:${index}`;
	}

	function graphElements(): ElementDefinition[] {
		const elements: ElementDefinition[] = [];
		const selectedID = `term:${term.loincNum}`;
		const parentWidth = Math.max(1, mapParents.length - 1);
		const childWidth = Math.max(1, mapChildren.length - 1);
		const parentStart = -Math.min(560, parentWidth * 74);
		const childStart = -Math.min(760, childWidth * 48);

		for (const [index, item] of mapParents.entries()) {
			const id = accessoryNodeID('parent', item, index);
			elements.push({
				data: {
					id,
					label: nodeLabel(item.code, title(item)),
					type: 'parent',
					loincNum: isLoincNumber(item.code) ? item.code : '',
				},
				position: { x: parentStart + index * 148, y: 80 },
			});
			elements.push({
				data: {
					id: `edge:${id}:${selectedID}`,
					source: id,
					target: selectedID,
					label: sequenceLabel(item),
					type: 'parent-edge',
				},
			});
		}

		elements.push({
			data: {
				id: selectedID,
				label: nodeLabel(term.loincNum, 'selected term'),
				type: 'selected',
				loincNum: term.loincNum,
			},
			position: { x: 0, y: 300 },
		});

		for (const [index, item] of mapChildren.entries()) {
			const id = accessoryNodeID('child', item, index);
			elements.push({
				data: {
					id,
					label: nodeLabel(item.code, title(item)),
					type: 'child',
					loincNum: isLoincNumber(item.code) ? item.code : '',
				},
				position: { x: childStart + index * 96, y: 520 },
			});
			elements.push({
				data: {
					id: `edge:${selectedID}:${id}`,
					source: selectedID,
					target: id,
					label: sequenceLabel(item),
					type: 'child-edge',
				},
			});
		}

		for (const [index, item] of mapHierarchy.entries()) {
			const id = accessoryNodeID('hierarchy', item, index);
			elements.push({
				data: {
					id,
					label: title(item),
					type: 'hierarchy',
					nodeId: item.fields?.nodeId ?? '',
				},
				position: { x: 600, y: 150 + index * 95 },
			});
			elements.push({
				data: {
					id: `edge:${id}:${selectedID}`,
					source: id,
					target: selectedID,
					type: 'hierarchy-edge',
				},
			});
		}
		for (const [index, item] of mapAnswerLists.entries()) {
			const id = accessoryNodeID('answer', item, index);
			elements.push({
				data: {
					id,
					label: nodeLabel(item.code, title(item)),
					type: 'answer-list',
				},
				position: { x: -600, y: 150 + index * 95 },
			});
			elements.push({
				data: {
					id: `edge:${selectedID}:${id}`,
					source: selectedID,
					target: id,
					type: 'answer-edge',
				},
			});
		}
		return elements;
	}

	async function renderMap() {
		if (!container || !hasAnyLane) return;
		cytoscapeFactory ??= (await import('cytoscape')).default;
		cy?.destroy();
		cy = cytoscapeFactory({
			container,
			elements: graphElements(),
			layout: { name: 'preset', fit: true, padding: 52 },
			style: [
				{
					selector: 'node',
					style: {
						'background-color': '#f4f4f5',
						'border-color': '#a1a1aa',
						'border-width': 1,
						color: '#18181b',
						'font-size': 10,
						label: 'data(label)',
						'text-halign': 'center',
						'text-valign': 'center',
						'text-wrap': 'wrap',
						'text-max-width': '112px',
						width: '118px',
						height: '70px',
						shape: 'round-rectangle',
					},
				},
				{
					selector: 'node[type = "selected"]',
					style: {
						'background-color': '#18181b',
						'border-color': '#18181b',
						color: '#ffffff',
						'font-size': 14,
						'font-weight': 700,
						width: '118px',
						height: '118px',
						shape: 'ellipse',
					},
				},
				{ selector: 'node[type = "parent"]', style: { 'background-color': '#ede9fe', 'border-color': '#7c3aed', color: '#4c1d95' } },
				{ selector: 'node[type = "child"]', style: { 'background-color': '#dcfce7', 'border-color': '#16a34a', color: '#14532d', width: '108px', height: '64px', 'font-size': 9 } },
				{ selector: 'node[type = "answer-list"]', style: { 'background-color': '#fef3c7', 'border-color': '#d97706', color: '#78350f', width: '128px', height: '64px', 'font-size': 9 } },
				{ selector: 'node[type = "hierarchy"]', style: { 'background-color': '#cffafe', 'border-color': '#0891b2', color: '#164e63', width: '128px', height: '60px', 'font-size': 9 } },
				{
					selector: 'edge',
					style: {
						'curve-style': 'bezier',
						'line-color': '#d4d4d8',
						'target-arrow-color': '#a1a1aa',
						'target-arrow-shape': 'triangle',
						width: 1.5,
					},
				},
				{ selector: 'edge[type = "parent-edge"]', style: { 'line-color': '#c4b5fd', 'target-arrow-color': '#7c3aed' } },
				{ selector: 'edge[type = "child-edge"]', style: { 'line-color': '#86efac', 'target-arrow-color': '#16a34a' } },
				{ selector: 'edge[type = "answer-edge"]', style: { 'line-color': '#fcd34d', 'target-arrow-color': '#d97706' } },
				{ selector: 'edge[type = "hierarchy-edge"]', style: { 'line-color': '#67e8f9', 'target-arrow-color': '#0891b2', 'line-style': 'dashed' } },
			],
			wheelSensitivity: 0.25,
		});
		cy.on('tap', 'node', (event) => {
			const loincNum = event.target.data('loincNum');
			if (loincNum) onOpenTerm(loincNum);
			const nodeId = event.target.data('nodeId');
			if (nodeId) onBrowseHierarchy(nodeId, event.target.data('label') || '');
		});
		cy.on('zoom', () => {
			zoomPercent = Math.round((cy?.zoom() ?? 1) * 100);
		});
		zoomPercent = Math.round(cy.zoom() * 100);
	}

	function zoomMap(delta: number) {
		if (!cy) return;
		const next = Math.min(2.5, Math.max(0.35, cy.zoom() + delta));
		cy.zoom({ level: next, renderedPosition: { x: cy.width() / 2, y: cy.height() / 2 } });
		zoomPercent = Math.round(next * 100);
	}

	function resetMap() {
		if (!cy) return;
		cy.fit(undefined, 52);
		zoomPercent = Math.round(cy.zoom() * 100);
	}

	onMount(() => {
		void tick().then(() => renderMap());
	});

	$effect(() => {
		term.loincNum;
		mapParents;
		mapChildren;
		mapAnswerLists;
		mapHierarchy;
		void tick().then(() => renderMap());
	});

	onDestroy(() => {
		cy?.destroy();
	});
</script>

<div class="grid min-h-0 flex-1 gap-0 overflow-hidden lg:grid-cols-[minmax(0,2fr)_390px]">
	<div class="min-h-[620px] bg-zinc-50 p-4">
		{#if !hasAnyLane}
			<div class="flex h-full min-h-[620px] items-center justify-center rounded-md border border-zinc-200 bg-white">
				<EmptyState title="No clinical lanes" body="No parent containers, child items, hierarchy placements, or shared concepts are available for this term." />
			</div>
		{:else}
			<div class="relative h-full min-h-[620px] overflow-hidden rounded-md border border-zinc-200 bg-white">
				<div class="absolute left-3 top-3 z-10 flex flex-wrap items-center gap-2 rounded-md border border-zinc-200 bg-white/95 px-2 py-1.5 text-xs text-zinc-600 shadow-sm">
					<span class="font-semibold uppercase tracking-wide text-zinc-500">Clinical lanes</span>
					<span class="rounded bg-violet-50 px-1.5 py-0.5 text-violet-800">Contained in {parentContainers.length}</span>
					<span class="rounded bg-emerald-50 px-1.5 py-0.5 text-emerald-800">{childLaneTitle()} {childItems.length}</span>
					<span class="rounded bg-amber-50 px-1.5 py-0.5 text-amber-800">Answer lists {answerLists.length}</span>
					<span class="rounded bg-cyan-50 px-1.5 py-0.5 text-cyan-800">Hierarchy {hierarchyRows.length}</span>
				</div>
				<div class="absolute bottom-3 right-3 z-10 flex items-center gap-2 rounded-md border border-zinc-200 bg-white/95 p-1 shadow-sm">
					<button type="button" class="rounded px-2.5 py-1.5 text-xs font-medium text-zinc-700 hover:bg-zinc-100" onclick={() => zoomMap(-0.15)}>Zoom out</button>
					<span class="w-12 text-center text-xs tabular-nums text-zinc-500">{zoomPercent}%</span>
					<button type="button" class="rounded px-2.5 py-1.5 text-xs font-medium text-zinc-700 hover:bg-zinc-100" onclick={() => zoomMap(0.15)}>Zoom in</button>
					<button type="button" class="rounded px-2.5 py-1.5 text-xs font-medium text-zinc-700 hover:bg-zinc-100" onclick={resetMap}>Reset view</button>
				</div>
				<div bind:this={container} class="h-full min-h-[620px]"></div>
				<div class="pointer-events-none absolute bottom-3 left-3 max-w-[260px] rounded-md border border-zinc-200 bg-white/95 px-2 py-1.5 text-xs text-zinc-500 shadow-sm">
					Pan/zoom map. Click LOINC nodes to open.
				</div>
			</div>
		{/if}
	</div>

	<aside class="min-h-0 overflow-auto border-t border-zinc-200 p-4 lg:border-l lg:border-t-0">
		<div class="flex flex-col gap-3">
			<section class="rounded-md border border-zinc-200 bg-white p-3">
				<h3 class="text-xs font-semibold uppercase tracking-wide text-zinc-500">Clinical role</h3>
				<div class="mt-2 flex flex-wrap gap-1.5">
					{#each clinicalRoleBadges() as badge}
						<span class="rounded bg-zinc-100 px-2 py-1 text-xs font-medium text-zinc-700">{badge}</span>
					{/each}
				</div>
				<div class="mt-3 grid grid-cols-2 gap-2 text-xs text-zinc-600">
					<div><span class="font-medium text-zinc-500">Class</span><br />{term.class || '-'}</div>
					<div><span class="font-medium text-zinc-500">System</span><br />{term.system || '-'}</div>
					<div><span class="font-medium text-zinc-500">Scale</span><br />{term.scale || '-'}</div>
					<div><span class="font-medium text-zinc-500">Method</span><br />{term.method || '-'}</div>
				</div>
			</section>

			<section class="rounded-md border border-zinc-200 bg-white p-3">
				<div class="mb-2 flex items-center justify-between gap-2">
					<h3 class="text-xs font-semibold uppercase tracking-wide text-zinc-500">Contained in</h3>
					<span class="text-xs text-zinc-500">{parentContainers.length}</span>
				</div>
				{#if parentContainers.length}
					<div class="flex flex-col gap-2">
						{#each parentContainers.slice(0, 12) as item}
							<button type="button" class="rounded-md bg-zinc-50 px-2 py-1.5 text-left hover:bg-zinc-100" onclick={() => openAccessory(item)}>
								<div class="break-words text-sm font-medium text-zinc-950">{title(item)}</div>
								<div class="mt-1 flex flex-wrap gap-2 text-xs text-zinc-500">
									<span class="font-mono">{item.code}</span>
									{#if sequenceLabel(item)}<span>{sequenceLabel(item)}</span>{/if}
									{#if rankLabel(item)}<span>{rankLabel(item)}</span>{/if}
									{#if duplicateLabel(item)}<span>{duplicateLabel(item)}</span>{/if}
								</div>
							</button>
						{/each}
					</div>
				{:else}
					<EmptyState title="No parent containers" body="This term is not listed as a child item in a panel, order, questionnaire, scale, or form." />
				{/if}
			</section>

			<section class="rounded-md border border-zinc-200 bg-white p-3">
				<div class="mb-2 flex items-center justify-between gap-2">
					<h3 class="text-xs font-semibold uppercase tracking-wide text-zinc-500">{childLaneTitle()}</h3>
					<span class="text-xs text-zinc-500">{childItems.length}</span>
				</div>
				{#if childItems.length}
					<div class="flex flex-col gap-2">
						{#each childItems.slice(0, 20) as item}
							<button type="button" class="rounded-md bg-zinc-50 px-2 py-1.5 text-left hover:bg-zinc-100" onclick={() => openAccessory(item)}>
								<div class="break-words text-sm font-medium text-zinc-950">{title(item)}</div>
								<div class="mt-1 flex flex-wrap gap-2 text-xs text-zinc-500">
									{#if sequenceLabel(item)}<span>{sequenceLabel(item)}</span>{/if}
									<span class="font-mono">{item.code}</span>
									{#if duplicateLabel(item)}<span>{duplicateLabel(item)}</span>{/if}
								</div>
							</button>
						{/each}
					</div>
				{:else}
					<EmptyState title="No child items" body="This term is not a panel, order, questionnaire, scale, or form with listed child items." />
				{/if}
			</section>

			<section class="rounded-md border border-zinc-200 bg-white p-3">
				<div class="mb-2 flex items-center justify-between gap-2">
					<h3 class="text-xs font-semibold uppercase tracking-wide text-zinc-500">Answer lists</h3>
					<span class="text-xs text-zinc-500">{answerLists.length}</span>
				</div>
				{#if answerLists.length}
					<div class="flex flex-col gap-2">
						{#each answerLists as item}
							<div class="rounded-md bg-amber-50 px-2 py-1.5">
								<div class="break-words text-sm font-medium text-zinc-950">{title(item)}</div>
								<div class="mt-1 flex flex-wrap gap-2 text-xs text-zinc-500">
									<span class="font-mono">{item.code}</span>
									{#if item.fields?.answerListLinkType}<span>{item.fields.answerListLinkType}</span>{/if}
									{#if item.subtitle}<span>{item.subtitle}</span>{/if}
								</div>
							</div>
						{/each}
					</div>
				{:else}
					<EmptyState title="No answer lists" body="This term has no linked answer list." />
				{/if}
			</section>

			<section class="rounded-md border border-zinc-200 bg-white p-3">
				<div class="mb-2 flex items-center justify-between gap-2">
					<h3 class="text-xs font-semibold uppercase tracking-wide text-zinc-500">Hierarchy path</h3>
					<span class="text-xs text-zinc-500">{hierarchyRows.length}</span>
				</div>
				{#if hierarchyRows.length}
					<div class="flex flex-col gap-2">
						{#each hierarchyRows as item}
							<button type="button" class="rounded-md bg-zinc-50 px-2 py-1.5 text-left text-sm hover:bg-zinc-100" onclick={() => browseHierarchy(item)}>
								<div class="break-words font-medium text-zinc-950">{title(item)}</div>
								<div class="mt-1 text-xs text-zinc-500">{item.subtitle || item.code}</div>
							</button>
						{/each}
					</div>
				{:else}
					<EmptyState title="No hierarchy path" body="No hierarchy placement is available for this term." />
				{/if}
			</section>

			<section class="rounded-md border border-zinc-200 bg-white p-3">
				<div class="mb-2 flex items-center justify-between gap-2">
					<h3 class="text-xs font-semibold uppercase tracking-wide text-zinc-500">Nearby context</h3>
					<span class="text-xs text-zinc-500">{siblingContext.length}</span>
				</div>
				{#if siblingContext.length}
					<p class="text-sm leading-5 text-zinc-600">Use the top parent container to review adjacent panel or questionnaire items in sequence.</p>
					<div class="mt-2 flex flex-col gap-2">
						{#each siblingContext as item}
							<button type="button" class="rounded-md bg-zinc-50 px-2 py-1.5 text-left text-sm hover:bg-zinc-100" onclick={() => openAccessory(item)}>
								<span class="font-mono font-semibold">{item.code}</span>
								<span class="ml-2">{title(item)}</span>
							</button>
						{/each}
					</div>
				{:else}
					<EmptyState title="No nearby context" body="No parent container is available for sibling context." />
				{/if}
			</section>
		</div>
	</aside>
</div>
