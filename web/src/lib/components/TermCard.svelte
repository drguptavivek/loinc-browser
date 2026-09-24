<script lang="ts">
	import { AlertTriangle, Check, Copy, ExternalLink, Layers, Plus, X } from '@lucide/svelte';
	import Badge from '$lib/components/Badge.svelte';
	import Button from '$lib/components/Button.svelte';
	import DetailField from '$lib/components/DetailField.svelte';
	import EmptyState from '$lib/components/EmptyState.svelte';
	import { getPanelItems, getPanelMemberships, getTerm, getTermFit, getTermRelationships, searchTerms } from '$lib/api';
	import type { PanelItem, SearchResult, Term, TermAccessory } from '$lib/api';
	import { COPY_FORMATS, copyText, formatTerm, type CopyFormatId } from '$lib/copy';
	import { addToBasket, basket, removeFromBasket } from '$lib/basket';
	import { setup, setupParams } from '$lib/setup';
	import { loincVersion } from '$lib/version';

	let {
		loincNum,
		onOpen,
		onClose,
		onOpenInExplorer,
	}: {
		loincNum: string;
		onOpen: (n: string) => void;
		onClose: () => void;
		onOpenInExplorer?: (n: string) => void;
	} = $props();

	let term = $state<Term | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let variants = $state<SearchResult[]>([]);
	let variantsExpanded = $state(false);
	let panelMemberships = $state<TermAccessory[]>([]);
	let hasPanelItems = $state(false);
	let panelItems = $state<PanelItem[]>([]);
	let version = $state<string | undefined>(undefined);
	let copiedId = $state<CopyFormatId | null>(null);
	let copiedTimer: ReturnType<typeof setTimeout> | undefined;

	const VARIANT_PAGE_SIZE = 12;

	function statusVariant(value: string) {
		if (value === 'ACTIVE') return 'default' as const;
		if (value === 'DISCOURAGED' || value === 'DEPRECATED') return 'warning' as const;
		return 'secondary' as const;
	}

	$effect(() => {
		const code = loincNum;
		const lang = $setup.lang;
		let cancelled = false;

		loading = true;
		error = null;
		term = null;
		variants = [];
		variantsExpanded = false;
		panelMemberships = [];
		panelItems = [];
		hasPanelItems = false;

		loincVersion().then((v) => {
			if (!cancelled) version = v;
		});

		getTerm(code, lang)
			.then((t) => {
				if (cancelled) return;
				term = t;
				loading = false;

				// /terms/{n} leaves mapTo empty; the relationships view carries the replacement codes.
				if ((t.status === 'DEPRECATED' || t.status === 'DISCOURAGED') && !t.mapTo?.length) {
					getTermRelationships(code)
						.then((graph) => {
							if (!cancelled && term && graph.mapTo?.length) term = { ...term, mapTo: graph.mapTo };
						})
						.catch(() => {});
				}

				if (t.component) {
					searchTerms({ component: t.component, componentFamily: true, limit: 50 })
						.then((res) => {
							if (!cancelled) variants = res.results;
						})
						.catch(() => {});
				}

				getPanelMemberships(code)
					.then((page) => {
						if (!cancelled) panelMemberships = page.results;
					})
					.catch(() => {});

				getTermFit(code)
					.then((fit) => {
						if (cancelled) return;
						hasPanelItems = fit.hasPanelItems;
						if (fit.hasPanelItems) {
							getPanelItems(code)
								.then((page) => {
									if (!cancelled) panelItems = page.results;
								})
								.catch(() => {});
						}
					})
					.catch(() => {});
			})
			.catch((err) => {
				if (cancelled) return;
				error = err instanceof Error ? err.message : String(err);
				loading = false;
			});

		return () => {
			cancelled = true;
		};
	});

	let isDeprecated = $derived(term?.status === 'DEPRECATED' || term?.status === 'DISCOURAGED');

	type VariantRow = { loincNum: string; longCommonName: string; property: string; timeAspect: string; system: string; scale: string; method: string; isCurrent: boolean };

	let variantRows = $derived.by((): VariantRow[] => {
		if (!term) return [];
		const others = variants
			.filter((v) => v.loincNum !== term!.loincNum)
			.sort((a, b) => {
				const ra = a.commonTestRank > 0 ? a.commonTestRank : Infinity;
				const rb = b.commonTestRank > 0 ? b.commonTestRank : Infinity;
				return ra - rb;
			});
		const current: VariantRow = {
			loincNum: term.loincNum,
			longCommonName: term.longCommonName,
			property: term.property,
			timeAspect: term.timeAspect,
			system: term.system,
			scale: term.scale,
			method: term.method,
			isCurrent: true,
		};
		return [
			current,
			...others.map((v) => ({ loincNum: v.loincNum, longCommonName: v.longCommonName, property: v.property, timeAspect: v.timeAspect, system: v.system, scale: v.scale, method: v.method, isCurrent: false })),
		];
	});

	let visibleVariantRows = $derived(variantsExpanded ? variantRows : variantRows.slice(0, VARIANT_PAGE_SIZE));

	let inBasket = $derived(term ? $basket.some((i) => i.loincNum === term!.loincNum) : false);

	async function handleCopy(id: CopyFormatId) {
		if (!term) return;
		const ok = await copyText(formatTerm(term, id, version));
		if (!ok) return;
		copiedId = id;
		clearTimeout(copiedTimer);
		copiedTimer = setTimeout(() => {
			copiedId = null;
		}, 1500);
	}

	function toggleBasket() {
		if (!term) return;
		if (inBasket) removeFromBasket(term.loincNum);
		else addToBasket({ loincNum: term.loincNum, longCommonName: term.longCommonName, exampleUcum: term.exampleUcum });
	}

	function panelMemberLabel(item: PanelItem): string {
		return item.displayNameForForm || item.childTerm?.longCommonName || item.childLoincNum;
	}
