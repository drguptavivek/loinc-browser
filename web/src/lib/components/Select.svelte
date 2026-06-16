<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import { cn } from '$lib/utils';

	export let value = '';
	export let disabled = false;
	export let className = '';

	const dispatch = createEventDispatcher<{ valueChange: string }>();

	function handleChange(event: Event) {
		value = (event.currentTarget as HTMLSelectElement).value;
		dispatch('valueChange', value);
	}
</script>

<select
	bind:value
	{disabled}
	class={cn(
		'mt-1 h-10 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm normal-case tracking-normal text-zinc-800 outline-none transition focus:border-zinc-400 focus:ring-2 focus:ring-zinc-100 disabled:cursor-not-allowed disabled:opacity-50',
		className,
	)}
	on:change={handleChange}
>
	<slot />
</select>
