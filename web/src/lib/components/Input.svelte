<script lang="ts">
	import { cn } from '$lib/utils';

	export let type = 'text';
	export let value: string | number = '';
	export let placeholder = '';
	export let disabled = false;
	export let min: string | number | undefined = undefined;
	export let max: string | number | undefined = undefined;
	export let autocomplete: HTMLInputElement['autocomplete'] | undefined = undefined;
	export let ariaLabel: string | undefined = undefined;
	export let className = '';
</script>

<input
	{type}
	aria-label={ariaLabel}
	bind:value
	{placeholder}
	{disabled}
	{min}
	{max}
	{autocomplete}
	class={cn(
		'mt-1 h-10 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm normal-case tracking-normal text-zinc-800 outline-none transition focus:border-zinc-400 focus:ring-2 focus:ring-zinc-100 disabled:cursor-not-allowed disabled:opacity-50',
		className,
	)}
	on:input
	on:change
/>
