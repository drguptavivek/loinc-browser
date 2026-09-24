import assert from 'node:assert';
import { parseSSEChunk } from './sse.ts';

// One complete event in one chunk
{
	const { events, rest } = parseSSEChunk('event: text\ndata: hello\n\n');
	assert.deepStrictEqual(events, [{ event: 'text', data: 'hello' }]);
	assert.strictEqual(rest, '');
}

// Multi-line data joined with \n
{
	const { events } = parseSSEChunk('event: thinking\ndata: line1\ndata: line2\n\n');
	assert.deepStrictEqual(events, [{ event: 'thinking', data: 'line1\nline2' }]);
}

// A frame split across two chunks: partial frame is returned as rest and must be re-fed
{
	const first = parseSSEChunk('event: tool-start\ndata: {"id":1}\n\nevent: tex');
	assert.deepStrictEqual(first.events, [{ event: 'tool-start', data: '{"id":1}' }]);
	assert.strictEqual(first.rest, 'event: tex');
	const second = parseSSEChunk(`${first.rest}t\ndata: {"text":"hi"}\n\n`);
	assert.deepStrictEqual(second.events, [{ event: 'text', data: '{"text":"hi"}' }]);
}

// CRLF line endings
{
	const { events } = parseSSEChunk('event: done\r\ndata: {}\r\n\r\n');
	assert.deepStrictEqual(events, [{ event: 'done', data: '{}' }]);
}

// Default event type when omitted
{
	const { events } = parseSSEChunk('data: no-type\n\n');
	assert.deepStrictEqual(events, [{ event: 'message', data: 'no-type' }]);
}

console.log('sse.check.ts: all assertions passed');
