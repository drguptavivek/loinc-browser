<script lang="ts">
	import type { Term, TermAccessory, TermRelationshipGraph } from '$lib/api';
	import EmptyState from '$lib/components/EmptyState.svelte';

	type Props = {
		term: Term;
		graph?: TermRelationshipGraph | null;
		onOpenTerm?: (loincNum: string) => void;
		onBrowseHierarchy?: (nodeId: string, label: string) => void;
	};

	let { term, graph = null, onOpenTerm = () => {}, onBrowseHierarchy = () => {} }: Props = $props();

	const parentContainers = $derived(sortParents(graph?.panelMemberships ?? []));
	const childItems = $derived(dedupePanelItems(graph?.panelItems ?? []));
	const hierarchyRows = $derived((graph?.hierarchy ?? term.hierarchy ?? []).slice(0, 6));
	const siblingContext = $derived(parentContainers.slice(0, 3));
	const hasAnyLane = $derived(parentContainers.length > 0 || childItems.length > 0 || hierarchyRows.length > 0 || (graph?.sharedConcepts?.length ?? 0) > 0);

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
		if ((graph?.answerLists?.length ?? term.answerLists?.length ?? 0) > 0) badges.push('Answer list');
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
</script>

<div class="grid min-h-0 flex-1 gap-0 overflow-hidden lg:grid-cols-[minmax(0,2fr)_390px]">
	<div class="min-h-[620px] bg-zinc-50 p-4">
		{#if !hasAnyLane}
			<div class="flex h-full min-h-[620px] items-center justify-center rounded-md border border-zinc-200 bg-white">
				<EmptyState title="No clinical lanes" body="No parent containers, child items, hierarchy placements, or shared concepts are available for this term." />
			</div>
		{:else}
			<div class="flex h-full min-h-[620px] flex-col justify-between gap-4 rounded-md border border-zinc-200 bg-white p-5">
				<section>
					<div class="mb-2 flex items-center justify-between gap-3">
						<h3 class="text-xs font-semibold uppercase tracking-wide text-zinc-500">Contained in</h3>
						<span class="text-xs text-zinc-500">{parentContainers.length}</span>
					</div>
					<div class="grid gap-2 md:grid-cols-2 xl:grid-cols-3">
						{#each parentContainers.slice(0, 6) as item}
							<button type="button" class="min-h-20 rounded-md border border-violet-200 bg-violet-50 px-3 py-2 text-left hover:border-violet-400" onclick={() => openAccessory(item)}>
								<div class="font-mono text-xs font-semibold text-violet-950">{item.code}</div>
								<div class="mt-1 line-clamp-2 text-sm font-medium text-zinc-950">{title(item)}</div>
								<div class="mt-1 text-xs text-violet-700">{[sequenceLabel(item), rankLabel(item)].filter(Boolean).join(' · ')}</div>
							</button>
						{/each}
					</div>
				</section>

				<div class="flex items-center justify-center">
					<div class="flex size-36 flex-col items-center justify-center rounded-full bg-zinc-950 p-4 text-center text-white shadow-sm">
						<div class="font-mono text-xl font-bold">{term.loincNum}</div>
						<div class="mt-1 text-xs text-zinc-300">selected term</div>
					</div>
				</div>

				<section>
					<div class="mb-2 flex items-center justify-between gap-3">
						<h3 class="text-xs font-semibold uppercase tracking-wide text-zinc-500">{childLaneTitle()}</h3>
						<span class="text-xs text-zinc-500">{childItems.length}</span>
					</div>
					<div class="grid gap-2 md:grid-cols-2 xl:grid-cols-3">
						{#each childItems.slice(0, 9) as item}
							<button type="button" class="min-h-20 rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-left hover:border-emerald-400" onclick={() => openAccessory(item)}>
								<div class="flex items-center justify-between gap-2">
									<span class="text-xs font-medium text-emerald-700">{sequenceLabel(item) || 'Item'}</span>
									<span class="font-mono text-xs font-semibold text-emerald-950">{item.code}</span>
								</div>
								<div class="mt-1 line-clamp-2 text-sm font-medium text-zinc-950">{title(item)}</div>
								{#if duplicateLabel(item)}<div class="mt-1 text-xs text-emerald-700">{duplicateLabel(item)}</div>{/if}
							</button>
						{/each}
					</div>
				</section>

				<section class="rounded-md border border-cyan-200 bg-cyan-50 p-3">
					<div class="mb-2 flex items-center justify-between gap-3">
						<h3 class="text-xs font-semibold uppercase tracking-wide text-cyan-800">Hierarchy path</h3>
						<span class="text-xs text-cyan-700">{hierarchyRows.length}</span>
					</div>
					<div class="flex flex-wrap gap-2">
						{#each hierarchyRows.slice(0, 4) as item}
							<button type="button" class="rounded border border-cyan-200 bg-white px-2 py-1 text-xs text-cyan-950 hover:border-cyan-500" onclick={() => browseHierarchy(item)}>{title(item)}</button>
						{/each}
					</div>
				</section>
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
