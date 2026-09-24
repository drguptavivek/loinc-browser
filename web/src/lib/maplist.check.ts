import assert from 'node:assert';
import {
	buildExportRows,
	dedupeNames,
	detectDelimiter,
	exportColumns,
	parseDelimited,
	parseDelimitedTable,
	parseNameLines,
	chunk,
} from './maplist.ts';

// Delimiter detection: tabs win when more frequent than commas
assert.strictEqual(detectDelimiter('a\tb\tc'), '\t');
assert.strictEqual(detectDelimiter('a,b,c'), ',');
assert.strictEqual(detectDelimiter('plain name, with a comma'), ',');

// RFC-4180: quoted field with embedded comma and newline, escaped quote
assert.deepStrictEqual(
	parseDelimited('a,"b, with comma","line1\nline2",d\n', ','),
	[['a', 'b, with comma', 'line1\nline2', 'd']],
);
assert.deepStrictEqual(parseDelimited('x,"say ""hi""",y', ','), [['x', 'say "hi"', 'y']]);

// TSV, no trailing newline, CRLF line endings
assert.deepStrictEqual(parseDelimited('a\tb\r\nc\td', '\t'), [
	['a', 'b'],
	['c', 'd'],
]);

// parseNameLines: trims, drops blanks
assert.deepStrictEqual(parseNameLines('Sodium\n\n  HbA1c  \r\nTSH'), ['Sodium', 'HbA1c', 'TSH']);

// parseDelimitedTable: header split, blank rows dropped
{
	const table = parseDelimitedTable('Name,Code\nSodium,SOD\n,,\nHbA1c,A1C\n', ',', true);
	assert.deepStrictEqual(table.header, ['Name', 'Code']);
	assert.deepStrictEqual(table.rows, [
		['Sodium', 'SOD'],
		['HbA1c', 'A1C'],
	]);
}
{
	const table = parseDelimitedTable('Sodium\nHbA1c\n', ',', false);
	assert.strictEqual(table.header, null);
	assert.deepStrictEqual(table.rows, [['Sodium'], ['HbA1c']]);
}

// dedupeNames: case-insensitive, trimmed, first-occurrence order/casing kept
assert.deepStrictEqual(dedupeNames(['Sodium', ' sodium ', 'HbA1c', 'SODIUM', '']), ['Sodium', 'HbA1c']);

// chunk: batches of N, preserves order, handles remainder
assert.deepStrictEqual(chunk([1, 2, 3, 4, 5], 2), [[1, 2], [3, 4], [5]]);
assert.deepStrictEqual(chunk([], 200), []);

// buildExportRows + exportColumns: with header
{
	const header = ['Name', 'Code'];
	const rows = [
		['Sodium', 'SOD'],
		['Unknown Test', 'UNK'],
	];
	const resolve = (name: string) =>
		name === 'sodium'
			? { loincCode: '2951-2', loincLongCommonName: 'Sodium [Moles/volume] in Serum or Plasma', matchStatus: 'confident' as const }
			: undefined;
	const out = buildExportRows(header, rows, 0, resolve);
	assert.deepStrictEqual(out, [
		{ Name: 'Sodium', Code: 'SOD', loinc_code: '2951-2', loinc_long_common_name: 'Sodium [Moles/volume] in Serum or Plasma', match_status: 'confident' },
		{ Name: 'Unknown Test', Code: 'UNK', loinc_code: '', loinc_long_common_name: '', match_status: 'unmapped' },
	]);
	assert.deepStrictEqual(exportColumns(header), ['Name', 'Code', 'loinc_code', 'loinc_long_common_name', 'match_status']);
}

// buildExportRows: pasted names, no header -> single "name" column
{
	const out = buildExportRows(null, [['Sodium']], 0, () => undefined);
	assert.deepStrictEqual(out, [{ name: 'Sodium', loinc_code: '', loinc_long_common_name: '', match_status: 'unmapped' }]);
	assert.deepStrictEqual(exportColumns(null), ['name', 'loinc_code', 'loinc_long_common_name', 'match_status']);
}

console.log('maplist.check.ts: all assertions passed');
