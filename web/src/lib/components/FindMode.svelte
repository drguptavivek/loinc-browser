<script lang="ts">
	import { untrack } from 'svelte';
	import { Search, Copy, Check, Plus, ShoppingBasket, ListChecks, Settings, Sparkles } from '@lucide/svelte';
	import { getVersion, searchTerms, searchPanels, type SearchResponse, type SearchResult } from '$lib/api';
	import { DOMAINS, domainById, type DomainId } from '$lib/domains';
	import { basket, addToBasket } from '$lib/basket';
	import { copyText, formatTerm } from '$lib/copy';
	import { setup, setupParams } from '$lib/setup';
	import { loincVersion } from '$lib/version';
	import Button from './Button.svelte';
	import Badge from './Badge.svelte';
	import EmptyState from './EmptyState.svelte';

	let {
		onOpen,
		onMapList,
		onOpenBasket,
		onOpenSetup,
		onAsk,
		initialQuery = '',
		initialDomain = 'lab',
		disabled = false,
		onStateChange
	}: {
		onOpen: (loincNum: string) => void;
		onMapList: () => void;
		onOpenBasket: () => void;
		onOpenSetup: () => void;
		onAsk: (prefill?: string) => void;
		initialQuery?: string;
		initialDomain?: string;
		// true while a drawer (term card, basket, setup, ask) covers the list: keys belong to the drawer then
		disabled?: boolean;
		// reports query and domain so App can keep them in the URL (Back, shared links)
		onStateChange?: (query: string, domain: string) => void;
	} = $props();

	// initialQuery/initialDomain are one-time seeds: App remounts FindMode via {#key} when they
	// should change, so intentionally capture only the initial value here.
	let query = $state(untrack(() => initialQuery));
	let domainId = $state<DomainId>(domainById(untrack(() => initialDomain)).id);
	let radModality = $state('');
	let radRegion = $state('');
	let response = $state<SearchResponse | null>(null);
	let loading = $state(false);
	let error = $state('');
	let activeIndex = $state(-1);
	let copiedCode = $state('');
	let version = $state<string | undefined>(undefined);
	let commonCodesLabel = $state<string | undefined>(undefined);
	loincVersion().then((v) => (version = v));
	getVersion()
		.then((v) => (commonCodesLabel = v.commonCodes?.label))
		.catch(() => {});

	let inputEl: HTMLInputElement | undefined = $state();
	let rowEls: (HTMLElement | undefined)[] = [];

	// Search box is the page's purpose, so focus it on mount instead of using the autofocus
	// attribute (flagged by a11y_autofocus).
	$effect(() => {
		inputEl?.focus();
	});

	const domain = $derived(domainById(domainId));
	const setupSummary = $derived(
		[$setup.lang, $setup.commonCodesOnly ? 'common codes only' : '', $setup.orderableOnly ? 'orderable only' : '']
			.filter(Boolean)
			.join(' · ')
	);
	const results = $derived(response?.results ?? []);
	// Nudge toward Ask AI once the query looks conversational or the exact-match search came up empty.
	const askSuggestionVisible = $derived(
		query.trim().length > 0 && (query.trim().split(/\s+/).length >= 4 || (!!response && results.length === 0))
	);
	const exampleQueries = $derived(
		domain.hint
			.replace(/^e\.g\.\s*/i, '')
			.replace(/^search all loinc terms$/i, 'sodium, glucose, hba1c')
			.split(',')
			.map((s) => s.trim())
			.filter(Boolean)
			.slice(0, 3)
	);

	let requestSeq = 0;
	let debounceTimer: ReturnType<typeof setTimeout> | undefined;

	function runSearch() {
		const q = query.trim();
		if (!q) {
			response = null;
			error = '';
			loading = false;
			activeIndex = -1;
			return;
		}
		const seq = ++requestSeq;
		loading = true;
		error = '';
		const params = {
			...domain.params,
			...setupParams($setup),
			q,
			limit: 25,
			...(domainId === 'radiology' && radModality ? { radModality } : {}),
			...(domainId === 'radiology' && radRegion ? { radRegion } : {})
		};
		const call = domain.panels ? searchPanels(params) : searchTerms(params);
		call
			.then((res) => {
				if (seq !== requestSeq) return;
				response = res;
				activeIndex = res.results.length ? 0 : -1;
			})
			.catch((err: unknown) => {
				if (seq !== requestSeq) return;
				response = null;
				error = err instanceof Error ? err.message : 'Search failed';
			})
			.finally(() => {
				if (seq === requestSeq) loading = false;
			});
	}

	function scheduleSearch() {
		if (debounceTimer) clearTimeout(debounceTimer);
		debounceTimer = setTimeout(runSearch, 250);
	}

	$effect(() => {
		// Re-run whenever query, domain, radiology filters, or setup (lang/filters) change.
		void query;
		void domainId;
		void radModality;
		void radRegion;
		void $setup;
		scheduleSearch();
		onStateChange?.(query, domainId);
	});

	function selectDomain(id: DomainId) {
		domainId = id;
	}

	function useExample(q: string) {
		query = q;
		inputEl?.focus();
	}

	function scrollActiveIntoView() {
		rowEls[activeIndex]?.scrollIntoView({ block: 'nearest' });
	}

	function moveActive(delta: number) {
		if (!results.length) return;
		activeIndex = Math.min(results.length - 1, Math.max(0, activeIndex + delta));
		scrollActiveIntoView();
	}

	async function copyRow(r: SearchResult) {
		const ok = await copyText(formatTerm(r, $setup.copyFormat, version));
		if (ok) {
			copiedCode = r.loincNum;
			setTimeout(() => {
				if (copiedCode === r.loincNum) copiedCode = '';
			}, 1200);
		}
	}

	function addRow(r: SearchResult) {
		addToBasket({ loincNum: r.loincNum, longCommonName: r.longCommonName, exampleUcum: r.exampleUcum });
	}

	function handleKeydown(event: KeyboardEvent) {
		if (disabled) return;
		const target = event.target as HTMLElement | null;
		const isInput = target?.tagName === 'INPUT' || target?.tagName === 'TEXTAREA';
		if (event.key === '/' && !isInput) {
			event.preventDefault();
			inputEl?.focus();
			return;
		}
		if (target !== inputEl && isInput) return;
		switch (event.key) {
			case 'ArrowDown':
				event.preventDefault();
				moveActive(1);
				break;
			case 'ArrowUp':
				event.preventDefault();
				moveActive(-1);
				break;
			case 'Enter': {
				const row = results[activeIndex];
				if (row) {
					event.preventDefault();
					onOpen(row.loincNum);
				}
				break;
			}
			case 'c':
			case 'C': {
				if (target === inputEl) break;
				const row = results[activeIndex];
				if (row) {
					event.preventDefault();
					copyRow(row);
				}
				break;
			}
			case 'Escape':
				if (query) {
					event.preventDefault();
					query = '';
				}
				break;
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="flex flex-col gap-4">
	<div class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<p class="text-xs font-medium uppercase tracking-wide text-zinc-500">Find a LOINC code</p>
			<h1 class="text-lg font-semibold text-zinc-950">Search by local test name</h1>
		</div>
		<div class="flex items-center gap-2">
			<Button variant="outline" size="sm" on:click={onMapList}>
				<ListChecks size={14} />
				Map a list
			</Button>
			<Button variant="outline" size="sm" on:click={onOpenBasket} ariaLabel="Open basket">
				<ShoppingBasket size={14} />
				Basket ({$basket.length})
			</Button>
			<Button variant="outline" size="sm" on:click={onOpenSetup} ariaLabel="Open setup">
				<Settings size={14} />
				{setupSummary || 'Setup'}
			</Button>
			<Button variant="outline" size="sm" on:click={() => onAsk()} ariaLabel="Ask AI">
				<Sparkles size={14} />
				Ask AI
			</Button>
		</div>
	</div>

	<div class="flex flex-wrap gap-2" role="tablist" aria-label="Domain">
		{#each DOMAINS as d (d.id)}
			<button
				type="button"
				role="tab"
				aria-selected={d.id === domainId}
				class={`rounded-full border px-3 py-1.5 text-sm font-medium transition-colors ${
					d.id === domainId
						? 'border-zinc-950 bg-zinc-950 text-white'
						: 'border-zinc-200 bg-white text-zinc-700 hover:bg-zinc-50'
				}`}
				onclick={() => selectDomain(d.id)}
			>
				{d.label}
			</button>
		{/each}
	</div>

	<div class="relative">
		<Search size={18} class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-zinc-400" />
		<input
			type="text"
			bind:value={query}
			bind:this={inputEl}
			aria-label="Search LOINC terms"
			placeholder={domain.hint}
			autocomplete="off"
			class="mt-1 h-12 w-full rounded-md border border-zinc-200 bg-white pl-10 pr-3 text-base normal-case tracking-normal text-zinc-800 outline-none transition focus:border-zinc-400 focus:ring-2 focus:ring-zinc-100"
		/>
	</div>

	{#if askSuggestionVisible}
		<button
			type="button"
			class="flex w-fit items-center gap-1.5 rounded-full border border-dashed border-zinc-300 bg-zinc-50 px-3 py-1 text-xs text-zinc-600 hover:bg-zinc-100"
			onclick={() => onAsk(query.trim())}
		>
			<Sparkles size={12} />
			Ask AI about &ldquo;{query.trim()}&rdquo;
		</button>
	{/if}

	{#if domainId === 'radiology'}
		<div class="flex flex-wrap gap-3">
			<label class="flex flex-col text-xs font-medium text-zinc-600">
				Modality
				<input
					type="text"
					bind:value={radModality}
					placeholder="e.g. CT, US, MR"
					class="mt-1 h-9 w-40 rounded-md border border-zinc-200 bg-white px-3 text-sm text-zinc-800 outline-none focus:border-zinc-400 focus:ring-2 focus:ring-zinc-100"
				/>
			</label>
			<label class="flex flex-col text-xs font-medium text-zinc-600">
				Body region
				<input
					type="text"
					bind:value={radRegion}
					placeholder="e.g. Chest, Abdomen"
					class="mt-1 h-9 w-48 rounded-md border border-zinc-200 bg-white px-3 text-sm text-zinc-800 outline-none focus:border-zinc-400 focus:ring-2 focus:ring-zinc-100"
				/>
			</label>
		</div>
	{/if}

	{#if !query.trim()}
		<div class="flex flex-col gap-2 rounded-lg border border-dashed border-zinc-200 bg-white p-4">
			<p class="text-sm text-zinc-500">Try one of these:</p>
			<div class="flex flex-wrap gap-2">
				{#each exampleQueries as ex (ex)}
					<button
						type="button"
						class="rounded-full border border-zinc-200 bg-zinc-50 px-3 py-1 text-sm text-zinc-700 hover:bg-zinc-100"
						onclick={() => useExample(ex)}
					>
						{ex}
					</button>
				{/each}
			</div>
		</div>
	{:else if error}
		<EmptyState title="Search failed" body={error} />
	{:else if loading && !response}
		<p class="text-sm text-zinc-500">Searching...</p>
	{:else if response && results.length === 0}
		<EmptyState title="No results" body="Try another name or a different domain." />
	{:else if response}
		<div class="flex flex-col gap-1">
			<div class="flex flex-wrap items-center gap-2 text-xs text-zinc-500">
				<span>{response.total} result{response.total === 1 ? '' : 's'}</span>
				{#if response.notice}
					<span class="text-zinc-400">· {response.notice}</span>
				{/if}
				{#if response.synonyms?.length}
					<span class="text-zinc-400">· Searched: {response.synonyms.join(', ')}</span>
				{/if}
				{#if response.ignoredWords?.length}
					<span class="text-zinc-400">· Ignored: {response.ignoredWords.join(', ')}</span>
				{/if}
			</div>

			<ul class="flex flex-col gap-1.5" role="listbox" aria-label="Search results">
				{#each results as r, i (r.loincNum)}
					<li
						id={`find-row-${i}`}
						role="option"
						aria-selected={i === activeIndex}
						bind:this={rowEls[i]}
					>
						<div
							class={`flex items-center gap-3 rounded-lg border bg-white px-3 py-2.5 transition-colors ${
								i === activeIndex ? 'border-zinc-400 ring-1 ring-zinc-200' : 'border-zinc-200'
							}`}
						>
							<button
								type="button"
								class="flex min-w-0 flex-1 flex-col items-start gap-1 text-left"
								onclick={() => onOpen(r.loincNum)}
								onmouseenter={() => (activeIndex = i)}
							>
								<div class="flex flex-wrap items-center gap-2">
									<span class="font-mono text-xs text-zinc-500">{r.loincNum}</span>
									<span class="font-semibold text-zinc-950">{r.longCommonName}</span>
									{#if r.status && r.status !== 'ACTIVE'}
										<Badge variant="warning">{r.status}</Badge>
									{/if}
									{#if r.localName ?? r.clciName}
										<Badge variant="secondary" title={commonCodesLabel ?? 'Local name'}>{r.localName ?? r.clciName}</Badge>
									{/if}
								</div>
								{#if r.localizedName}
									<div class="text-sm text-zinc-600">{r.localizedName}</div>
								{/if}
								<div class="flex flex-wrap gap-1.5 text-xs text-zinc-500">
									{#if r.component}<span>{r.component}</span>{/if}
									{#if r.property}<span>· {r.property}</span>{/if}
									{#if r.system}<span>· {r.system}</span>{/if}
									{#if r.scale}<span>· {r.scale}</span>{/if}
									{#if r.method}<span>· {r.method}</span>{/if}
									{#if r.timeAspect}
										<span>· {r.timeAspect}</span>
									{/if}
									{#if r.exampleUcum}<span class="text-zinc-400">· {r.exampleUcum}</span>{/if}
								</div>
							</button>
							<div class="flex shrink-0 items-center gap-1">
								<Button
									variant="ghost"
									size="icon"
									ariaLabel={`Copy code ${r.loincNum}`}
									on:click={() => copyRow(r)}
								>
									{#if copiedCode === r.loincNum}
										<Check size={14} class="text-emerald-600" />
									{:else}
										<Copy size={14} />
									{/if}
								</Button>
								<Button variant="ghost" size="icon" ariaLabel={`Add ${r.loincNum} to basket`} on:click={() => addRow(r)}>
									<Plus size={14} />
								</Button>
							</div>
						</div>
					</li>
				{/each}
			</ul>
		</div>
	{/if}
</div>
