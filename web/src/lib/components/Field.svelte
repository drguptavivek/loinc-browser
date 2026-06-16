<script lang="ts">
	import { cn } from '$lib/utils';

	export let label: string;
	export let className = '';
	export let labelClassName = '';
</script>

<label class={cn('text-xs font-semibold uppercase tracking-wide text-zinc-500', className)}>
	<span class={labelClassName}>{label}</span>
	<slot />
</label>
