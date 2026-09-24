// Pure parsing/export helpers for the "map a whole test master" flow (MapList.svelte).
// Kept dependency-free and framework-free so they can run under plain node for maplist.check.ts.

export type Delimiter = ',' | '\t';

// detectDelimiter picks tab over comma when the first non-empty line has more tabs than
// commas (typical of a pasted spreadsheet range or a .tsv export).
export function detectDelimiter(text: string): Delimiter {
	const firstLine = text.split(/\r\n|\r|\n/).find((line) => line.trim() !== '') ?? '';
	const tabs = (firstLine.match(/\t/g) ?? []).length;
	const commas = (firstLine.match(/,/g) ?? []).length;
	return tabs > commas ? '\t' : ',';
}

// parseDelimited is an RFC-4180 parser: quoted fields may contain the delimiter, newlines,
// and "" as an escaped quote. Handles \r\n, \n, and a missing trailing newline.
export function parseDelimited(text: string, delimiter: Delimiter): string[][] {
	const rows: string[][] = [];
	let row: string[] = [];
	let field = '';
	let inQuotes = false;
	let sawAny = false;

	for (let i = 0; i < text.length; i++) {
		const c = text[i];
		if (inQuotes) {
			if (c === '"') {
				if (text[i + 1] === '"') {
					field += '"';
					i++;
				} else {
					inQuotes = false;
				}
			} else {
				field += c;
			}
			continue;
		}
		if (c === '"') {
			inQuotes = true;
			sawAny = true;
		} else if (c === delimiter) {
			row.push(field);
			field = '';
			sawAny = true;
		} else if (c === '\r') {
			continue;
		} else if (c === '\n') {
			row.push(field);
			rows.push(row);
			row = [];
			field = '';
			sawAny = true;
		} else {
			field += c;
			sawAny = true;
		}
	}
	if (field !== '' || row.length > 0) {
		row.push(field);
		rows.push(row);
	}
	if (!sawAny) return [];
	return rows;
}

// parseNameLines splits pasted "one name per line" text, trimming and dropping blank lines.
export function parseNameLines(text: string): string[] {
	return text
		.split(/\r\n|\r|\n/)
		.map((line) => line.trim())
		.filter((line) => line !== '');
}

export type ParsedTable = {
	header: string[] | null;
	rows: string[][];
};

// parseDelimitedTable parses a file's contents and optionally splits off a header row.
// Fully blank rows (every cell empty after trim) are dropped.
export function parseDelimitedTable(text: string, delimiter: Delimiter, hasHeader: boolean): ParsedTable {
	const allRows = parseDelimited(text, delimiter).filter((r) => r.some((cell) => cell.trim() !== ''));
	if (allRows.length === 0) return { header: null, rows: [] };
	if (hasHeader) {
		return { header: allRows[0], rows: allRows.slice(1) };
	}
	return { header: null, rows: allRows };
}

export function normalizeName(name: string): string {
	return name.trim().toLowerCase();
}

// dedupeNames returns unique names (first-occurrence casing, trimmed) in first-seen order,
// case-insensitively. The server-bound request list should use this, not the raw rows.
export function dedupeNames(names: string[]): string[] {
	const seen = new Set<string>();
	const unique: string[] = [];
	for (const raw of names) {
		const name = raw.trim();
		if (name === '') continue;
		const key = normalizeName(name);
		if (seen.has(key)) continue;
		seen.add(key);
		unique.push(name);
	}
	return unique;
}

// chunk splits an array into groups of at most `size`, preserving order. Used to send
// matchNames requests in batches (server caps at 1000 names/request).
export function chunk<T>(items: T[], size: number): T[][] {
	const out: T[][] = [];
	for (let i = 0; i < items.length; i += size) out.push(items.slice(i, i + size));
	return out;
}

// pending: needs a look and nobody picked yet; unmapped: no match, "None of these", or a rejected confident match.
export type MatchStatus = 'confident' | 'reviewed' | 'pending' | 'unmapped';

export type ResolvedMatch = {
	loincCode: string;
	loincLongCommonName: string;
	matchStatus: MatchStatus;
};

// buildExportRows joins each original row to its resolved LOINC match (looked up by
// normalized name) and appends loinc_code / loinc_long_common_name / match_status columns.
// `header` is null for plain pasted names, in which case the sole input column is named "name".
export function buildExportRows(
	header: string[] | null,
	rows: string[][],
	nameColumnIndex: number,
	resolve: (normalizedName: string) => ResolvedMatch | undefined,
): Record<string, string>[] {
	const columnNames = header ?? ['name'];
	return rows.map((row) => {
		const out: Record<string, string> = {};
		columnNames.forEach((col, i) => {
			out[col] = row[i] ?? '';
		});
		const name = row[nameColumnIndex] ?? '';
		const resolved = resolve(normalizeName(name));
		out.loinc_code = resolved?.loincCode ?? '';
		out.loinc_long_common_name = resolved?.loincLongCommonName ?? '';
		out.match_status = resolved?.matchStatus ?? 'unmapped';
		return out;
	});
}

export function exportColumns(header: string[] | null): string[] {
	return [...(header ?? ['name']), 'loinc_code', 'loinc_long_common_name', 'match_status'];
}
