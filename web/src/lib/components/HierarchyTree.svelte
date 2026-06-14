<script lang="ts">
	import { ChevronDown, ChevronRight, Network, Search } from '@lucide/svelte';
	import { getHierarchyChildren, type HierarchyNode } from '$lib/api';

	export let onOpenTerm: (loincNum: string) => void;
	export let onBrowseNode: (node: HierarchyNode) => void;

	let expanded = true;
	let query = '';
	let loading = false;
	let error = '';
	let roots: HierarchyNode[] = [];
	let searchResults: HierarchyNode[] = [];
	let expandedKeys = new Set<string>();
	let childrenByNodeId = new Map<string, HierarchyNode[]>();
	let loadedKeys = new Set<string>();

	$: normalizedQuery = query.trim();
	$: visibleRoots = normalizedQuery ? searchResults : roots;

	void loadRoots();

	async function loadRoots() {
		loading = true;
		error = '';
		try {
			const response = await getHierarchyChildren();
			roots = response.results;
			await preloadHierarchy(response.results, 5, 2);
		} catch (err) {
			error = errorMessage(err);
		} finally {
			loading = false;
		}
	}

	async function preloadHierarchy(nodes: HierarchyNode[], levelsRemaining: number, expandLevelsRemaining: number) {
		if (levelsRemaining <= 0 || nodes.length === 0) return;
		const settledChildren = await Promise.allSettled(
			nodes
				.filter((node) => node.hasChildren && !loadedKeys.has(node.nodeId))
				.map(async (node) => {
					const response = await getHierarchyChildren({ parentNodeId: node.nodeId });
					return { node, children: response.results };
				}),
		);
		const nextChildren = settledChildren
			.filter((item): item is PromiseFulfilledResult<{ node: HierarchyNode; children: HierarchyNode[] }> => item.status === 'fulfilled')
			.map((item) => item.value);
		if (nextChildren.length === 0) return;
		const nextChildrenByNodeId = new Map(childrenByNodeId);
		const nextLoadedKeys = new Set(loadedKeys);
		const nextExpandedKeys = new Set(expandedKeys);
		const descendants: HierarchyNode[] = [];
		for (const item of nextChildren) {
			nextChildrenByNodeId.set(item.node.nodeId, item.children);
			nextLoadedKeys.add(item.node.nodeId);
			if (expandLevelsRemaining > 0) {
				nextExpandedKeys.add(item.node.nodeId);
			}
			descendants.push(...item.children);
		}
		childrenByNodeId = nextChildrenByNodeId;
		loadedKeys = nextLoadedKeys;
		expandedKeys = nextExpandedKeys;
		await preloadHierarchy(descendants, levelsRemaining - 1, Math.max(0, expandLevelsRemaining - 1));
	}

	async function updateQuery(value: string) {
		query = value;
		if (!normalizedQuery) {
			searchResults = [];
			return;
		}
		loading = true;
		error = '';
		try {
			const response = await getHierarchyChildren({ q: normalizedQuery });
			searchResults = response.results;
		} catch (err) {
			error = errorMessage(err);
		} finally {
			loading = false;
		}
	}

	async function toggleNode(node: HierarchyNode) {
		if (!node.hasChildren) {
			pickNode(node);
			return;
		}
		const next = new Set(expandedKeys);
		if (next.has(node.nodeId)) {
			next.delete(node.nodeId);
			expandedKeys = next;
			return;
		}
		next.add(node.nodeId);
		expandedKeys = next;
		if (!loadedKeys.has(node.nodeId)) {
			loading = true;
			error = '';
			try {
				await loadNodeChildren(node);
			} catch (err) {
				error = errorMessage(err);
			} finally {
				loading = false;
			}
		}
	}

	function activateNode(node: HierarchyNode) {
		pickNode(node);
	}

	async function loadNodeChildren(node: HierarchyNode) {
		const response = await getHierarchyChildren({ parentNodeId: node.nodeId });
		childrenByNodeId = new Map(childrenByNodeId).set(node.nodeId, response.results);
		loadedKeys = new Set(loadedKeys).add(node.nodeId);
	}

	function pickNode(node: HierarchyNode) {
		if (node.isTerm) {
			onOpenTerm(node.code);
			return;
		}
		onBrowseNode(node);
	}

	function childrenFor(node: HierarchyNode) {
		return childrenByNodeId.get(node.nodeId) ?? [];
	}

	function errorMessage(err: unknown) {
		return err instanceof Error ? err.message : String(err);
	}
</script>

