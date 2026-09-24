<script lang="ts">
	import { X } from '@lucide/svelte';
	import Button from '$lib/components/Button.svelte';
	import Checkbox from '$lib/components/Checkbox.svelte';
	import Field from '$lib/components/Field.svelte';
	import Input from '$lib/components/Input.svelte';
	import Select from '$lib/components/Select.svelte';
	import {
		getAgentSettings,
		getVersion,
		listAgentModels,
		saveAgentSettings,
		testAgent,
		type AgentSettings,
		type AgentTestResult,
		type VersionInfo,
	} from '$lib/api';
	import { COPY_FORMATS } from '$lib/copy';
	import { setup } from '$lib/setup';

	let { onClose }: { onClose: () => void } = $props();

	let version = $state<VersionInfo | null>(null);
	getVersion()
		.then((v) => (version = v))
		.catch(() => {});

	let agentSettings = $state<AgentSettings | null>(null);
	let agentBaseUrl = $state('');
	let agentModel = $state('');
	let agentThinking = $state(false);
	let agentApiKey = $state('');
	let agentClearApiKey = $state(false);
	let agentModels = $state<string[]>([]);
	let agentModelsError = $state('');
	let agentModelsLoading = $state(false);
	let agentSaving = $state(false);
	let agentSaveError = $state('');
	let agentPassphrase = $state('');
	let agentPassphraseNeeded = $state(false);
	let agentTesting = $state(false);
	let agentTestResult = $state<AgentTestResult | null>(null);

	function loadAgentSettings() {
		getAgentSettings()
			.then((s) => {
				agentSettings = s;
				agentBaseUrl = s.baseUrl;
				agentModel = s.model;
				agentThinking = s.thinking;
			})
			.catch(() => {});
	}
	loadAgentSettings();

	function loadAgentModels() {
		agentModelsLoading = true;
		agentModelsError = '';
		listAgentModels(agentBaseUrl || undefined)
			.then((r) => {
				agentModels = r.models.map((m) => m.id);
			})
			.catch((err: unknown) => {
				agentModels = [];
				agentModelsError = err instanceof Error ? err.message : 'Failed to load models';
			})
			.finally(() => {
				agentModelsLoading = false;
			});
	}

	async function saveAgent() {
		agentSaving = true;
		agentSaveError = '';
		try {
			const update = {
				baseUrl: agentBaseUrl,
				model: agentModel,
				thinking: agentThinking,
				...(agentApiKey ? { apiKey: agentApiKey } : {}),
				...(agentClearApiKey ? { clearApiKey: true } : {}),
			};
			agentSettings = await saveAgentSettings(update, agentPassphrase);
			agentApiKey = '';
			agentClearApiKey = false;
			agentPassphraseNeeded = false;
			agentTestResult = null;
		} catch (err) {
			const message = err instanceof Error ? err.message : 'Save failed';
			if (message.toLowerCase().includes('passphrase')) agentPassphraseNeeded = true;
			agentSaveError = message;
		} finally {
			agentSaving = false;
		}
	}

	function testAgentConnection() {
		agentTesting = true;
		agentTestResult = null;
		testAgent()
			.then((r) => (agentTestResult = r))
			.catch((err: unknown) => {
				agentTestResult = { ok: false, model: '', latencyMs: 0, toolCalls: false, error: err instanceof Error ? err.message : 'Test failed' };
			})
			.finally(() => {
				agentTesting = false;
			});
	}
</script>

<div class="flex items-center justify-between border-b border-zinc-200 px-4 py-3">
	<h2 class="text-sm font-semibold">My setup</h2>
	<Button variant="ghost" size="icon" ariaLabel="Close setup" on:click={onClose}><X size={16} /></Button>
</div>

