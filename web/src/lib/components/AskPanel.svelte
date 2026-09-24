<script lang="ts">
	import { untrack } from 'svelte';
	import { Loader2, Search, Send, Settings, Square, X } from '@lucide/svelte';
	import Badge from '$lib/components/Badge.svelte';
	import Button from '$lib/components/Button.svelte';
	import EmptyState from '$lib/components/EmptyState.svelte';
	import { addToBasket } from '$lib/basket';
	import { getAgentSettings, streamAgentChat, type AgentChatMessage, type AgentEvent } from '$lib/api';

	let { onOpen, onClose, onOpenSetup, prefill = '' }: {
		onOpen: (loincNum: string) => void;
		onClose: () => void;
		onOpenSetup: () => void;
		// pre-filled and auto-sent when opened from a Find Mode suggestion
		prefill?: string;
	} = $props();

	type ToolActivity = { id: string; name: string; done: boolean; ok?: boolean; detail?: string };
	type AskCode = { loincNum: string; longCommonName: string; status: string };
	type ChatTurn = {
		role: 'user' | 'assistant';
		content: string;
		thinking?: string;
		tools?: ToolActivity[];
		codes?: AskCode[];
		unverified?: string[];
		done?: { rounds: number; model: string; elapsedMs: number };
		error?: string;
	};

	const STORAGE_KEY = 'loinc.ask.v1';
	const MAX_HISTORY = 20;

	// Friendly verbs for tool activity lines; falls back to the raw name for anything unlisted.
	// Keys are MCP tool names without the "loinc_" prefix.
	const TOOL_VERBS: Record<string, string> = {
		search_terms: 'Searched LOINC terms',
		search_panels: 'Searched LOINC panels',
		get_term: 'Looked up',
		get_term_fit: 'Checked fit of',
		get_term_relationships: 'Checked related terms of',
		get_panel_items: 'Listed panel items of',
		match_names: 'Matched local test names',
		explain_concepts: 'Read LOINC guidance',
	};

	function toolLabel(tool: ToolActivity): string {
		const key = tool.name.replace(/^loinc_/, '');
		const verb = TOOL_VERBS[key] ?? `Ran ${tool.name}`;
		return tool.detail ? `${verb} “${tool.detail}”` : verb;
	}

	// The most telling argument of a tool call (query or code), shown so users see what was searched.
	function toolDetail(args: unknown): string | undefined {
		let value: unknown = args;
		if (typeof value === 'string') {
			try {
				value = JSON.parse(value);
			} catch {
				return undefined;
			}
		}
		if (!value || typeof value !== 'object') return undefined;
		const a = value as Record<string, unknown>;
		const pick = a.query ?? a.q ?? a.loincNum ?? a.loinc_num ?? a.code ?? a.topic;
		return typeof pick === 'string' && pick.trim() ? pick.trim().slice(0, 80) : undefined;
	}

	function loadTurns(): ChatTurn[] {
		try {
			const raw = sessionStorage.getItem(STORAGE_KEY);
			if (!raw) return [];
			const parsed = JSON.parse(raw);
			return Array.isArray(parsed) ? parsed : [];
		} catch {
			return [];
		}
	}

	function saveTurns(value: ChatTurn[]): void {
		try {
			sessionStorage.setItem(STORAGE_KEY, JSON.stringify(value));
		} catch {
			// ignore quota/availability errors
		}
	}

	let turns = $state<ChatTurn[]>(loadTurns());
	// prefill is a one-time seed (App remounts the Ask drawer, not this field, on reopen);
	// intentionally capture only the initial value here, like FindMode's initialQuery.
	let input = $state(untrack(() => prefill));
	let sending = $state(false);
	let configured = $state<boolean | null>(null);
	let modelName = $state('');
	let thinkingOn = $state(false);
	let sendError = $state('');
	let abortController: AbortController | undefined;
	let listEl: HTMLElement | undefined = $state();

	getAgentSettings()
		.then((s) => {
			configured = s.configured;
			modelName = s.model;
			thinkingOn = s.thinking;
			if (prefill.trim()) send();
		})
		.catch(() => {
			configured = false;
		});

	function persist() {
		saveTurns(turns);
	}

	function scrollToEnd() {
		requestAnimationFrame(() => listEl?.scrollTo({ top: listEl.scrollHeight }));
	}

	function newChat() {
		turns = [];
		persist();
	}

	function stop() {
		abortController?.abort();
	}

	async function send() {
		const text = input.trim();
		if (!text || sending) return;
		input = '';
		sendError = '';
		turns = [...turns, { role: 'user', content: text }, { role: 'assistant', content: '' }];
		persist();
		scrollToEnd();
		sending = true;
		abortController = new AbortController();

		const history: AgentChatMessage[] = turns
			.slice(0, -1)
			.slice(-MAX_HISTORY)
			.map((t) => ({ role: t.role, content: t.content }));

		function updateLast(patch: Partial<ChatTurn>) {
			turns = turns.map((t, i) => (i === turns.length - 1 ? { ...t, ...patch } : t));
		}

		try {
			await streamAgentChat({ messages: history, thinking: thinkingOn }, (event: AgentEvent) => {
				const last = turns[turns.length - 1];
				switch (event.type) {
					case 'thinking':
						updateLast({ thinking: (last.thinking ?? '') + event.text });
						break;
					case 'text':
						updateLast({ content: last.content + event.text });
						break;
					case 'tool-start':
						updateLast({ tools: [...(last.tools ?? []), { id: event.id, name: event.name, done: false, detail: toolDetail(event.args) }] });
						break;
					case 'tool-end':
						updateLast({
							tools: (last.tools ?? []).map((t) => (t.id === event.id ? { ...t, done: true, ok: event.ok } : t)),
						});
						break;
					case 'codes':
						updateLast({ codes: event.codes });
						break;
					case 'unverified':
						updateLast({ unverified: event.codes });
						break;
					case 'done':
						updateLast({ done: { rounds: event.rounds, model: event.model, elapsedMs: event.elapsedMs } });
						break;
					case 'error':
						updateLast({ error: event.message });
						break;
				}
				scrollToEnd();
			}, abortController.signal);
		} catch (err) {
			if (err instanceof Error && err.name === 'AbortError') {
				// stopped by the user; keep whatever streamed so far
			} else {
				sendError = err instanceof Error ? err.message : 'Ask failed';
			}
		} finally {
			sending = false;
			abortController = undefined;
			persist();
		}
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter' && !event.shiftKey) {
			event.preventDefault();
			send();
		}
	}

	// Splits assistant text on LOINC-shaped codes (\d{1,7}-\d) and marks each occurrence as
	// verified (clickable, from `codes`) or unverified (warning style) so the template can
	// render chips inline instead of plain text there.
	const CODE_PATTERN = /\d{1,7}-\d/g;

	function renderParts(turn: ChatTurn): { text: string; code?: string; verified?: boolean }[] {
		const verifiedSet = new Set((turn.codes ?? []).map((c) => c.loincNum));
		const unverifiedSet = new Set(turn.unverified ?? []);
		const parts: { text: string; code?: string; verified?: boolean }[] = [];
		let lastIndex = 0;
		for (const match of turn.content.matchAll(CODE_PATTERN)) {
			const code = match[0];
			if (!verifiedSet.has(code) && !unverifiedSet.has(code)) continue;
			if (match.index! > lastIndex) parts.push({ text: turn.content.slice(lastIndex, match.index) });
			parts.push({ text: code, code, verified: verifiedSet.has(code) });
			lastIndex = match.index! + code.length;
		}
		if (lastIndex < turn.content.length) parts.push({ text: turn.content.slice(lastIndex) });
		// Models write markdown; show it as plain text without the ** / __ emphasis markers.
		return parts.map((p) => (p.code ? p : { ...p, text: p.text.replace(/\*\*|__/g, '') }));
	}

	function openCode(loincNum: string) {
		onOpen(loincNum);
	}
