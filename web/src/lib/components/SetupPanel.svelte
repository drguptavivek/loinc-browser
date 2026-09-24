<script lang="ts">
	import { X } from '@lucide/svelte';
	import Button from '$lib/components/Button.svelte';
	import Checkbox from '$lib/components/Checkbox.svelte';
	import Select from '$lib/components/Select.svelte';
	import { getVersion, type VersionInfo } from '$lib/api';
	import { COPY_FORMATS } from '$lib/copy';
	import { setup } from '$lib/setup';

	let { onClose }: { onClose: () => void } = $props();

	let version = $state<VersionInfo | null>(null);
	getVersion()
		.then((v) => (version = v))
		.catch(() => {});
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
	</div>
</div>
