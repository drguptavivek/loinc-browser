<script lang="ts">
	export let title = 'No results';
	export let body = 'Try another query or remove a filter.';
</script>

<div class="flex min-h-48 flex-col items-center justify-center rounded-lg border border-dashed border-zinc-200 bg-white p-8 text-center">
	<h3 class="text-sm font-semibold text-zinc-950">{title}</h3>
	<p class="mt-2 max-w-md text-sm text-zinc-500">{body}</p>
</div>