{#snippet treeNode(node: HierarchyNode, level = 0)}
	<div>
		<div class="grid grid-cols-[18px_minmax(0,1fr)_54px] items-start gap-1 rounded-md hover:bg-zinc-50" style={`padding-left: ${level * 12}px`}>
			<button
				type="button"
				class="mt-0.5 flex size-5 items-center justify-center rounded-sm text-zinc-500 hover:bg-zinc-100 hover:text-zinc-900 disabled:opacity-40"
				aria-expanded={expandedKeys.has(node.nodeId)}
				aria-label={`${expandedKeys.has(node.nodeId) ? 'Collapse' : 'Expand'} ${node.label}`}
				disabled={!node.hasChildren}
				on:click={() => toggleNode(node)}
			>
				{#if node.hasChildren}
					{#if expandedKeys.has(node.nodeId)}<ChevronDown size={13} />{:else}<ChevronRight size={13} />{/if}
				{:else}
					<span class="h-px w-3 bg-zinc-200"></span>
				{/if}
			</button>
			<button
				type="button"
				title={`${node.label} (${node.code})`}
				class="min-w-0 rounded-md px-1.5 py-1 text-left text-xs leading-4 text-zinc-700 transition hover:bg-zinc-100 [overflow-wrap:anywhere]"
				on:click={() => activateNode(node)}
			>
				<span class="block">{node.label || node.code}</span>
				<span class="mt-0.5 block font-mono text-[10px] text-zinc-400">{node.code}</span>
			</button>
			<button
				type="button"
				class="rounded px-1 py-0.5 text-right text-[10px] leading-3 text-zinc-400 hover:bg-zinc-100 hover:text-zinc-700"
				aria-label={`Browse ${node.label} terms`}
				on:click={() => pickNode(node)}
			>
				<span class="block" title="Child branches">{node.childCount.toLocaleString()} ch</span>
				<span class="block" title="Descendant items">{node.termCount.toLocaleString()} items</span>
			</button>
		</div>
		{#if expandedKeys.has(node.nodeId)}
			<div class="mt-1 flex flex-col gap-1">
				{#each childrenFor(node) as child (child.nodeId)}
					{@render treeNode(child, level + 1)}
				{/each}
			</div>
		{/if}
	</div>
{/snippet}

<section class="flex min-h-0 flex-1 flex-col rounded-md border border-zinc-200 bg-white">
	<button
		type="button"
		class="flex w-full shrink-0 items-center justify-between gap-2 px-2.5 py-2 text-left"
		aria-expanded={expanded}
		aria-label={`${expanded ? 'Collapse' : 'Expand'} hierarchy browser`}
		on:click={() => (expanded = !expanded)}
	>
		<span class="flex min-w-0 items-center gap-2">
			{#if expanded}<ChevronDown size={15} class="shrink-0 text-zinc-500" />{:else}<ChevronRight size={15} class="shrink-0 text-zinc-500" />{/if}
			<Network size={14} class="shrink-0 text-zinc-500" />
			<span class="truncate text-[11px] font-semibold uppercase tracking-wide text-zinc-600">Hierarchy browser</span>
		</span>
		<span class="rounded-md bg-zinc-100 px-1.5 py-0.5 text-[11px] text-zinc-500">{roots.length.toLocaleString()}</span>
	</button>

	{#if expanded}
		<div class="flex min-h-0 flex-1 flex-col border-t border-zinc-100 p-2">
			<label class="relative block shrink-0">
				<Search class="pointer-events-none absolute left-2 top-1/2 -translate-y-1/2 text-zinc-400" size={14} />
				<input
					class="h-7 w-full rounded-md border border-zinc-200 bg-white pl-7 pr-2 text-[11px] outline-none transition focus:border-zinc-400 focus:ring-2 focus:ring-zinc-100"
					value={query}
					placeholder="Find hierarchy"
					on:input={(event) => updateQuery(event.currentTarget.value)}
				/>
			</label>
			{#if error}
				<div class="mt-2 shrink-0 rounded-md border border-red-200 bg-red-50 px-2 py-2 text-[11px] text-red-700">{error}</div>
			{/if}
			<div class="mt-2 flex min-h-0 flex-1 flex-col gap-1 overflow-auto">
				{#if loading && visibleRoots.length === 0}
					<div class="rounded-md bg-zinc-50 px-2 py-3 text-[11px] text-zinc-500">Loading hierarchy...</div>
				{:else if visibleRoots.length === 0}
					<div class="rounded-md border border-dashed border-zinc-200 px-2 py-3 text-center text-[11px] text-zinc-500">No hierarchy rows</div>
				{:else}
					{#each visibleRoots as node (node.nodeId)}
						{@render treeNode(node)}
					{/each}
				{/if}
			</div>
		</div>
	{/if}
</section>
