<script lang="ts">
	import { GripVertical } from '@lucide/svelte';
	import { PaneResizer, type PaneResizerProps } from 'paneforge';
	import { cn } from '$lib/utils';

	let { class: className, withHandle = false, ...restProps }: PaneResizerProps & { withHandle?: boolean } = $props();
</script>

<PaneResizer
	class={cn(
		'relative flex w-px items-center justify-center bg-zinc-200 after:absolute after:inset-y-0 after:left-1/2 after:w-6 after:-translate-x-1/2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500 data-[direction=vertical]:h-px data-[direction=vertical]:w-full data-[direction=vertical]:after:left-0 data-[direction=vertical]:after:h-6 data-[direction=vertical]:after:w-full data-[direction=vertical]:after:-translate-y-1/2 data-[direction=vertical]:after:translate-x-0 [&[data-direction=vertical]>div]:rotate-90',
		className,
	)}
	{...restProps}
>
	{#if withHandle}
		<div class="z-10 flex h-5 w-4 items-center justify-center rounded-sm border border-zinc-200 bg-white shadow-sm">
			<GripVertical class="size-3 text-zinc-500" />
		</div>
	{/if}
</PaneResizer>
