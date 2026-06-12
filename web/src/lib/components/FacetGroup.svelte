<script lang="ts">
	import { ChevronDown, ChevronRight, Search } from '@lucide/svelte';

	export let title: string;
	export let entries: [string, number][] = [];
	export let active = '';
	export let kind: 'status' | 'class' | 'system' | 'scale' | 'property' | 'orderObs';
	export let onPick: (
		kind: 'status' | 'class' | 'system' | 'scale' | 'property' | 'orderObs',
		value: string,
	) => void;
	export let pageSize = 10;

	type FacetNode = {
		key: string;
		label: string;
		value: string;
		count: number;
		children: FacetNode[];
	};

	let expanded = true;
	let facetQuery = '';
	let page = 0;

	$: normalizedQuery = facetQuery.trim().toLowerCase();
	$: tree = buildTree(entries);
	$: filteredTree = normalizedQuery ? filterTree(tree, normalizedQuery) : tree;
	$: pageCount = Math.max(1, Math.ceil(filteredTree.length / pageSize));
	$: if (page > pageCount - 1) page = pageCount - 1;
	$: visibleTree = filteredTree.slice(page * pageSize, page * pageSize + pageSize);

	function updateFacetQuery(value: string) {
		facetQuery = value;
		page = 0;
	}

	function splitFacet(value: string) {
		if (value.includes('>')) return value.split('>').map((part) => part.trim()).filter(Boolean);
		if (value.includes('.')) return value.split('.').map((part) => part.trim()).filter(Boolean);
		return [value];
	}

	function buildTree(values: [string, number][]) {
		const roots: FacetNode[] = [];
		const byKey = new Map<string, FacetNode>();

		for (const [value, count] of values) {
			const parts = splitFacet(value);
			let path = '';
			let siblings = roots;

			for (let index = 0; index < parts.length; index += 1) {
				path = path ? `${path}${value.includes('>') ? '>' : '.'}${parts[index]}` : parts[index];
				let node = byKey.get(path);
				if (!node) {
					node = { key: path, label: parts[index], value: path, count: 0, children: [] };
					byKey.set(path, node);
					siblings.push(node);
				}
				if (index === parts.length - 1) {
					node.value = value;
					node.count = count;
				}
				siblings = node.children;
			}
		}

		return roots;
	}

	function filterTree(nodes: FacetNode[], needle: string): FacetNode[] {
		return nodes
			.map((node) => {
				const children = filterTree(node.children, needle);
				if (node.value.toLowerCase().includes(needle) || node.label.toLowerCase().includes(needle) || children.length > 0) {
					return { ...node, children };
				}
				return null;
			})
			.filter((node): node is FacetNode => node !== null);
	}

	function isActivePath(node: FacetNode) {
		return active === node.value || active.startsWith(`${node.value}>`) || active.startsWith(`${node.value}.`);
	}

	function isNodeExpanded(node: FacetNode) {
		return normalizedQuery !== '' || isActivePath(node);
	}
</script>

