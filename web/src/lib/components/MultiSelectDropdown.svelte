<script lang="ts">
	import { onMount } from 'svelte';
	import { ChevronDown, X } from '@lucide/svelte';
	import Badge from '$lib/components/Badge.svelte';
	import Button from '$lib/components/Button.svelte';

	export let label: string;
	export let options: [string, number][] = [];
	export let selected: string[] = [];
	export let emptyLabel = 'Any';
	export let onToggle: (value: string) => void;
	export let onClear: () => void;

	let open = false;
	let root: HTMLDivElement;

	$: visibleOptions = options.slice(0, 60);
	$: summary = selected.length === 0 ? emptyLabel : selected.length === 1 ? selected[0] : `${selected.length} selected`;

	onMount(() => {
		const closeOnOutsidePointer = (event: PointerEvent) => {
			if (open && root && event.target instanceof Node && !root.contains(event.target)) {
				open = false;
			}
		};
		document.addEventListener('pointerdown', closeOnOutsidePointer, true);
		return () => document.removeEventListener('pointerdown', closeOnOutsidePointer, true);
	});

	function isSelected(value: string) {
		return selected.includes(value);
	}
</script>

<div class="relative" bind:this={root}>
	<button
		type="button"
		class="flex h-8 min-w-44 items-center justify-between gap-2 rounded-md border border-zinc-200 bg-white px-2.5 text-left text-[11px] leading-4 text-zinc-700 hover:bg-zinc-50"
		aria-expanded={open}
		on:click={() => (open = !open)}
	>
		<span class="flex min-w-0 items-center gap-1.5">
			<span class="shrink-0 text-[10px] font-semibold uppercase tracking-wide text-zinc-500">{label}</span>
			<span class="min-w-0 truncate text-zinc-800">{summary}</span>
		</span>
		<ChevronDown size={14} class="shrink-0 text-zinc-400" />
	</button>

	{#if open}
		<div class="absolute left-0 top-10 z-30 w-64 rounded-md border border-zinc-200 bg-white p-2 shadow-lg">
			<div class="mb-2 flex items-center justify-between gap-2 border-b border-zinc-100 pb-2">
				<div class="text-xs font-semibold text-zinc-700">{label}</div>
				{#if selected.length > 0}
					<Button variant="ghost" size="sm" on:click={onClear}>
						<X size={13} />
						Clear
					</Button>
				{/if}
			</div>
			<div class="max-h-64 overflow-auto">
				{#each visibleOptions as [value, count]}
					<label class="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-xs text-zinc-700 hover:bg-zinc-50">
						<input
							type="checkbox"
							class="size-3.5 rounded border-zinc-300"
							checked={isSelected(value)}
							on:change={() => onToggle(value)}
						/>
						<span class="min-w-0 flex-1 truncate">{value}</span>
						<Badge variant="secondary">{count.toLocaleString()}</Badge>
					</label>
				{/each}
			</div>
		</div>
	{/if}
</div>
