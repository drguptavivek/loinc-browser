<script lang="ts">
	import { onDestroy, onMount, tick } from 'svelte';
	import type cytoscape from 'cytoscape';
	import type { Core, ElementDefinition } from 'cytoscape';
	import type { RelationshipConcept, Term, TermSummary } from '$lib/api';
	import EmptyState from '$lib/components/EmptyState.svelte';

	type Props = {
		term: Term;
		concepts: RelationshipConcept[];
		maxConcepts?: number;
		maxTermsPerConcept?: number;
		onOpenTerm?: (loincNum: string) => void;
		onBrowseConcept?: (concept: RelationshipConcept) => void;
	};

	let {
		term,
		concepts,
		maxConcepts = 8,
		maxTermsPerConcept = 3,
		onOpenTerm = () => {},
		onBrowseConcept = () => {},
	}: Props = $props();

	let container = $state<HTMLDivElement>();
	let cy: Core | null = null;
	let cytoscapeFactory: typeof cytoscape | null = null;
	let selectedConcept: RelationshipConcept | null = $state(null);
	let zoomPercent = $state(100);

	const displayedConcepts = $derived(
		concepts
			.filter((concept) => concept.relatedTotal > 0 || (concept.relatedTerms?.length ?? 0) > 0)
			.slice(0, maxConcepts),
	);

	function conceptID(concept: RelationshipConcept) {
		return `concept:${concept.kind}:${concept.code || concept.title}`;
	}

	function termID(loincNum: string) {
		return `term:${loincNum}`;
	}

	function conceptLabel(concept: RelationshipConcept) {
		return concept.title || concept.code || concept.kind;
	}

	function termLabel(item: TermSummary) {
		return item.displayName || item.shortName || item.longCommonName || item.loincNum;
	}

	function compactNodeLabel(primary: string, secondary: string) {
		const cleanSecondary = secondary.trim();
		if (!cleanSecondary || cleanSecondary === primary) return primary;
		return `${primary}\n${cleanSecondary}`;
	}

	function relationshipType(kind: string) {
		if (kind === 'part-primary') return 'primary-part';
		if (kind === 'part-supplementary') return 'supplementary-part';
		if (kind.includes('answer')) return 'answer-list';
		if (kind.includes('panel')) return 'panel';
		if (kind.includes('hierarchy')) return 'hierarchy';
		if (kind.includes('group')) return 'group';
		return 'related';
	}

	function conceptTone(kind: string) {
		switch (relationshipType(kind)) {
			case 'primary-part':
				return { background: '#dbeafe', border: '#2563eb', text: '#1e3a8a' };
			case 'supplementary-part':
				return { background: '#dcfce7', border: '#16a34a', text: '#14532d' };
			case 'answer-list':
				return { background: '#fef3c7', border: '#d97706', text: '#78350f' };
			case 'panel':
				return { background: '#ede9fe', border: '#7c3aed', text: '#4c1d95' };
			case 'hierarchy':
				return { background: '#cffafe', border: '#0891b2', text: '#164e63' };
			case 'group':
				return { background: '#ffe4e6', border: '#e11d48', text: '#881337' };
			default:
				return { background: '#f4f4f5', border: '#71717a', text: '#18181b' };
		}
	}

	function conceptToneStyle(kind: string) {
		const tone = conceptTone(kind);
		return `background:${tone.background};border:1px solid ${tone.border};color:${tone.text}`;
	}

	function graphElements(): ElementDefinition[] {
		const elements: ElementDefinition[] = [
			{
				data: {
					id: termID(term.loincNum),
					label: term.loincNum,
					subtitle: 'selected term',
					type: 'selected',
				},
			},
		];
		const seen = new Set([termID(term.loincNum)]);
		for (const concept of displayedConcepts) {
			const cID = conceptID(concept);
			if (!seen.has(cID)) {
				seen.add(cID);
				elements.push({
					data: {
						id: cID,
						label: compactNodeLabel(conceptLabel(concept), concept.subtitle || concept.kind),
						subtitle: concept.code || conceptLabel(concept),
						fullLabel: conceptLabel(concept),
						type: 'concept',
						relationship: relationshipType(concept.kind),
					},
				});
			}
			elements.push({
				data: {
					id: `edge:${termID(term.loincNum)}:${cID}`,
					source: termID(term.loincNum),
					target: cID,
					label: concept.relatedTotal ? String(concept.relatedTotal) : '',
					type: 'concept-edge',
					relationship: relationshipType(concept.kind),
				},
			});
			for (const related of (concept.relatedTerms ?? []).slice(0, maxTermsPerConcept)) {
				const rID = termID(related.loincNum);
				if (!seen.has(rID)) {
					seen.add(rID);
					elements.push({
						data: {
							id: rID,
							label: compactNodeLabel(related.loincNum, termLabel(related)),
							subtitle: termLabel(related),
							type: 'related',
							loincNum: related.loincNum,
							relationship: relationshipType(concept.kind),
						},
					});
				}
				elements.push({
					data: {
						id: `edge:${cID}:${rID}`,
						source: cID,
						target: rID,
						type: 'related-edge',
						relationship: relationshipType(concept.kind),
					},
				});
			}
		}
		return elements;
	}

	async function renderGraph() {
		if (!container) return;
		cytoscapeFactory ??= (await import('cytoscape')).default;
		cy?.destroy();
		cy = cytoscapeFactory({
			container,
			elements: graphElements(),
			layout: {
				name: 'breadthfirst',
				animate: false,
				directed: true,
				fit: true,
				padding: 48,
				roots: [termID(term.loincNum)],
				spacingFactor: 1.25,
			},
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
						'text-max-width': '116px',
						width: '118px',
						height: '78px',
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
						width: '104px',
						height: '104px',
						shape: 'ellipse',
					},
				},
				{
					selector: 'node[type = "related"]',
					style: {
						'background-color': '#ffffff',
						'border-color': '#d4d4d8',
						'font-family': 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
						'font-size': 9,
						'text-max-width': '96px',
						width: '102px',
						height: '58px',
						shape: 'round-rectangle',
					},
				},
				{ selector: 'node[relationship = "primary-part"]', style: { 'background-color': '#dbeafe', 'border-color': '#2563eb', color: '#1e3a8a' } },
				{ selector: 'node[relationship = "supplementary-part"]', style: { 'background-color': '#dcfce7', 'border-color': '#16a34a', color: '#14532d' } },
				{ selector: 'node[relationship = "answer-list"]', style: { 'background-color': '#fef3c7', 'border-color': '#d97706', color: '#78350f' } },
				{ selector: 'node[relationship = "panel"]', style: { 'background-color': '#ede9fe', 'border-color': '#7c3aed', color: '#4c1d95' } },
				{ selector: 'node[relationship = "hierarchy"]', style: { 'background-color': '#cffafe', 'border-color': '#0891b2', color: '#164e63' } },
				{ selector: 'node[relationship = "group"]', style: { 'background-color': '#ffe4e6', 'border-color': '#e11d48', color: '#881337' } },
				{ selector: 'node[type = "selected"]', style: { 'background-color': '#18181b', 'border-color': '#18181b', color: '#ffffff' } },
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
				{ selector: 'edge[relationship = "primary-part"]', style: { 'line-color': '#93c5fd', 'target-arrow-color': '#2563eb' } },
				{ selector: 'edge[relationship = "supplementary-part"]', style: { 'line-color': '#86efac', 'target-arrow-color': '#16a34a' } },
				{ selector: 'edge[relationship = "answer-list"]', style: { 'line-color': '#fcd34d', 'target-arrow-color': '#d97706' } },
				{ selector: 'edge[relationship = "panel"]', style: { 'line-color': '#c4b5fd', 'target-arrow-color': '#7c3aed' } },
				{ selector: 'edge[relationship = "hierarchy"]', style: { 'line-color': '#67e8f9', 'target-arrow-color': '#0891b2' } },
				{ selector: 'edge[relationship = "group"]', style: { 'line-color': '#fda4af', 'target-arrow-color': '#e11d48' } },
				{
					selector: 'edge[type = "related-edge"]',
					style: {
						'target-arrow-shape': 'none',
						width: 1,
					},
				},
			],
			wheelSensitivity: 0.25,
		});
		cy.on('tap', 'node[type = "concept"]', (event) => {
			const id = event.target.id();
			selectedConcept = displayedConcepts.find((concept) => conceptID(concept) === id) ?? null;
		});
		cy.on('tap', 'node[type = "related"]', (event) => {
			const loincNum = event.target.data('loincNum');
			if (loincNum) onOpenTerm(loincNum);
		});
		cy.on('zoom', () => {
			zoomPercent = Math.round((cy?.zoom() ?? 1) * 100);
		});
		zoomPercent = Math.round(cy.zoom() * 100);
	}

	function zoomGraph(delta: number) {
		if (!cy) return;
		const current = cy.zoom();
		const next = Math.min(2.5, Math.max(0.35, current + delta));
		cy.zoom({
			level: next,
			renderedPosition: { x: cy.width() / 2, y: cy.height() / 2 },
		});
		zoomPercent = Math.round(next * 100);
	}

	function resetView() {
		if (!cy) return;
		cy.fit(undefined, 48);
		zoomPercent = Math.round(cy.zoom() * 100);
	}

	function organizeGraph() {
		if (!cy) return;
		cy.layout({
			name: 'breadthfirst',
			animate: true,
			animationDuration: 250,
			directed: true,
			fit: true,
			padding: 48,
			roots: [termID(term.loincNum)],
			spacingFactor: 1.25,
		}).run();
		zoomPercent = Math.round(cy.zoom() * 100);
	}

	onMount(() => {
		void tick().then(() => renderGraph());
	});

	$effect(() => {
		term.loincNum;
		displayedConcepts;
		void tick().then(() => renderGraph());
	});

	onDestroy(() => {
		cy?.destroy();
	});