{#snippet facetNode(node: FacetNode, level = 0)}
	<div>
		{#if node.children.length > 0}
			<details class="group" open={isNodeExpanded(node)}>
				<summary
					class="grid w-full cursor-pointer list-none grid-cols-[18px_minmax(0,1fr)_auto] items-start gap-1 rounded-md hover:bg-zinc-50 [&::-webkit-details-marker]:hidden"
					style={`padding-left: ${level * 12}px`}
					aria-label={`Toggle ${node.value}`}
				>
					<span class="mt-0.5 flex size-5 items-center justify-center rounded-sm text-zinc-500">
						<ChevronRight size={13} class="group-open:hidden" />
						<ChevronDown size={13} class="hidden group-open:block" />
					</span>
					<span title={node.value} class={`block min-w-0 rounded-md px-1.5 py-1 text-left text-xs leading-4 [overflow-wrap:anywhere] ${active === node.value ? 'bg-zinc-950 text-white' : 'text-zinc-700'}`}>{node.label}</span>
					<span class={`pt-1 text-[11px] ${active === node.value ? 'text-zinc-200' : 'text-zinc-400'}`}>{node.count.toLocaleString()}</span>
				</summary>
				<div class="mt-1 flex flex-col gap-1">
					{#if node.count > 0}
						<div class="grid grid-cols-[18px_minmax(0,1fr)_auto] items-start gap-1" style={`padding-left: ${(level + 1) * 12}px`}>
							<span class="mx-auto mt-3 h-px w-3 bg-zinc-200"></span>
							<button
								type="button"
								title={node.value}
								class={`min-w-0 rounded-md px-1.5 py-1 text-left text-xs leading-4 transition [overflow-wrap:anywhere] ${active === node.value ? 'bg-zinc-950 text-white' : 'text-zinc-700 hover:bg-zinc-100'}`}
								on:click={() => onPick(kind, node.value)}
							>
								<span class="block">All {node.label}</span>
							</button>
							<span class={`pt-1 text-[11px] ${active === node.value ? 'text-zinc-200' : 'text-zinc-400'}`}>{node.count.toLocaleString()}</span>
						</div>
					{/if}
					{#each node.children as child}
						{@render facetNode(child, level + 1)}
					{/each}
				</div>
			</details>
		{:else}
			<div class="grid grid-cols-[18px_minmax(0,1fr)_auto] items-start gap-1" style={`padding-left: ${level * 12}px`}>
				<span class="mx-auto mt-3 h-px w-3 bg-zinc-200"></span>
				<button
					type="button"
					title={node.value}
					class={`min-w-0 rounded-md px-1.5 py-1 text-left text-xs leading-4 transition [overflow-wrap:anywhere] ${active === node.value ? 'bg-zinc-950 text-white' : 'text-zinc-700 hover:bg-zinc-100'}`}
					on:click={() => onPick(kind, node.value)}
				>
					<span class="block">{node.label}</span>
				</button>
				<span class={`pt-1 text-[11px] ${active === node.value ? 'text-zinc-200' : 'text-zinc-400'}`}>{node.count.toLocaleString()}</span>
			</div>
		{/if}
	</div>
{/snippet}

<section class="rounded-md border border-zinc-200 bg-white">
	<button
		type="button"
		class="flex w-full items-center justify-between gap-2 px-2.5 py-2 text-left"
		aria-expanded={expanded}
		aria-label={`${expanded ? 'Collapse' : 'Expand'} ${title} facets`}
		on:click={() => (expanded = !expanded)}
	>
		<span class="flex min-w-0 items-center gap-2">
			{#if expanded}
				<ChevronDown size={15} class="shrink-0 text-zinc-500" />
			{:else}
				<ChevronRight size={15} class="shrink-0 text-zinc-500" />
			{/if}
			<span class="truncate text-[11px] font-semibold uppercase tracking-wide text-zinc-600">{title}</span>
		</span>
		<span class="rounded-md bg-zinc-100 px-1.5 py-0.5 text-[11px] text-zinc-500">{entries.length.toLocaleString()}</span>
	</button>

	{#if expanded}
		<div class="border-t border-zinc-100 p-2">
			<label class="relative block">
				<Search class="pointer-events-none absolute left-2 top-1/2 -translate-y-1/2 text-zinc-400" size={14} />
				<input
					class="h-7 w-full rounded-md border border-zinc-200 bg-white pl-7 pr-2 text-[11px] outline-none transition focus:border-zinc-400 focus:ring-2 focus:ring-zinc-100"
					value={facetQuery}
					placeholder={`Find ${title.toLowerCase()}`}
					on:input={(event) => updateFacetQuery(event.currentTarget.value)}
				/>
			</label>

			{#key `${active}:${normalizedQuery}:${page}`}
				<div class="mt-2 flex flex-col gap-1">
					{#if visibleTree.length === 0}
						<div class="rounded-md border border-dashed border-zinc-200 px-2 py-3 text-center text-[11px] text-zinc-500">
							No matching facets
						</div>
					{:else}
						{#each visibleTree as node}
							{@render facetNode(node)}
						{/each}
					{/if}
				</div>
			{/key}

			<div class="mt-2 flex items-center justify-between gap-2 border-t border-zinc-100 pt-2">
				<span class="text-[11px] text-zinc-500">{entries.length.toLocaleString()} values</span>
				<div class="flex items-center gap-1">
					<button
						type="button"
						class="inline-flex h-8 shrink-0 items-center justify-center rounded-md px-3 text-xs font-medium text-zinc-700 transition-colors hover:bg-zinc-100 hover:text-zinc-950 disabled:pointer-events-none disabled:opacity-50"
						disabled={page === 0}
						aria-label={`Previous ${title} facet page`}
						on:click={() => (page = Math.max(0, page - 1))}
					>
						Prev
					</button>
					<span class="min-w-12 text-center text-[11px] text-zinc-500">{page + 1}/{pageCount}</span>
					<button
						type="button"
						class="inline-flex h-8 shrink-0 items-center justify-center rounded-md px-3 text-xs font-medium text-zinc-700 transition-colors hover:bg-zinc-100 hover:text-zinc-950 disabled:pointer-events-none disabled:opacity-50"
						disabled={page >= pageCount - 1}
						aria-label={`Next ${title} facet page`}
						on:click={() => (page = Math.min(pageCount - 1, page + 1))}
					>
						Next
					</button>
				</div>
			</div>
		</div>
	{/if}
</section>
