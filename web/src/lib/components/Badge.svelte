<script lang="ts">
	import { cn } from '$lib/utils';

	export let variant: 'default' | 'secondary' | 'outline' | 'warning' = 'default';
	export let className = '';

	const variants = {
		default: 'border-transparent bg-zinc-950 text-white',
		secondary: 'border-transparent bg-zinc-100 text-zinc-800',
		outline: 'border-zinc-200 bg-white text-zinc-700',
		warning: 'border-amber-200 bg-amber-50 text-amber-900',
	};
</script>

<span class={cn('inline-flex items-center rounded-md border px-2 py-0.5 text-xs font-medium', variants[variant], className)}>
	<slot />
</span>