</script>

<div class="flex items-center justify-between border-b border-zinc-200 px-4 py-3">
	<div class="flex items-center gap-2">
		<h2 class="text-sm font-semibold">Ask AI</h2>
		{#if modelName}<span class="text-xs text-zinc-500">{modelName}</span>{/if}
	</div>
	<div class="flex items-center gap-2">
		<label class="flex items-center gap-1.5 text-xs text-zinc-600">
			<input type="checkbox" bind:checked={thinkingOn} class="size-3.5 rounded border-zinc-300" />
			Thinking
		</label>
		<Button variant="outline" size="sm" on:click={newChat}>New chat</Button>
		<Button variant="ghost" size="icon" ariaLabel="Close Ask AI" on:click={onClose}><X size={16} /></Button>
	</div>
</div>

<div class="min-h-0 flex-1 overflow-auto p-4" bind:this={listEl}>
	{#if configured === false}
		<EmptyState title="AI assistant not set up" body="Connect an OpenAI-compatible endpoint to ask questions in plain language." />
		<div class="mt-3 flex justify-center">
			<Button size="sm" on:click={onOpenSetup}><Settings size={14} />Set up AI</Button>
		</div>
	{:else if !turns.length}
		<EmptyState title="Ask about a test" body="Describe a test in plain words, e.g. “fasting blood sugar venous plasma mg/dL”." />
	{:else}
		<div class="flex flex-col gap-3">
			{#each turns as turn, i (i)}
				{#if turn.role === 'user'}
					<div class="ml-auto max-w-[85%] rounded-lg bg-zinc-950 px-3 py-2 text-sm text-white">{turn.content}</div>
				{:else}
					<div class="flex max-w-[95%] flex-col gap-2 rounded-lg border border-zinc-200 bg-white px-3 py-2.5">
						{#if turn.thinking}
							<details class="text-xs text-zinc-500">
								<summary class="cursor-pointer select-none">Thinking…</summary>
								<p class="mt-1 whitespace-pre-wrap">{turn.thinking}</p>
							</details>
						{/if}
						{#each turn.tools ?? [] as tool (tool.id)}
							<div class="flex items-center gap-1.5 text-xs text-zinc-500">
								{#if !tool.done}<Loader2 size={12} class="animate-spin" />{/if}
								<Search size={12} />
								{toolLabel(tool)}
							</div>
						{/each}
						{#if turn.content}
							<p class="whitespace-pre-wrap text-sm text-zinc-800">
								{#each renderParts(turn) as part}
									{#if part.code}
										<button
											type="button"
											class={`mx-0.5 rounded border px-1 font-mono text-xs ${part.verified ? 'border-zinc-300 bg-zinc-50 text-zinc-700 hover:bg-zinc-100' : 'border-amber-300 bg-amber-50 text-amber-800'}`}
											title={part.verified ? undefined : 'not verified in LOINC data'}
											onclick={() => part.code && openCode(part.code)}
										>
											{part.text}
										</button>
									{:else}
										{part.text}
									{/if}
								{/each}
							</p>
						{:else if sending && i === turns.length - 1}
							<p class="text-sm text-zinc-400">…</p>
						{/if}
						{#if turn.error}
							<p class="text-xs text-red-600">{turn.error}</p>
						{/if}
						{#if turn.codes?.length}
							<div class="flex flex-col gap-1.5">
								{#each turn.codes as c (c.loincNum)}
									<div class="flex items-center justify-between gap-2 rounded-md border border-zinc-200 bg-zinc-50 px-2.5 py-1.5">
										<div class="min-w-0 text-sm">
											<span class="font-mono text-xs text-zinc-500">{c.loincNum}</span>
											<span class="ml-1.5 text-zinc-800">{c.longCommonName}</span>
											{#if c.status && c.status !== 'ACTIVE'}<Badge variant="warning" className="ml-1.5">{c.status}</Badge>{/if}
										</div>
										<div class="flex shrink-0 items-center gap-1">
											<Button variant="outline" size="sm" on:click={() => openCode(c.loincNum)}>Open</Button>
											<Button
												variant="ghost"
												size="sm"
												on:click={() => addToBasket({ loincNum: c.loincNum, longCommonName: c.longCommonName })}
											>
												+ Basket
											</Button>
										</div>
									</div>
								{/each}
							</div>
						{/if}
						{#if turn.done}
							<p class="text-[11px] text-zinc-400">
								{turn.done.model} · {turn.done.rounds} round{turn.done.rounds === 1 ? '' : 's'} · {(turn.done.elapsedMs / 1000).toFixed(1)}s
							</p>
						{/if}
					</div>
				{/if}
			{/each}
		</div>
	{/if}
</div>

{#if sendError}
	<p class="border-t border-zinc-200 px-4 py-2 text-xs text-red-600">{sendError}</p>
{/if}

{#if configured !== false}
	<div class="flex items-end gap-2 border-t border-zinc-200 p-3">
		<textarea
			bind:value={input}
			onkeydown={handleKeydown}
			disabled={sending}
			placeholder="Describe the test..."
			rows={2}
			class="min-h-[2.5rem] flex-1 resize-none rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-800 outline-none focus:border-zinc-400 focus:ring-2 focus:ring-zinc-100 disabled:opacity-50"
		></textarea>
		{#if sending}
			<Button variant="outline" on:click={stop}><Square size={14} />Stop</Button>
		{:else}
			<Button on:click={send} disabled={!input.trim()}><Send size={14} />Send</Button>
		{/if}
	</div>
{/if}
