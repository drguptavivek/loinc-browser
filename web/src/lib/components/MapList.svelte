<script lang="ts">
	import { ChevronDown, ChevronRight, Copy, Download, RotateCcw, Upload, X } from '@lucide/svelte';
	import { onMount } from 'svelte';
	import { matchNames, type NameMatch, type SearchResult } from '$lib/api';
	import { DOMAINS, domainById, type DomainId } from '$lib/domains';
	import { copyText, downloadText, toCSV } from '$lib/copy';
	import { setup, setupParams } from '$lib/setup';
	import {
		buildExportRows,
		dedupeNames,
		detectDelimiter,
		exportColumns,
		normalizeName,
		parseDelimitedTable,
		parseNameLines,
		chunk,
		type MatchStatus,
		type ParsedTable,
	} from '$lib/maplist';
	import { readXlsxFirstSheet } from '$lib/xlsx';
	import Button from '$lib/components/Button.svelte';
	import Badge from '$lib/components/Badge.svelte';
	import Input from '$lib/components/Input.svelte';
	import Select from '$lib/components/Select.svelte';
	import Checkbox from '$lib/components/Checkbox.svelte';
	import EmptyState from '$lib/components/EmptyState.svelte';

	type Props = {
		onOpen: (loincNum: string) => void;
		onClose: () => void;
	};

	let { onOpen, onClose }: Props = $props();

	const STORAGE_KEY = 'loinc.maplist.v1';
	const BATCH_SIZE = 200;

	// --- Step 1: input ---
	let step = $state<'input' | 'review'>('input');
	let pasteText = $state('');
	let fileName = $state<string | null>(null);
	let table = $state<ParsedTable | null>(null); // set once a file is parsed
	let hasHeader = $state(true);
	let nameColumnIndex = $state('0');
	let localCodeColumnIndex = $state('-1'); // '-1' = none
	let domainId = $state<DomainId>('lab');
	let fileError = $state<string | null>(null);

	// Rows as the user's original data, independent of the pasted/file distinction:
	// header is null and each row is a single-cell [name] for pasted text.
	const activeTable = $derived<ParsedTable>(
		table ?? { header: null, rows: parseNameLines(pasteText).map((n) => [n]) },
	);
	const columnCount = $derived(activeTable.header?.length ?? (activeTable.rows[0]?.length ?? 1));
	const allNames = $derived(activeTable.rows.map((r) => r[Number(nameColumnIndex)] ?? '').filter((n) => n.trim() !== ''));
	const uniqueNames = $derived(dedupeNames(allNames));

	function resetInputFile() {
		table = null;
		fileName = null;
		fileError = null;
		nameColumnIndex = '0';
		localCodeColumnIndex = '-1';
	}

	async function handleFile(e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		fileError = null;
		if (/\.xls$/i.test(file.name)) {
			fileError = "Old .xls files aren't supported — save as .xlsx or CSV.";
			table = null;
			fileName = null;
			return;
		}
		if (/\.xlsx$/i.test(file.name)) {
			try {
				const grid = await readXlsxFirstSheet(await file.arrayBuffer());
				const multiColumn = (grid[0]?.length ?? 1) > 1;
				table = tableFromGrid(grid, hasHeader && multiColumn);
				fileName = file.name;
				nameColumnIndex = '0';
				localCodeColumnIndex = '-1';
			} catch (err) {
				fileError = err instanceof Error ? err.message : 'Could not read this .xlsx file.';
				table = null;
				fileName = null;
			}
			return;
		}
		const text = await file.text();
		const delimiter = detectDelimiter(text);
		const parsed = parseDelimitedTable(text, delimiter, hasHeader && looksMultiColumn(text, delimiter));
		table = parsed;
		fileName = file.name;
		nameColumnIndex = '0';
		localCodeColumnIndex = '-1';
	}

	function looksMultiColumn(text: string, delimiter: string): boolean {
		const firstLine = text.split(/\r\n|\r|\n/).find((l) => l.trim() !== '') ?? '';
		return firstLine.includes(delimiter);
	}

	// tableFromGrid mirrors parseDelimitedTable's header-split/blank-row-drop behaviour for a
	// grid that's already split into cells (from readXlsxFirstSheet), so both file paths feed
	// the same ParsedTable shape into the rest of the flow.
	function tableFromGrid(grid: string[][], splitHeader: boolean): ParsedTable {
		const rows = grid.filter((r) => r.some((cell) => cell.trim() !== ''));
		if (rows.length === 0) return { header: null, rows: [] };
		if (splitHeader) return { header: rows[0], rows: rows.slice(1) };
		return { header: null, rows };
	}

	function onHasHeaderToggle() {
		if (!fileName) return;
		// re-derive header split against the last parsed text isn't kept; simplest correct
		// behaviour is to ask the user to re-choose the file. Keep it lazy: just re-split
		// in place using the current rows+header.
		if (!table) return;
		if (hasHeader && table.header === null && table.rows.length > 0) {
			table = { header: table.rows[0], rows: table.rows.slice(1) };
		} else if (!hasHeader && table.header !== null) {
			table = { header: null, rows: [table.header, ...table.rows] };
		}
		nameColumnIndex = '0';
		localCodeColumnIndex = '-1';
	}

	// --- Step 2: matching ---
	let matching = $state(false);
	let cancelRequested = $state(false);
	let progressDone = $state(0);
	let progressTotal = $state(0);
	let matchError = $state<string | null>(null);
	let matches = $state<Record<string, NameMatch>>({}); // keyed by normalizeName(name)

	// Per-review-name user picks: candidate loincNum, 'none', or undefined (pending).
	let picks = $state<Record<string, string>>({});
	// Confident-bucket rows the user has unchecked.
	let confidentRejected = $state<Record<string, boolean>>({});

	let confidentCollapsed = $state(true);
	let filterText = $state('');
	let saveNotice = $state<string | null>(null);

	async function startMatching() {
		if (uniqueNames.length === 0) return;
		step = 'review';
		matching = true;
		cancelRequested = false;
		matchError = null;
		matches = {};
		picks = {};
		confidentRejected = {};
		progressDone = 0;
		const params = { ...(domainId === 'panels' ? {} : domainById(domainId).params), ...setupParams($setup) };
		const batches = chunk(uniqueNames, BATCH_SIZE);
		progressTotal = uniqueNames.length;
		try {
			for (const batch of batches) {
				if (cancelRequested) break;
				const res = await matchNames(batch, params);
				const next = { ...matches };
				for (const m of res.matches) next[normalizeName(m.name)] = m;
				matches = next;
				progressDone += batch.length;
			}
		} catch (err) {
			matchError = err instanceof Error ? err.message : 'Matching failed.';
		} finally {
			matching = false;
		}
	}

	function cancelMatching() {
		cancelRequested = true;
	}

	// --- Review derived state ---
	type Row = { name: string; key: string; match?: NameMatch };
	const rows = $derived<Row[]>(uniqueNames.map((name) => ({ name, key: normalizeName(name), match: matches[normalizeName(name)] })));
	const confidentRows = $derived(rows.filter((r) => r.match?.bucket === 'confident'));
	const reviewRows = $derived(rows.filter((r) => r.match?.bucket === 'review'));
	const noneRows = $derived(rows.filter((r) => r.match?.bucket === 'none'));

	function matchesFilter(name: string): boolean {
		if (!filterText.trim()) return true;
		return name.toLowerCase().includes(filterText.trim().toLowerCase());
	}
	const visibleConfident = $derived(confidentRows.filter((r) => matchesFilter(r.name)));
	const visibleReview = $derived(reviewRows.filter((r) => matchesFilter(r.name)));
	const visibleNone = $derived(noneRows.filter((r) => matchesFilter(r.name)));

	function isConfidentAccepted(key: string): boolean {
		return !confidentRejected[key];
	}
	function toggleConfident(key: string) {
		confidentRejected = { ...confidentRejected, [key]: !confidentRejected[key] };
	}
	function uncheckAllConfident() {
		const next = { ...confidentRejected };
		for (const r of confidentRows) next[r.key] = true;
		confidentRejected = next;
	}
	function checkAllConfident() {
		const next = { ...confidentRejected };
		for (const r of confidentRows) delete next[r.key];
		confidentRejected = next;
	}

	function pickCandidate(key: string, loincNum: string) {
		picks = { ...picks, [key]: loincNum };
	}
	function pickNone(key: string) {
		picks = { ...picks, [key]: 'none' };
	}

	const confidentAcceptedCount = $derived(confidentRows.filter((r) => isConfidentAccepted(r.key)).length);
	const reviewedCount = $derived(reviewRows.filter((r) => picks[r.key] !== undefined).length);
	const pendingCount = $derived(reviewRows.length - reviewedCount);
	const noMatchCount = $derived(noneRows.length);

	function axisChips(c: SearchResult): string {
		return [c.property, c.system, c.scale, c.method].filter(Boolean).join(' · ');
	}

	async function copyName(name: string) {
		await copyText(name);
	}

	// --- Export ---
	function resolveForExport(key: string): { loincCode: string; loincLongCommonName: string; matchStatus: MatchStatus } | undefined {
		const match = matches[key];
		if (!match) return undefined;
		if (match.bucket === 'confident') {
			if (!isConfidentAccepted(key)) return undefined;
			const top = match.candidates[0];
			if (!top) return undefined;
			return { loincCode: top.loincNum, loincLongCommonName: top.longCommonName, matchStatus: 'confident' };
		}
		if (match.bucket === 'review') {
			const pick = picks[key];
			if (!pick) return { loincCode: '', loincLongCommonName: '', matchStatus: 'pending' };
			if (pick === 'none') return undefined;
			const chosen = match.candidates.find((c) => c.loincNum === pick);
			if (!chosen) return undefined;
			return { loincCode: chosen.loincNum, loincLongCommonName: chosen.longCommonName, matchStatus: 'reviewed' };
		}
		return undefined;
	}

	function exportCSV() {
		const rowsOut = buildExportRows(activeTable.header, activeTable.rows, Number(nameColumnIndex), (key) => resolveForExport(key));
		const columns = exportColumns(activeTable.header);
		const csv = toCSV(rowsOut, columns);
		const base = fileName ? fileName.replace(/\.[^.]+$/, '') : 'local-tests';
		downloadText(`${base}-loinc.csv`, csv);
	}

	// --- Persistence ---
	function persist() {
		try {
			const payload = {
				step,
				pasteText,
				fileName,
				table,
				hasHeader,
				nameColumnIndex,
				localCodeColumnIndex,
				domainId,
				matches,
				picks,
				confidentRejected,
			};
			localStorage.setItem(STORAGE_KEY, JSON.stringify(payload));
			saveNotice = null;
		} catch {
			saveNotice = 'Not saved (storage limit reached).';
		}
	}

	onMount(() => {
		try {
			const raw = localStorage.getItem(STORAGE_KEY);
			if (!raw) return;
			const saved = JSON.parse(raw);
			step = saved.step ?? 'input';
			pasteText = saved.pasteText ?? '';
			fileName = saved.fileName ?? null;
			table = saved.table ?? null;
			hasHeader = saved.hasHeader ?? true;
			nameColumnIndex = saved.nameColumnIndex ?? '0';
			localCodeColumnIndex = saved.localCodeColumnIndex ?? '-1';
			domainId = saved.domainId ?? 'lab';
			matches = saved.matches ?? {};
			picks = saved.picks ?? {};
			confidentRejected = saved.confidentRejected ?? {};
		} catch {
			// ignore corrupt/blocked storage
		}
	});

	$effect(() => {
		// Track every piece of state that should survive a reload.
		void [step, pasteText, fileName, table, hasHeader, nameColumnIndex, localCodeColumnIndex, domainId, matches, picks, confidentRejected];
		persist();
	});

	let confirmingStartOver = $state(false);
	function startOver() {
		step = 'input';
		pasteText = '';
		resetInputFile();
		domainId = 'lab';
		matches = {};
		picks = {};
		confidentRejected = {};
		filterText = '';
		confirmingStartOver = false;
		try {
			localStorage.removeItem(STORAGE_KEY);
		} catch {
			// ignore
		}
	}
