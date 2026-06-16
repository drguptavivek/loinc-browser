<script lang="ts">
	import { cn } from '$lib/utils';

	export let checked = false;
	export let disabled = false;
	export let className = '';
</script>

<input
	type="checkbox"
	bind:checked
	{disabled}
	class={cn('size-4 rounded border-zinc-300 text-zinc-950 focus:ring-2 focus:ring-zinc-200 disabled:cursor-not-allowed disabled:opacity-50', className)}
	on:change
/>
