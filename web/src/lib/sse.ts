// Minimal text/event-stream parser for a POST-based SSE endpoint (fetch + ReadableStream,
// since EventSource can't send a POST body). The server writes framed events separated by a
// blank line: "event: <type>\n" + one or more "data: <line>\n" (joined with '\n' if multiple).

export type SSEEvent = { event: string; data: string };

// parseSSEChunk splits a growing text buffer into complete events plus the trailing partial
// frame (which the caller re-feeds on the next chunk, since a network chunk can split a frame
// anywhere, including mid-line). Frames use \n or \r\n line endings.
export function parseSSEChunk(buffer: string): { events: SSEEvent[]; rest: string } {
	const events: SSEEvent[] = [];
	const frames = buffer.split(/\r?\n\r?\n/);
	// The last element is either '' (buffer ended on a blank line) or an incomplete frame.
	const rest = frames.pop() ?? '';
	for (const frame of frames) {
		if (!frame.trim()) continue;
		let eventType = 'message';
		const dataLines: string[] = [];
		for (const line of frame.split(/\r?\n/)) {
			if (line.startsWith('event:')) eventType = line.slice(6).trim();
			else if (line.startsWith('data:')) dataLines.push(line.slice(5).trim());
		}
		if (dataLines.length) events.push({ event: eventType, data: dataLines.join('\n') });
	}
	return { events, rest };
}
