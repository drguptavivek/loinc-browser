<script lang="ts">
	import { XCircle } from '@lucide/svelte';

	export let label: string;
	export let value: string;
	export let onRemove: () => void;
	export let truncate = true;
</script>

<button
	type="button"
	class="inline-flex max-w-full items-center gap-1 rounded-md border border-zinc-200 bg-zinc-50 px-2 py-1 text-xs text-zinc-700 hover:bg-zinc-100"
	on:click={onRemove}
	aria-label={`Remove ${label} filter ${value}`}
>
	<span class="font-medium">{label}:</span>
	<span class={truncate ? 'max-w-48 truncate' : 'break-all text-left'}>{value}</span>
	<XCircle size={13} class="shrink-0 text-zinc-400" />
</button>