</script>

<div class="grid min-h-0 flex-1 gap-0 overflow-hidden lg:grid-cols-[minmax(0,2fr)_360px]">
	<div class="min-h-[620px] bg-zinc-50 p-4">
		{#if displayedConcepts.length === 0}
			<div class="flex h-full min-h-[620px] items-center justify-center rounded-md border border-zinc-200 bg-white">
				<EmptyState title="No graph links" body="This term has no shared concept neighborhoods to draw." />
			</div>
		{:else}
			<div class="relative h-full min-h-[620px] rounded-md border border-zinc-200 bg-white">
				<div class="absolute right-3 top-3 z-10 flex items-center gap-2 rounded-md border border-zinc-200 bg-white/95 p-1 shadow-sm">
					<button type="button" class="rounded px-2.5 py-1.5 text-xs font-medium text-zinc-700 hover:bg-zinc-100" onclick={() => zoomGraph(-0.15)}>Zoom out</button>
					<span class="w-12 text-center text-xs tabular-nums text-zinc-500">{zoomPercent}%</span>
					<button type="button" class="rounded px-2.5 py-1.5 text-xs font-medium text-zinc-700 hover:bg-zinc-100" onclick={() => zoomGraph(0.15)}>Zoom in</button>
					<button type="button" class="rounded px-2.5 py-1.5 text-xs font-medium text-zinc-700 hover:bg-zinc-100" onclick={resetView}>Reset view</button>
					<button type="button" class="rounded px-2.5 py-1.5 text-xs font-medium text-zinc-700 hover:bg-zinc-100" onclick={organizeGraph}>Organize</button>
				</div>
				<div bind:this={container} class="h-full min-h-[620px]"></div>
			</div>
		{/if}
	</div>
	<aside class="min-h-0 overflow-auto border-t border-zinc-200 p-4 lg:border-l lg:border-t-0">
		<div class="mb-3 flex items-center justify-between gap-2">
			<h3 class="text-xs font-semibold uppercase tracking-wide text-zinc-500">Visible concepts</h3>
			<span class="text-xs text-zinc-500">{displayedConcepts.length} of {concepts.length}</span>
		</div>
		<div class="flex flex-col gap-3">
			{#if displayedConcepts.length === 0}
				<EmptyState title="No graph links" body="This term has no shared concept neighborhoods to draw." />
			{:else}
				{#each displayedConcepts as concept}
					<section class={`rounded-md border p-3 ${selectedConcept && conceptID(selectedConcept) === conceptID(concept) ? 'border-zinc-900 bg-zinc-50' : 'border-zinc-200'}`}>
						<div class="break-words text-sm font-medium text-zinc-950">{conceptLabel(concept)}</div>
						<div class="mt-1 flex flex-wrap gap-2 text-xs text-zinc-500">
							<span class="rounded px-1.5 py-0.5 font-medium" style={conceptToneStyle(concept.kind)}>{concept.kind}</span>
							{#if concept.code}<span class="font-mono">{concept.code}</span>{/if}
							<span>{concept.relatedTotal.toLocaleString()} other terms</span>
						</div>
						<div class="mt-2 flex flex-wrap gap-3">
							<button type="button" class="text-xs font-medium text-zinc-700 hover:underline" onclick={() => (selectedConcept = concept)}>Focus node</button>
							<button type="button" class="text-xs font-medium text-zinc-700 hover:underline" onclick={() => onBrowseConcept(concept)}>Browse concept</button>
						</div>
						{#if concept.relatedTerms?.length}
							<div class="mt-2 flex flex-col gap-1.5">
								{#each concept.relatedTerms.slice(0, maxTermsPerConcept) as related}
									<button type="button" class="rounded-md bg-zinc-50 px-2 py-1.5 text-left text-xs hover:bg-zinc-100" onclick={() => onOpenTerm(related.loincNum)}>
										<span class="font-mono font-semibold text-zinc-900">{related.loincNum}</span>
										<span class="mt-0.5 block break-words text-zinc-600">{termLabel(related)}</span>
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