</script>

<div class="flex h-full flex-col gap-4 overflow-y-auto p-4 text-zinc-800">
	<div class="flex items-center justify-between">
		<h2 class="text-lg font-semibold text-zinc-950">Map a test master</h2>
		<div class="flex items-center gap-2">
			{#if !confirmingStartOver}
				<Button variant="ghost" size="sm" on:click={() => (confirmingStartOver = true)}>
					<RotateCcw size={14} />
					Start over
				</Button>
			{:else}
				<span class="text-xs text-zinc-500">Discard everything?</span>
				<Button variant="outline" size="sm" on:click={startOver}>Yes, start over</Button>
				<Button variant="ghost" size="sm" on:click={() => (confirmingStartOver = false)}>Cancel</Button>
			{/if}
			<Button variant="ghost" size="icon" ariaLabel="Close" on:click={onClose}>
				<X size={16} />
			</Button>
		</div>
	</div>

	{#if saveNotice}
		<p class="text-xs text-amber-700">{saveNotice}</p>
	{/if}

	{#if step === 'input'}
		<div class="flex flex-col gap-4">
			<div>
				<label class="text-sm font-medium text-zinc-700">Domain</label>
				<Select bind:value={domainId} className="max-w-xs">
					{#each DOMAINS as d (d.id)}
						<option value={d.id}>{d.label}</option>
					{/each}
				</Select>
			</div>

			<div>
				<label class="text-sm font-medium text-zinc-700" for="maplist-paste">Paste test names (one per line)</label>
				<textarea
					id="maplist-paste"
					class="mt-1 h-40 w-full rounded-md border border-zinc-200 bg-white p-3 text-sm outline-none focus:border-zinc-400 focus:ring-2 focus:ring-zinc-100"
					placeholder={'S. Sodium\nHbA1c\nCBC'}
					bind:value={pasteText}
					on:input={resetInputFile}
				></textarea>
			</div>

			<div class="flex items-center gap-2 text-sm text-zinc-500">
				<div class="h-px flex-1 bg-zinc-200"></div>
				or
				<div class="h-px flex-1 bg-zinc-200"></div>
			</div>

			<div>
				<label class="flex w-fit cursor-pointer items-center gap-2 rounded-md border border-dashed border-zinc-300 px-4 py-3 text-sm text-zinc-600 hover:bg-zinc-50">
					<Upload size={16} />
					{fileName ?? 'Upload .csv, .tsv, .txt, or .xlsx'}
					<input type="file" accept=".csv,.tsv,.txt,.xlsx" class="hidden" on:change={handleFile} />
				</label>
				{#if fileError}
					<p class="mt-2 text-sm text-red-600">{fileError}</p>
				{/if}
			</div>

			{#if table && columnCount > 1}
				<div class="flex flex-wrap items-end gap-4 rounded-md border border-zinc-200 bg-zinc-50 p-3">
					<Checkbox checked={hasHeader} on:change={() => { hasHeader = !hasHeader; onHasHeaderToggle(); }} />
					<span class="-ml-2 text-sm text-zinc-600">First row is a header</span>
					<div>
						<label class="text-xs font-medium text-zinc-600">Test name column</label>
						<Select bind:value={nameColumnIndex} className="w-48">
							{#each Array.from({ length: columnCount }) as _, i (i)}
								<option value={String(i)}>{table.header ? table.header[i] : `Column ${i + 1}`}</option>
							{/each}
						</Select>
					</div>
					<div>
						<label class="text-xs font-medium text-zinc-600">Local code column (optional)</label>
						<Select bind:value={localCodeColumnIndex} className="w-48">
							<option value="-1">None</option>
							{#each Array.from({ length: columnCount }) as _, i (i)}
								<option value={String(i)}>{table.header ? table.header[i] : `Column ${i + 1}`}</option>
							{/each}
						</Select>
					</div>
				</div>
			{/if}

			<div>
				<Button disabled={uniqueNames.length === 0} on:click={startMatching}>
					Find matches ({uniqueNames.length} name{uniqueNames.length === 1 ? '' : 's'})
				</Button>
			</div>
		</div>
	{:else}
		<div class="flex flex-col gap-4">
			{#if matching}
				<div class="rounded-md border border-zinc-200 bg-white p-4">
					<div class="flex items-center justify-between text-sm text-zinc-600">
						<span>Matching {progressDone} of {progressTotal}…</span>
						<Button variant="outline" size="sm" on:click={cancelMatching}>Cancel</Button>
					</div>
					<div class="mt-2 h-2 w-full overflow-hidden rounded-full bg-zinc-100">
						<div
							class="h-full bg-zinc-950 transition-all"
							style={`width: ${progressTotal ? (progressDone / progressTotal) * 100 : 0}%`}
						></div>
					</div>
				</div>
			{:else if matchError}
				<p class="text-sm text-red-600">{matchError}</p>
			{/if}

			{#if !matching}
				<div class="flex flex-wrap items-center gap-3 rounded-md border border-zinc-200 bg-zinc-50 p-3 text-sm text-zinc-600">
					<span>{confidentAcceptedCount} confident accepted</span>
					<span>·</span>
					<span>{reviewedCount} reviewed</span>
					<span>·</span>
					<span>{pendingCount} pending</span>
					<span>·</span>
					<span>{noMatchCount} no match</span>
				</div>

				<Input placeholder="Filter by name…" bind:value={filterText} ariaLabel="Filter rows by name" />

				<!-- Confident -->
				<section class="rounded-md border border-zinc-200 bg-white">
					<button
						type="button"
						class="flex w-full items-center justify-between px-4 py-3 text-left"
						on:click={() => (confidentCollapsed = !confidentCollapsed)}
					>
						<span class="text-sm font-semibold text-zinc-950">
							Confident ({confidentRows.length})
						</span>
						{#if confidentCollapsed}<ChevronRight size={16} />{:else}<ChevronDown size={16} />{/if}
					</button>
					{#if !confidentCollapsed}
						<div class="border-t border-zinc-200 p-3">
							<div class="mb-2 flex justify-end">
								<Button variant="ghost" size="sm" on:click={() => (confidentRows.some((r) => isConfidentAccepted(r.key)) ? uncheckAllConfident() : checkAllConfident())}>
									{confidentRows.some((r) => isConfidentAccepted(r.key)) ? 'Uncheck all' : 'Check all'}
								</Button>
							</div>
							{#if visibleConfident.length === 0}
								<p class="p-2 text-sm text-zinc-500">No confident matches.</p>
							{:else}
								<table class="w-full text-sm">
									<thead>
										<tr class="text-left text-xs uppercase text-zinc-400">
											<th class="w-8"></th>
											<th class="py-1">Name</th>
											<th class="py-1">Code</th>
											<th class="py-1">Long common name</th>
										</tr>
									</thead>
									<tbody>
										{#each visibleConfident as r (r.key)}
											{@const top = r.match?.candidates[0]}
											<tr class="border-t border-zinc-100">
												<td class="py-1.5"><Checkbox checked={isConfidentAccepted(r.key)} on:change={() => toggleConfident(r.key)} /></td>
												<td class="py-1.5">{r.name}</td>
												<td class="py-1.5 font-mono text-xs">{top?.loincNum}</td>
												<td class="py-1.5 text-zinc-600">{top?.longCommonName}</td>
											</tr>
										{/each}
									</tbody>
								</table>
							{/if}
						</div>
					{/if}
				</section>

				<!-- Needs a look -->
				<section class="rounded-md border border-zinc-200 bg-white">
					<div class="border-b border-zinc-200 px-4 py-3">
						<span class="text-sm font-semibold text-zinc-950">Needs a look ({reviewRows.length})</span>
					</div>
					<div class="flex flex-col divide-y divide-zinc-100">
						{#if visibleReview.length === 0}
							<p class="p-4 text-sm text-zinc-500">Nothing to review.</p>
						{/if}
						{#each visibleReview as r (r.key)}
							<div class="p-3">
								<div class="mb-2 flex items-center justify-between">
									<span class="text-sm font-medium text-zinc-900">{r.name}</span>
									{#if picks[r.key]}
										<Badge variant="secondary">reviewed</Badge>
									{/if}
								</div>
								<div class="flex flex-col gap-2">
									{#each r.match?.candidates ?? [] as c (c.loincNum)}
										<label class="flex cursor-pointer items-start gap-2 rounded-md border border-zinc-200 p-2 text-sm hover:bg-zinc-50">
											<input
												type="radio"
												name={`pick-${r.key}`}
												class="mt-1"
												checked={picks[r.key] === c.loincNum}
												on:change={() => pickCandidate(r.key, c.loincNum)}
											/>
											<span class="flex-1">
												<span class="font-mono text-xs">{c.loincNum}</span>
												<span class="ml-2">{c.longCommonName}</span>
												{#if c.localizedName}
													<span class="mt-0.5 block text-xs text-zinc-600">{c.localizedName}</span>
												{/if}
												<span class="mt-1 block text-xs text-zinc-500">
													{axisChips(c)}
													{#if c.localName ?? c.clciName}
														<Badge variant="secondary" className="ml-1">{c.localName ?? c.clciName}</Badge>
													{/if}
												</span>
											</span>
											<button type="button" class="text-xs text-zinc-400 underline hover:text-zinc-700" on:click={(e) => { e.preventDefault(); onOpen(c.loincNum); }}>
												details
											</button>
										</label>
									{/each}
									<label class="flex cursor-pointer items-center gap-2 rounded-md border border-zinc-200 p-2 text-sm hover:bg-zinc-50">
										<input type="radio" name={`pick-${r.key}`} checked={picks[r.key] === 'none'} on:change={() => pickNone(r.key)} />
										None of these
									</label>
								</div>
							</div>
						{/each}
					</div>
				</section>

				<!-- No match -->
				<section class="rounded-md border border-zinc-200 bg-white">
					<div class="border-b border-zinc-200 px-4 py-3">
						<span class="text-sm font-semibold text-zinc-950">No match ({noneRows.length})</span>
					</div>
					<div class="flex flex-col divide-y divide-zinc-100">
						{#if visibleNone.length === 0}
							<p class="p-4 text-sm text-zinc-500">None.</p>
						{/if}
						{#each visibleNone as r (r.key)}
							<div class="flex items-center justify-between p-3 text-sm">
								<span>{r.name}</span>
								<Button variant="ghost" size="sm" on:click={() => copyName(r.name)}>
									<Copy size={14} />
									Copy name
								</Button>
							</div>
						{/each}
					</div>
				</section>

				{#if rows.length === 0}
					<EmptyState title="No results yet" body="Matching hasn't returned any rows." />
				{/if}

				<div>
					<Button on:click={exportCSV}>
						<Download size={14} />
						Download CSV
					</Button>
				</div>
			{/if}
		</div>
	{/if}
</div>