<div class="min-h-0 flex-1 overflow-auto">
	<div class="flex flex-col gap-5 p-4">
		<div>
			<label class="text-sm font-medium text-zinc-700">
				Language
				<Select bind:value={$setup.lang} className="mt-1 max-w-xs">
					<option value="">English (default)</option>
					{#each version?.languages ?? [] as lang (lang.code)}
						<option value={lang.code}>{lang.label}</option>
					{/each}
				</Select>
			</label>
			<p class="mt-1 text-xs text-zinc-500">Shows the local-language name under results. Copying still uses the official English name.</p>
		</div>

		<div>
			{#if version?.commonCodes}
				<p class="text-sm text-zinc-700">
					Local names from: <span class="font-medium">{version.commonCodes.label}</span> ({version.commonCodes.count} codes) — ranked first
				</p>
				<label class="mt-2 flex items-center gap-2 text-sm text-zinc-700">
					<Checkbox checked={$setup.commonCodesOnly} on:change={() => ($setup.commonCodesOnly = !$setup.commonCodesOnly)} />
					Only show codes from this list
				</label>
			{:else}
				<p class="text-xs text-zinc-500">
					No local code list loaded. Your administrator can load one with <code class="rounded bg-zinc-100 px-1 py-0.5">LOINC_COMMON_CODES_CSV</code>.
				</p>
			{/if}
		</div>

		<label class="flex items-center gap-2 text-sm text-zinc-700">
			<Checkbox checked={$setup.orderableOnly} on:change={() => ($setup.orderableOnly = !$setup.orderableOnly)} />
			Orderable lab codes only (LOINC Universal Lab Orders)
		</label>

		<div>
			<label class="text-sm font-medium text-zinc-700">
				Default copy format
				<Select bind:value={$setup.copyFormat} className="mt-1 max-w-xs">
					{#each COPY_FORMATS as format (format.id)}
						<option value={format.id}>{format.label}</option>
					{/each}
				</Select>
			</label>
		</div>

		<div class="border-t border-zinc-200 pt-4">
			<h3 class="text-sm font-semibold text-zinc-950">AI assistant</h3>
			{#if agentSettings?.localOnly}
				<p class="mt-1 text-xs text-zinc-500">Only local/private endpoints allowed.</p>
			{/if}

			<div class="mt-3 flex flex-col gap-3">
				<Field label="Endpoint base URL">
					<Input bind:value={agentBaseUrl} placeholder="http://127.0.0.1:1234/v1" autocomplete="off" />
				</Field>

				<div class="flex items-end gap-2">
					<Field label="Model" className="flex-1">
						{#if agentModels.length}
							<Select bind:value={agentModel} className="mt-1">
								{#each agentModels as m (m)}
									<option value={m}>{m}</option>
								{/each}
							</Select>
						{:else}
							<Input bind:value={agentModel} placeholder="Model id" autocomplete="off" />
						{/if}
					</Field>
					<Button type="button" variant="outline" size="sm" on:click={loadAgentModels} disabled={agentModelsLoading}>
						{agentModelsLoading ? 'Loading...' : 'Load models'}
					</Button>
				</div>
				{#if agentModelsError}
					<p class="text-xs text-red-600">{agentModelsError}</p>
				{/if}

				<Field label="API key">
					<Input type="password" bind:value={agentApiKey} placeholder={agentSettings?.apiKeySet ? 'set (leave blank to keep)' : 'optional'} autocomplete="off" />
				</Field>
				{#if agentSettings?.apiKeySet}
					<label class="flex items-center gap-2 text-sm text-zinc-700">
						<Checkbox checked={agentClearApiKey} on:change={() => (agentClearApiKey = !agentClearApiKey)} />
						Clear saved API key
					</label>
				{/if}

				<label class="flex items-center gap-2 text-sm text-zinc-700">
					<Checkbox checked={agentThinking} on:change={() => (agentThinking = !agentThinking)} />
					Thinking (show model's reasoning by default)
				</label>

				{#if agentPassphraseNeeded}
					<Field label="Server passphrase">
						<Input type="password" bind:value={agentPassphrase} autocomplete="off" />
					</Field>
				{/if}

				{#if agentSaveError}
					<p class="text-xs text-red-600">{agentSaveError}</p>
				{/if}

				<div class="flex flex-wrap items-center gap-2">
					<Button type="button" size="sm" on:click={saveAgent} disabled={agentSaving}>
						{agentSaving ? 'Saving...' : 'Save'}
					</Button>
					<Button type="button" variant="outline" size="sm" on:click={testAgentConnection} disabled={agentTesting || !agentSettings?.configured}>
						{agentTesting ? 'Testing...' : 'Test connection'}
					</Button>
					{#if agentTestResult}
						{#if agentTestResult.ok}
							<span class="text-xs text-zinc-600">
								OK · {agentTestResult.model} · {agentTestResult.latencyMs}ms · tools {agentTestResult.toolCalls ? 'supported ✓' : 'unsupported ✗'}
							</span>
						{:else}
							<span class="text-xs text-red-600">{agentTestResult.error || 'Test failed'}</span>
						{/if}
					{/if}
				</div>
			</div>
		</div>
	</div>
</div>
