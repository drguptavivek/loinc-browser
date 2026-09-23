<script lang="ts">
	import { onMount } from 'svelte';
	import Button from './Button.svelte';
	import Input from './Input.svelte';
	import Select from './Select.svelte';
	import Field from './Field.svelte';
	import Badge from './Badge.svelte';
	import EmptyState from './EmptyState.svelte';
	import { runApiConsoleRequest, type ApiConsoleResult } from '$lib/api';

	// One preset per representative call from scripts/capture-exemplars.sh (the
	// exemplar catalogue for docs/FHIR_TERMINOLOGY_PLAN.md), with paths made
	// relative to this app's own /fhir and /searchapi clones instead of the
	// official hosts.
	type Preset = {
		id: string;
		group: string;
		label: string;
		method: 'GET' | 'POST';
		path: string;
		params: { key: string; value: string }[];
		body?: string;
		docAnchor: string;
	};

	const presets: Preset[] = [
		{ id: 'metadata', group: 'Metadata', label: 'CapabilityStatement', method: 'GET', path: '/fhir/metadata', params: [], docAnchor: 'metadata' },
		{ id: 'metadata-terminology', group: 'Metadata', label: 'TerminologyCapabilities', method: 'GET', path: '/fhir/metadata', params: [{ key: 'mode', value: 'terminology' }], docAnchor: 'metadata' },

		{ id: 'cs-search', group: 'FHIR CodeSystem', label: 'Search: CodeSystem?url=', method: 'GET', path: '/fhir/CodeSystem', params: [{ key: 'url', value: 'http://loinc.org' }], docAnchor: 'codesystem' },
		{ id: 'cs-read', group: 'FHIR CodeSystem', label: 'Read: CodeSystem/loinc-2.82', method: 'GET', path: '/fhir/CodeSystem/loinc-2.82', params: [], docAnchor: 'codesystem' },
		{ id: 'cs-lookup-term', group: 'FHIR CodeSystem', label: '$lookup: term 718-7', method: 'GET', path: '/fhir/CodeSystem/$lookup', params: [{ key: 'system', value: 'http://loinc.org' }, { key: 'code', value: '718-7' }], docAnchor: 'lookup' },
		{ id: 'cs-lookup-part', group: 'FHIR CodeSystem', label: '$lookup: LP part', method: 'GET', path: '/fhir/CodeSystem/$lookup', params: [{ key: 'system', value: 'http://loinc.org' }, { key: 'code', value: 'LP384441-4' }], docAnchor: 'lookup' },
		{ id: 'cs-validate', group: 'FHIR CodeSystem', label: '$validate-code: 718-7', method: 'GET', path: '/fhir/CodeSystem/$validate-code', params: [{ key: 'url', value: 'http://loinc.org' }, { key: 'code', value: '718-7' }], docAnchor: 'validate-code' },
		{ id: 'cs-subsumes', group: 'FHIR CodeSystem', label: '$subsumes: LP384441-4 / 30064-0', method: 'GET', path: '/fhir/CodeSystem/$subsumes', params: [{ key: 'system', value: 'http://loinc.org' }, { key: 'codeA', value: 'LP384441-4' }, { key: 'codeB', value: '30064-0' }], docAnchor: 'subsumes' },

		{ id: 'vs-search', group: 'FHIR ValueSet', label: 'Search: ValueSet?url=', method: 'GET', path: '/fhir/ValueSet', params: [{ key: 'url', value: 'http://loinc.org/vs/LL1162-8' }], docAnchor: 'valueset' },
		{ id: 'vs-read', group: 'FHIR ValueSet', label: 'Read: ValueSet/LL1162-8', method: 'GET', path: '/fhir/ValueSet/LL1162-8', params: [], docAnchor: 'valueset' },
		{ id: 'vs-expand', group: 'FHIR ValueSet', label: '$expand: LL1162-8', method: 'GET', path: '/fhir/ValueSet/$expand', params: [{ key: 'url', value: 'http://loinc.org/vs/LL1162-8' }], docAnchor: 'expand' },
		{ id: 'vs-expand-count', group: 'FHIR ValueSet', label: '$expand: all LOINC, count=2', method: 'GET', path: '/fhir/ValueSet/$expand', params: [{ key: 'url', value: 'http://loinc.org/vs' }, { key: 'count', value: '2' }], docAnchor: 'expand' },
		{
			id: 'vs-expand-inline',
			group: 'FHIR ValueSet',
			label: '$expand: inline compose (POST)',
			method: 'POST',
			path: '/fhir/ValueSet/$expand',
			params: [],
			body: JSON.stringify(
				{
					resourceType: 'Parameters',
					parameter: [
						{ name: 'count', valueInteger: 3 },
						{ name: 'valueSet', resource: { resourceType: 'ValueSet', status: 'active', compose: { include: [{ system: 'http://loinc.org', filter: [{ property: 'COMPONENT', op: '=', value: 'LP14449-0' }] }] } } },
					],
				},
				null,
				2,
			),
			docAnchor: 'expand',
		},
		{ id: 'vs-validate', group: 'FHIR ValueSet', label: '$validate-code: LL1162-8', method: 'GET', path: '/fhir/ValueSet/LL1162-8/$validate-code', params: [{ key: 'system', value: 'http://loinc.org' }, { key: 'code', value: 'LA15679-6' }], docAnchor: 'validate-code' },
		{ id: 'vs-search-summary', group: 'FHIR ValueSet', label: 'Search: ValueSet?url= with _summary=true', method: 'GET', path: '/fhir/ValueSet', params: [{ key: 'url', value: 'http://loinc.org/vs/LL1162-8' }, { key: '_summary', value: 'true' }], docAnchor: 'summary-and-elements' },

		{ id: 'cm-search', group: 'FHIR ConceptMap', label: 'Search: loinc-to-ieee', method: 'GET', path: '/fhir/ConceptMap', params: [{ key: 'url', value: 'http://loinc.org/cm/loinc-to-ieee-11073-10101' }], docAnchor: 'conceptmap' },
		{ id: 'cm-read', group: 'FHIR ConceptMap', label: 'Read: loinc-to-ieee-11073-10101', method: 'GET', path: '/fhir/ConceptMap/loinc-to-ieee-11073-10101', params: [], docAnchor: 'conceptmap' },
		{ id: 'cm-translate', group: 'FHIR ConceptMap', label: '$translate: 11556-8 (IEEE)', method: 'GET', path: '/fhir/ConceptMap/$translate', params: [{ key: 'system', value: 'http://loinc.org' }, { key: 'code', value: '11556-8' }], docAnchor: 'translate' },
		{ id: 'cm-translate-radlex', group: 'FHIR ConceptMap', label: '$translate: 30657-1 (RadLex)', method: 'GET', path: '/fhir/ConceptMap/$translate', params: [{ key: 'system', value: 'http://loinc.org' }, { key: 'code', value: '30657-1' }], docAnchor: 'translate' },

		{ id: 'q-search', group: 'FHIR Questionnaire', label: 'Search: Questionnaire?url=', method: 'GET', path: '/fhir/Questionnaire', params: [{ key: 'url', value: 'http://loinc.org/q/89689-4' }], docAnchor: 'questionnaire' },
		{ id: 'q-read', group: 'FHIR Questionnaire', label: 'Read: Questionnaire/89689-4', method: 'GET', path: '/fhir/Questionnaire/89689-4', params: [], docAnchor: 'questionnaire' },

		{ id: 'sa-loincs', group: 'LOINC Search API', label: 'loincs: glucose', method: 'GET', path: '/searchapi/loincs', params: [{ key: 'query', value: 'glucose' }, { key: 'rows', value: '2' }], docAnchor: 'loinc-search-api-searchapiscope' },
		{ id: 'sa-parts', group: 'LOINC Search API', label: 'parts: glucose', method: 'GET', path: '/searchapi/parts', params: [{ key: 'query', value: 'glucose' }, { key: 'rows', value: '2' }], docAnchor: 'loinc-search-api-searchapiscope' },
		{ id: 'sa-answerlists', group: 'LOINC Search API', label: 'answerlists: yes', method: 'GET', path: '/searchapi/answerlists', params: [{ key: 'query', value: 'yes' }, { key: 'rows', value: '2' }], docAnchor: 'loinc-search-api-searchapiscope' },
		{ id: 'sa-groups', group: 'LOINC Search API', label: 'groups: glucose', method: 'GET', path: '/searchapi/groups', params: [{ key: 'query', value: 'glucose' }, { key: 'rows', value: '2' }], docAnchor: 'loinc-search-api-searchapiscope' },
		{ id: 'sa-filtercounts', group: 'LOINC Search API', label: 'loincs: glucose + filter counts', method: 'GET', path: '/searchapi/loincs', params: [{ key: 'query', value: 'glucose' }, { key: 'rows', value: '1' }, { key: 'includefiltercounts', value: 'true' }], docAnchor: 'loinc-search-api-searchapiscope' },
	];

	const groupOrder = ['Metadata', 'FHIR CodeSystem', 'FHIR ValueSet', 'FHIR ConceptMap', 'FHIR Questionnaire', 'LOINC Search API'];
	const groups = groupOrder.map((name) => ({ name, presets: presets.filter((p) => p.group === name) }));

	export let initialPresetId = '';

	let selectedPresetId = '';
	let method: 'GET' | 'POST' = 'GET';
	let path = '';
	let paramRows: { key: string; value: string }[] = [{ key: '', value: '' }];
	let rawBody = '';
	let docAnchor = '';
	let result: ApiConsoleResult | null = null;
	let loading = false;
	let sendError = '';
	let copyLabel = 'Copy URL';
	let copyCurlLabel = 'Copy curl';

	function loadPreset(preset: Preset) {
		selectedPresetId = preset.id;
		method = preset.method;
		path = preset.path;
		paramRows = preset.params.length ? preset.params.map((p) => ({ ...p })) : [{ key: '', value: '' }];
		rawBody = preset.body ?? '';
		docAnchor = preset.docAnchor;
		result = null;
		sendError = '';
	}

	onMount(() => {
		const preset = presets.find((p) => p.id === initialPresetId) ?? presets[0];
		loadPreset(preset);
	});

	function addParamRow() {
		paramRows = [...paramRows, { key: '', value: '' }];
	}

	function removeParamRow(index: number) {
		paramRows = paramRows.filter((_, i) => i !== index);
		if (paramRows.length === 0) paramRows = [{ key: '', value: '' }];
	}

	function buildQuery(): string {
		const query = new URLSearchParams();
		for (const row of paramRows) {
			if (row.key.trim() !== '') query.append(row.key.trim(), row.value);
		}
		const qs = query.toString();
		return qs ? `?${qs}` : '';
	}

	function buildParametersBody(): string {
		return JSON.stringify(
			{
				resourceType: 'Parameters',
				parameter: paramRows.filter((row) => row.key.trim() !== '').map((row) => ({ name: row.key.trim(), valueString: row.value })),
			},
			null,
			2,
		);
	}

	// isExpand: only $expand supports a raw pasted JSON body (an inline ValueSet
	// resource); every other POST operation sends a Parameters body built from
	// the editable param rows.
	$: isExpand = path.endsWith('$expand');
	$: requestURL = method === 'GET' ? `${path}${buildQuery()}` : path;
	$: fullURL = typeof window !== 'undefined' ? `${window.location.origin}${requestURL}` : requestURL;
	$: curlCommand = buildCurl();

	function buildCurl(): string {
		const origin = typeof window !== 'undefined' ? window.location.origin : '';
		if (method === 'GET') {
			return `curl '${origin}${requestURL}'`;
		}
		const body = isExpand && rawBody.trim() ? rawBody : buildParametersBody();
		return `curl -X POST '${origin}${path}' -H 'content-type: application/fhir+json' --data '${body.replace(/'/g, "'\\''")}'`;
	}

	async function send() {
		loading = true;
		sendError = '';
		result = null;
		try {
			const init: RequestInit = { method };
			if (method === 'POST') {
				const body = isExpand && rawBody.trim() ? rawBody : buildParametersBody();
				init.headers = { 'content-type': 'application/fhir+json' };
				init.body = body;
			}
			result = await runApiConsoleRequest(requestURL, init);
		} catch (err) {
			sendError = err instanceof Error ? err.message : String(err);
		} finally {
			loading = false;
		}
	}

	async function copyText(text: string, setLabel: (label: string) => void, resetLabel: string) {
		try {
			await navigator.clipboard.writeText(text);
			setLabel('Copied');
			setTimeout(() => setLabel(resetLabel), 1500);
		} catch {
			setLabel('Copy failed');
			setTimeout(() => setLabel(resetLabel), 1500);
		}
	}

	function prettyBody(): string {
		if (!result) return '';
		try {
			return JSON.stringify(JSON.parse(result.bodyText), null, 2);
		} catch {
			return result.bodyText;
		}
	}

	function isOperationOutcome(): boolean {
		if (!result) return false;
		try {
			const parsed = JSON.parse(result.bodyText);
			return parsed?.resourceType === 'OperationOutcome';
		} catch {
			return false;
		}
	}
