<script lang="ts">
	export let label: string;
	export let value = '';
</script>

<div class="rounded-md border border-zinc-200 p-3">
	<div class="text-xs font-medium uppercase tracking-wide text-zinc-500">{label}</div>
	<div class="mt-1 min-h-5 break-words text-zinc-900">{value || '-'}</div>
</div>