</script>

<div class="flex items-center justify-between border-b border-zinc-200 px-4 py-3">
	<h2 class="text-sm font-semibold">Term</h2>
	<Button variant="ghost" size="icon" ariaLabel="Close" on:click={onClose}><X size={16} /></Button>
</div>

<div class="min-h-0 flex-1 overflow-auto">
	{#if loading}
		<div class="space-y-3 p-4">
			<div class="h-8 animate-pulse rounded-md bg-zinc-100"></div>
			<div class="h-24 animate-pulse rounded-md bg-zinc-100"></div>
			<div class="h-48 animate-pulse rounded-md bg-zinc-100"></div>
		</div>
	{:else if error}
		<div class="p-4"><EmptyState title="Could not load term" body={error} /></div>
	{:else if term}
		<div class="flex flex-col gap-4 p-4">
			<!-- a) header -->
			<div>
				<div class="flex items-center gap-2">
					<span class="font-mono text-lg font-semibold">{term.loincNum}</span>
					<Badge variant={statusVariant(term.status)}>{term.status || 'UNKNOWN'}</Badge>
				</div>
				<h3 class="mt-1 text-lg font-semibold leading-snug">{term.longCommonName}</h3>
				{#if term.localizedName}<p class="mt-0.5 text-sm text-zinc-600">{term.localizedName}</p>{/if}
				{#if term.shortName}<p class="mt-0.5 text-sm text-zinc-500">{term.shortName}</p>{/if}
				{#if term.localName ?? term.clciName}<p class="mt-1 text-sm text-zinc-600">Local name: {term.localName ?? term.clciName}</p>{/if}
			</div>

			<!-- b) deprecated / discouraged banner -->
			{#if isDeprecated}
				<div class="rounded-md border border-amber-200 bg-amber-50 p-3">
					<div class="flex items-center gap-2 text-sm font-semibold text-amber-900">
						<AlertTriangle size={16} />
						This code is {term.status === 'DEPRECATED' ? 'deprecated' : 'discouraged'}
					</div>
					{#if term.mapTo?.length}
						<div class="mt-2 flex flex-col gap-2">
							{#each term.mapTo as item}
								<div>
									<Button variant="outline" size="sm" on:click={() => onOpen(item.mapTo)}>Use {item.mapTo} instead</Button>
									{#if item.comment}<p class="mt-1 text-xs text-amber-800">{item.comment}</p>{/if}
								</div>
							{/each}
						</div>
					{/if}
				</div>
			{/if}

			<!-- c) copy block -->
			<div class="rounded-md border border-zinc-200 p-3">
				<div class="flex flex-wrap items-center gap-2">
					{#each COPY_FORMATS as format}
						<Button variant={format.id === $setup.copyFormat ? 'default' : 'outline'} size="sm" on:click={() => handleCopy(format.id)}>
							{#if copiedId === format.id}<Check size={14} />Copied{:else}<Copy size={14} />{format.label}{/if}
						</Button>
					{/each}
					<Button variant={inBasket ? 'default' : 'outline'} size="sm" on:click={toggleBasket}>
						{#if inBasket}<Check size={14} />In basket{:else}<Plus size={14} />Add to basket{/if}
					</Button>
				</div>
				<details class="mt-2 text-xs text-zinc-600">
					<summary class="cursor-pointer select-none font-medium text-zinc-700">Preview FHIR / HL7 formats</summary>
					<pre class="mt-2 whitespace-pre-wrap break-words rounded bg-zinc-50 p-2 font-mono text-[11px] text-zinc-700">{formatTerm(term, 'fhir', version)}
{formatTerm(term, 'hl7v2', version)}</pre>
				</details>
			</div>

			<!-- d) facts -->
			<div class="grid grid-cols-2 gap-2 text-sm">
				<DetailField label="Component" value={term.component} />
				<DetailField label="Property" value={term.property} />
				<DetailField label="Time" value={term.timeAspect} />
				<DetailField label="System" value={term.system} />
				<DetailField label="Scale" value={term.scale} />
				<DetailField label="Method" value={term.method} />
				<DetailField label="Class" value={term.class} />
				<DetailField label="Order/Observation" value={term.orderObs} />
			</div>
			{#if term.exampleUcum}<DetailField label="Example units (UCUM)" value={term.exampleUcum} />{/if}
			{#if term.mapperComment}<DetailField label="Mapping note" value={term.mapperComment} />{/if}

			<!-- e) similar terms -->
			{#if variantRows.length > 1}
				<div>
					<h4 class="text-xs font-semibold uppercase tracking-wide text-zinc-500">Similar terms (variants)</h4>
					<p class="mt-1 text-xs text-zinc-500">Same analyte — pick the row whose specimen, units and timing match your lab.</p>
					<div class="mt-2 overflow-x-auto rounded-md border border-zinc-200">
						<table class="min-w-full divide-y divide-zinc-100 text-xs">
							<thead class="bg-zinc-50 text-zinc-500">
								<tr>
									<th class="px-2 py-1.5 text-left font-medium">Code</th>
									<th class="px-2 py-1.5 text-left font-medium">Property</th>
									<th class="px-2 py-1.5 text-left font-medium">Time</th>
									<th class="px-2 py-1.5 text-left font-medium">System</th>
									<th class="px-2 py-1.5 text-left font-medium">Scale</th>
									<th class="px-2 py-1.5 text-left font-medium">Method</th>
								</tr>
							</thead>
							<tbody class="divide-y divide-zinc-100">
								{#each visibleVariantRows as row}
									<tr
										class={row.isCurrent ? 'bg-zinc-100' : 'cursor-pointer hover:bg-zinc-50'}
										onclick={() => !row.isCurrent && onOpen(row.loincNum)}
									>
										<td class="px-2 py-1.5"><span class="whitespace-nowrap font-mono">{row.loincNum}{#if row.isCurrent} <span class="text-zinc-400">(this)</span>{/if}</span><span class="block max-w-[16rem] truncate text-zinc-500" title={row.longCommonName}>{row.longCommonName}</span></td>
										<td class="px-2 py-1.5" class:bg-amber-50={!row.isCurrent && row.property !== term.property}>{row.property || '-'}</td>
										<td class="px-2 py-1.5" class:bg-amber-50={!row.isCurrent && row.timeAspect !== term.timeAspect}>{row.timeAspect || '-'}</td>
										<td class="px-2 py-1.5" class:bg-amber-50={!row.isCurrent && row.system !== term.system}>{row.system || '-'}</td>
										<td class="px-2 py-1.5" class:bg-amber-50={!row.isCurrent && row.scale !== term.scale}>{row.scale || '-'}</td>
										<td class="px-2 py-1.5" class:bg-amber-50={!row.isCurrent && row.method !== term.method}>{row.method || '-'}</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
					{#if variantRows.length > VARIANT_PAGE_SIZE && !variantsExpanded}
						<button type="button" class="mt-2 text-xs font-medium text-zinc-700 hover:underline" onclick={() => (variantsExpanded = true)}>
							Show all {variantRows.length}
						</button>
					{/if}
				</div>
			{/if}

			<!-- f) panels -->
			{#if panelMemberships.length}
				<div>
					<h4 class="text-xs font-semibold uppercase tracking-wide text-zinc-500">Part of these panels</h4>
					<div class="mt-2 rounded-md border border-zinc-200 divide-y divide-zinc-100">
						{#each panelMemberships as item}
							<button type="button" class="flex w-full items-start justify-between gap-2 px-3 py-2 text-left text-sm hover:bg-zinc-50" onclick={() => onOpen(item.code)}>
								<span class="min-w-0 break-words">{item.title}</span>
								<span class="shrink-0 font-mono text-xs text-zinc-500">{item.code}</span>
							</button>
						{/each}
					</div>
				</div>
			{/if}

			{#if hasPanelItems && panelItems.length}
				<div>
					<h4 class="flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wide text-zinc-500"><Layers size={13} />Panel members</h4>
					<div class="mt-2 rounded-md border border-zinc-200 divide-y divide-zinc-100">
						{#each panelItems as item}
							<button type="button" class="flex w-full items-start justify-between gap-2 px-3 py-2 text-left text-sm hover:bg-zinc-50" onclick={() => onOpen(item.childLoincNum)}>
								<span class="min-w-0 break-words">{panelMemberLabel(item)}</span>
								<span class="shrink-0 font-mono text-xs text-zinc-500">{item.childLoincNum}</span>
							</button>
						{/each}
					</div>
				</div>
			{/if}

			<!-- g) footer -->
			<div class="mt-2 flex flex-col gap-2 border-t border-zinc-200 pt-3">
				{#if onOpenInExplorer}
					<Button variant="outline" size="sm" on:click={() => onOpenInExplorer?.(term!.loincNum)}>
						<ExternalLink size={14} />
						Open in full explorer
					</Button>
				{/if}
				<p class="text-[11px] leading-4 text-zinc-500">
					LOINC® is a registered trademark of Regenstrief Institute, Inc. Content from LOINC is copyright © Regenstrief
					Institute, Inc. and the LOINC Committee, available at no cost under the license at
					<a class="underline underline-offset-2 hover:text-zinc-700" href="https://loinc.org/license/" target="_blank" rel="noreferrer">https://loinc.org/license/</a>.
				</p>
			</div>
		</div>
	{/if}
</div>
