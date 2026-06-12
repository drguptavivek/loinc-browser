<script lang="ts">
	import { cn } from '$lib/utils';

	export let type: 'button' | 'submit' = 'button';
	export let variant: 'default' | 'outline' | 'ghost' = 'default';
	export let size: 'default' | 'sm' | 'icon' = 'default';
	export let disabled = false;
	export let className = '';
	export let ariaLabel: string | undefined = undefined;

	const variants = {
		default: 'bg-zinc-950 text-white shadow-sm hover:bg-zinc-800',
		outline: 'border border-zinc-200 bg-white text-zinc-900 hover:bg-zinc-50',
		ghost: 'text-zinc-700 hover:bg-zinc-100 hover:text-zinc-950',
	};

	const sizes = {
		default: 'h-9 px-4',
		sm: 'h-8 px-3 text-xs',
		icon: 'size-8 p-0',
	};
</script>

<button
	{type}
	{disabled}
	aria-label={ariaLabel}
	class={cn(
		'inline-flex shrink-0 items-center justify-center gap-2 rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-400 disabled:pointer-events-none disabled:opacity-50',
		variants[variant],
		sizes[size],
		className,
	)}
	on:click
>
	<slot />
</button>
