<script lang="ts">
	import { Download, Trash2, X } from '@lucide/svelte';
	import Button from '$lib/components/Button.svelte';
	import EmptyState from '$lib/components/EmptyState.svelte';
	import { basket, basketCSV, clearBasket, removeFromBasket, setLocalName } from '$lib/basket';
	import { copyText, downloadText } from '$lib/copy';

	let { onClose, onOpen }: { onClose: () => void; onOpen: (n: string) => void } = $props();

	let confirmingClear = $state(false);
	let copied = $state(false);
	let copiedTimer: ReturnType<typeof setTimeout> | undefined;

	async function copyAllCodes() {
		const ok = await copyText($basket.map((i) => i.loincNum).join('\n'));
		if (!ok) return;
		copied = true;
		clearTimeout(copiedTimer);
		copiedTimer = setTimeout(() => {
			copied = false;
		}, 1500);
	}

	function exportCSV() {
		downloadText('loinc-basket.csv', basketCSV($basket));
	}

	function confirmClear() {
		clearBasket();
		confirmingClear = false;
	}
</script>

<div class="flex items-center justify-between border-b border-zinc-200 px-4 py-3">
	<h2 class="text-sm font-semibold">Basket ({$basket.length})</h2>
	<Button variant="ghost" size="icon" ariaLabel="Close basket" on:click={onClose}><X size={16} /></Button>
</div>

<div class="min-h-0 flex-1 overflow-auto">
	{#if !$basket.length}
		<div class="p-4"><EmptyState title="Your basket is empty" body="Add terms with the + button while searching." /></div>
	{:else}
		<div class="flex flex-col gap-3 p-4">
			<div class="flex flex-wrap items-center gap-2">
				<Button variant="outline" size="sm" on:click={copyAllCodes}>{copied ? 'Copied' : 'Copy all codes'}</Button>
				<Button variant="outline" size="sm" on:click={exportCSV}><Download size={14} />Export CSV</Button>
				{#if confirmingClear}
					<span class="text-xs text-zinc-600">Clear all?</span>
					<Button variant="outline" size="sm" on:click={confirmClear}>Yes</Button>
					<Button variant="ghost" size="sm" on:click={() => (confirmingClear = false)}>No</Button>
				{:else}
					<Button variant="ghost" size="sm" on:click={() => (confirmingClear = true)}><Trash2 size={14} />Clear</Button>
				{/if}
			</div>

			<div class="flex flex-col gap-2">
				{#each $basket as item (item.loincNum)}
					<div class="rounded-md border border-zinc-200 p-3">
						<div class="flex items-start justify-between gap-2">
							<button type="button" class="min-w-0 text-left" onclick={() => onOpen(item.loincNum)}>
								<div class="font-mono text-sm font-semibold hover:underline">{item.loincNum}</div>
								<div class="mt-0.5 break-words text-sm text-zinc-700">{item.longCommonName}</div>
								{#if item.exampleUcum}<div class="mt-0.5 text-xs text-zinc-500">Units: {item.exampleUcum}</div>{/if}
							</button>
							<Button variant="ghost" size="icon" ariaLabel={`Remove ${item.loincNum} from basket`} on:click={() => removeFromBasket(item.loincNum)}>
								<X size={14} />
							</Button>
						</div>
						<label class="mt-2 block text-xs text-zinc-500">
							Your local name
							<input
								type="text"
								class="mt-1 block w-full rounded-md border border-zinc-200 px-2 py-1 text-sm text-zinc-900 focus:outline-none focus:ring-2 focus:ring-zinc-400"
								value={item.localName ?? ''}
								oninput={(e) => setLocalName(item.loincNum, e.currentTarget.value)}
							/>
						</label>
					</div>
				{/each}
			</div>
		</div>
	{/if}
</div>