</script>

<div class="flex h-full flex-col gap-4 p-4">
	<div class="rounded-md border border-zinc-200 bg-zinc-50 p-3 text-xs leading-5 text-zinc-600">
		Clients built against the Regenstrief-hosted services only change their base URL:
		<code class="rounded bg-white px-1 py-0.5">https://fhir.loinc.org</code> &rarr;
		<code class="rounded bg-white px-1 py-0.5">{typeof window !== 'undefined' ? window.location.origin : ''}/fhir</code>,
		<code class="rounded bg-white px-1 py-0.5">https://loinc.regenstrief.org/searchapi</code> &rarr;
		<code class="rounded bg-white px-1 py-0.5">{typeof window !== 'undefined' ? window.location.origin : ''}/searchapi</code>.
		See <a class="underline underline-offset-2" href="/docs/local-apis">the local APIs guide</a> for the full route list.
	</div>

	<div class="grid gap-4 lg:grid-cols-[220px_minmax(0,1fr)]">
		<nav aria-label="API presets" class="flex max-h-[70vh] flex-col gap-3 overflow-auto rounded-md border border-zinc-200 bg-white p-2">
			{#each groups as group}
				<div>
					<div class="flex items-center justify-between px-2 py-1">
						<div class="text-[11px] font-semibold uppercase tracking-wide text-zinc-500">{group.name}</div>
						<a class="text-[11px] underline underline-offset-2 text-zinc-400 hover:text-zinc-700" href={`/docs/local-apis#${group.presets[0]?.docAnchor ?? ''}`}>Docs</a>
					</div>
					<ul class="flex flex-col gap-0.5">
						{#each group.presets as preset}
							<li>
								<button
									type="button"
									class={`w-full rounded px-2 py-1.5 text-left text-xs leading-4 ${selectedPresetId === preset.id ? 'bg-zinc-950 text-white' : 'text-zinc-700 hover:bg-zinc-100'}`}
									aria-current={selectedPresetId === preset.id ? 'true' : undefined}
									on:click={() => loadPreset(preset)}
								>
									{preset.label}
								</button>
							</li>
						{/each}
					</ul>
				</div>
			{/each}
		</nav>

		<div class="flex flex-col gap-3">
			<section class="rounded-md border border-zinc-200 bg-white p-3">
				<div class="grid gap-3 sm:grid-cols-[140px_minmax(0,1fr)]">
					<Field label="Method">
						<Select bind:value={method}>
							<option value="GET">GET</option>
							<option value="POST">POST (Parameters)</option>
						</Select>
					</Field>
					<Field label="Path">
						<Input bind:value={path} ariaLabel="Request path" />
					</Field>
				</div>

				<div class="mt-3">
					<div class="flex items-center justify-between">
						<span class="text-xs font-semibold uppercase tracking-wide text-zinc-500">Parameters</span>
						<Button type="button" variant="outline" size="sm" on:click={addParamRow}>Add param</Button>
					</div>
					<div class="mt-2 flex flex-col gap-2">
						{#each paramRows as row, index}
							<div class="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] items-center gap-2">
								<Input bind:value={row.key} placeholder="key" ariaLabel={`Parameter ${index + 1} key`} />
								<Input bind:value={row.value} placeholder="value" ariaLabel={`Parameter ${index + 1} value`} />
								<Button type="button" variant="ghost" size="sm" ariaLabel={`Remove parameter ${index + 1}`} on:click={() => removeParamRow(index)}>Remove</Button>
							</div>
						{/each}
					</div>
				</div>

				{#if method === 'POST' && isExpand}
					<div class="mt-3">
						<Field label="Raw JSON body (optional; overrides the Parameters built from rows above)">
							<textarea
								class="mt-1 h-32 w-full rounded-md border border-zinc-200 bg-white p-2 font-mono text-xs text-zinc-800 outline-none focus:border-zinc-400 focus:ring-2 focus:ring-zinc-100"
								bind:value={rawBody}
								aria-label="Raw JSON request body"
							></textarea>
						</Field>
					</div>
				{/if}

				<div class="mt-3 flex flex-wrap items-center gap-2">
					<Button type="button" disabled={loading} on:click={send}>{loading ? 'Sending...' : 'Send request'}</Button>
					<Button type="button" variant="outline" size="sm" on:click={() => copyText(fullURL, (l) => (copyLabel = l), 'Copy URL')}>{copyLabel}</Button>
					<Button type="button" variant="outline" size="sm" on:click={() => copyText(curlCommand, (l) => (copyCurlLabel = l), 'Copy curl')}>{copyCurlLabel}</Button>
				</div>

				<div class="mt-3 rounded-md bg-zinc-50 p-2">
					<div class="text-[11px] font-semibold uppercase tracking-wide text-zinc-500">Request URL</div>
					<code class="mt-1 block break-all text-xs text-zinc-800">{fullURL}</code>
				</div>
			</section>

			<section class="flex-1 rounded-md border border-zinc-200 bg-white">
				<div class="flex flex-wrap items-center justify-between gap-2 border-b border-zinc-200 px-3 py-2">
					<h3 class="text-sm font-semibold">Response</h3>
					{#if result}
						<div class="flex flex-wrap items-center gap-2 text-xs text-zinc-600">
							<Badge variant={result.status < 300 ? 'default' : 'warning'}>{result.status} {result.statusText}</Badge>
							<Badge variant="secondary">{result.durationMs.toFixed(0)} ms</Badge>
							{#if result.contentType}<Badge variant="outline">{result.contentType}</Badge>{/if}
							{#if isOperationOutcome()}<Badge variant="warning">OperationOutcome</Badge>{/if}
						</div>
					{/if}
				</div>
				<div class="max-h-[50vh] overflow-auto p-3">
					{#if sendError}
						<p class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-800">{sendError}</p>
					{:else if loading}
						<div class="space-y-2">{#each Array(4) as _}<div class="h-6 animate-pulse rounded bg-zinc-100"></div>{/each}</div>
					{:else if !result}
						<EmptyState title="No request sent yet" body="Pick a preset, adjust parameters, then send." />
					{:else}
						<pre class={`whitespace-pre-wrap break-words text-xs leading-5 ${isOperationOutcome() ? 'text-red-800' : 'text-zinc-700'}`}>{prettyBody()}</pre>
					{/if}
				</div>
			</section>
		</div>
	</div>
</div>
