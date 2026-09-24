// Minimal .xlsx (OOXML) reader for the "Map a list" file input, with zero npm dependencies.
// Parses the ZIP container by hand (central directory -> local headers), inflates DEFLATE
// entries with the platform DecompressionStream, and pulls text out of the handful of XML
// parts a spreadsheet needs (workbook.xml, its rels, sharedStrings.xml, the first sheet) with
// small regexes instead of a DOM parser, so this also runs under plain node for xlsx.check.ts.
//
// ponytail: only the first worksheet is read (matches what MapList needs); numeric cells keep
// their raw <v> text rather than being date/number-formatted, and ZIP64/encrypted archives are
// rejected outright. Add real formatting or multi-sheet support only if a user actually needs it.

type ZipEntry = {
	method: number;
	compressedSize: number;
	localHeaderOffset: number;
};

const EOCD_SIGNATURE = 0x06054b50;
const CENTRAL_DIR_SIGNATURE = 0x02014b50;
const LOCAL_HEADER_SIGNATURE = 0x04034b50;

function findEndOfCentralDirectory(bytes: Uint8Array): number {
	const minLen = 22;
	const maxCommentLen = 65535;
	const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
	const start = Math.max(0, bytes.length - minLen - maxCommentLen);
	for (let i = bytes.length - minLen; i >= start; i--) {
		if (view.getUint32(i, true) === EOCD_SIGNATURE) return i;
	}
	throw new Error('Not a valid .xlsx file (zip end-of-central-directory not found).');
}

function parseCentralDirectory(bytes: Uint8Array): Map<string, ZipEntry> {
	const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
	const eocdOffset = findEndOfCentralDirectory(bytes);
	const cdOffset = view.getUint32(eocdOffset + 16, true);
	const cdCount = view.getUint16(eocdOffset + 10, true);
	if (cdOffset === 0xffffffff || cdCount === 0xffff) {
		throw new Error('ZIP64 .xlsx files are not supported.');
	}
	const entries = new Map<string, ZipEntry>();
	let offset = cdOffset;
	for (let i = 0; i < cdCount; i++) {
		if (view.getUint32(offset, true) !== CENTRAL_DIR_SIGNATURE) {
			throw new Error('Invalid .xlsx file (bad central directory entry).');
		}
		const flags = view.getUint16(offset + 8, true);
		if (flags & 0x1) throw new Error('Encrypted .xlsx files are not supported.');
		const method = view.getUint16(offset + 10, true);
		const compressedSize = view.getUint32(offset + 20, true);
		const uncompressedSize = view.getUint32(offset + 24, true);
		const nameLen = view.getUint16(offset + 28, true);
		const extraLen = view.getUint16(offset + 30, true);
		const commentLen = view.getUint16(offset + 32, true);
		const localHeaderOffset = view.getUint32(offset + 42, true);
		if (compressedSize === 0xffffffff || uncompressedSize === 0xffffffff || localHeaderOffset === 0xffffffff) {
			throw new Error('ZIP64 .xlsx files are not supported.');
		}
		const name = new TextDecoder('utf-8').decode(bytes.subarray(offset + 46, offset + 46 + nameLen));
		entries.set(name, { method, compressedSize, localHeaderOffset });
		offset += 46 + nameLen + extraLen + commentLen;
	}
	return entries;
}

async function inflateRaw(data: Uint8Array): Promise<Uint8Array> {
	const copy = data.slice(); // detach from the shared ArrayBufferLike view so Blob's typing is satisfied
	const stream = new Blob([copy]).stream().pipeThrough(new DecompressionStream('deflate-raw'));
	return new Uint8Array(await new Response(stream).arrayBuffer());
}

async function readEntry(bytes: Uint8Array, entry: ZipEntry): Promise<Uint8Array> {
	const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
	const off = entry.localHeaderOffset;
	if (view.getUint32(off, true) !== LOCAL_HEADER_SIGNATURE) {
		throw new Error('Invalid .xlsx file (bad local file header).');
	}
	const nameLen = view.getUint16(off + 26, true);
	const extraLen = view.getUint16(off + 28, true);
	const dataStart = off + 30 + nameLen + extraLen;
	const compressed = bytes.subarray(dataStart, dataStart + entry.compressedSize);
	if (entry.method === 0) return compressed;
	if (entry.method === 8) return inflateRaw(compressed);
	throw new Error(`Unsupported compression method (${entry.method}) in .xlsx file.`);
}

async function readEntryText(bytes: Uint8Array, entries: Map<string, ZipEntry>, name: string): Promise<string | null> {
	const entry = entries.get(name);
	if (!entry) return null;
	return new TextDecoder('utf-8').decode(await readEntry(bytes, entry));
}

// decodeXmlEntities handles the five predefined XML entities plus numeric character references,
// in a single pass so an entity that itself contains "&" (e.g. a numeric ref) is never re-decoded.
function decodeXmlEntities(text: string): string {
	return text.replace(/&(#x[0-9a-fA-F]+|#\d+|amp|lt|gt|quot|apos);/g, (whole, ent: string) => {
		switch (ent) {
			case 'amp':
				return '&';
			case 'lt':
				return '<';
			case 'gt':
				return '>';
			case 'quot':
				return '"';
			case 'apos':
				return "'";
			default: {
				const code = ent[1] === 'x' || ent[1] === 'X' ? parseInt(ent.slice(2), 16) : parseInt(ent.slice(1), 10);
				return Number.isFinite(code) ? String.fromCodePoint(code) : whole;
			}
		}
	});
}

function extractRunText(xml: string): string {
	let text = '';
	const tRegex = /<t\b[^>]*>([\s\S]*?)<\/t>/g;
	let m: RegExpExecArray | null;
	while ((m = tRegex.exec(xml))) text += decodeXmlEntities(m[1]);
	return text;
}

// parseSharedStrings pulls the concatenated text of each <si> entry (a plain <t> or one/more
// rich-text <r><t> runs) in file order, matching how sharedStrings indices ("t=\"s\"" cells)
// reference this array.
function parseSharedStrings(xml: string): string[] {
	const result: string[] = [];
	const siRegex = /<si\b[^>]*>([\s\S]*?)<\/si>/g;
	let m: RegExpExecArray | null;
	while ((m = siRegex.exec(xml))) result.push(extractRunText(m[1]));
	return result;
}

function columnLetterToIndex(letters: string): number {
	let index = 0;
	for (const ch of letters) index = index * 26 + (ch.charCodeAt(0) - 64);
	return index - 1;
}

function parseCellRef(ref: string): { col: number; row: number } | null {
	const m = /^([A-Z]+)(\d+)$/.exec(ref);
	if (!m) return null;
	return { col: columnLetterToIndex(m[1]), row: parseInt(m[2], 10) - 1 };
}

// parseSheetXml reads <row>/<c> cells by their "r" address (falling back to sequential order
// when absent), resolves shared/inline/number/boolean/error cell types, fills gaps left by
// skipped columns with '', and trims fully-empty trailing rows.
function parseSheetXml(xml: string, sharedStrings: string[]): string[][] {
	const grid: string[][] = [];
	const rowRegex = /<row\b([^>]*)\/>|<row\b([^>]*)>([\s\S]*?)<\/row>/g;
	let rowFallback = 0;
	let rm: RegExpExecArray | null;
	while ((rm = rowRegex.exec(xml))) {
		const attrs = rm[1] ?? rm[2] ?? '';
		const inner = rm[3] ?? '';
		const rAttr = /\br="(\d+)"/.exec(attrs);
		const rowIndex = rAttr ? parseInt(rAttr[1], 10) - 1 : rowFallback;
		rowFallback = rowIndex + 1;
		if (!grid[rowIndex]) grid[rowIndex] = [];
		const cellRegex = /<c\b([^>]*)\/>|<c\b([^>]*)>([\s\S]*?)<\/c>/g;
		let colFallback = 0;
		let cm: RegExpExecArray | null;
		while ((cm = cellRegex.exec(inner))) {
			const selfClosing = cm[1] !== undefined;
			const cellAttrs = cm[1] ?? cm[2] ?? '';
			const cellInner = cm[3] ?? '';
			const refAttr = /\br="([A-Z]+\d+)"/.exec(cellAttrs);
			const ref = refAttr ? parseCellRef(refAttr[1]) : null;
			const colIndex = ref ? ref.col : colFallback;
			colFallback = colIndex + 1;
			grid[rowIndex][colIndex] = selfClosing ? '' : readCellValue(cellAttrs, cellInner, sharedStrings);
		}
	}
	const maxCols = grid.reduce((max, row) => Math.max(max, row ? row.length : 0), 0);
	const table: string[][] = [];
	for (let r = 0; r < grid.length; r++) {
		const row = grid[r] ?? [];
		const out: string[] = [];
		for (let c = 0; c < maxCols; c++) out.push(row[c] ?? '');
		table.push(out);
	}
	while (table.length && table[table.length - 1].every((cell) => cell === '')) table.pop();
	return table;
}

function readCellValue(cellAttrs: string, cellInner: string, sharedStrings: string[]): string {
	const typeAttr = /\bt="([a-zA-Z]+)"/.exec(cellAttrs);
	const type = typeAttr ? typeAttr[1] : 'n';
	if (type === 's') {
		const v = /<v[^>]*>([\s\S]*?)<\/v>/.exec(cellInner);
		const idx = v ? parseInt(v[1], 10) : NaN;
		return Number.isFinite(idx) ? (sharedStrings[idx] ?? '') : '';
	}
	if (type === 'inlineStr') {
		const is = /<is>([\s\S]*?)<\/is>/.exec(cellInner);
		return is ? extractRunText(is[1]) : '';
	}
	// n (number), str (formula result string), b (boolean), e (error): raw <v> text.
	const v = /<v[^>]*>([\s\S]*?)<\/v>/.exec(cellInner);
	return v ? decodeXmlEntities(v[1]) : '';
}

function findFirstSheetTarget(workbookXml: string, relsXml: string | null): string {
	const sheetTag = /<sheet\b[^>]*\/?>/.exec(workbookXml);
	const rId = sheetTag ? /r:id="([^"]+)"/.exec(sheetTag[0])?.[1] : null;
	if (rId && relsXml) {
		const relTag = new RegExp(`<Relationship\\b[^>]*\\bId="${rId}"[^>]*\\/>`).exec(relsXml);
		const target = relTag ? /Target="([^"]+)"/.exec(relTag[0])?.[1] : null;
		if (target) return target.startsWith('/') ? target.slice(1) : `xl/${target.replace(/^xl\//, '')}`;
	}
	return 'xl/worksheets/sheet1.xml';
}

// readXlsxFirstSheet reads a .xlsx file's first worksheet into a rectangular string grid,
// suitable for feeding straight into the same table state the CSV/TSV paste path builds.
export async function readXlsxFirstSheet(data: ArrayBuffer): Promise<string[][]> {
	const bytes = new Uint8Array(data);
	const entries = parseCentralDirectory(bytes);
	const workbookXml = await readEntryText(bytes, entries, 'xl/workbook.xml');
	if (workbookXml === null) throw new Error('Not a valid .xlsx file (missing xl/workbook.xml).');
	const relsXml = await readEntryText(bytes, entries, 'xl/_rels/workbook.xml.rels');
	const sheetTarget = findFirstSheetTarget(workbookXml, relsXml);
	const sheetXml =
		(await readEntryText(bytes, entries, sheetTarget)) ?? (await readEntryText(bytes, entries, 'xl/worksheets/sheet1.xml'));
	if (sheetXml === null) throw new Error('Could not find the first worksheet in this .xlsx file.');
	const sharedStringsXml = await readEntryText(bytes, entries, 'xl/sharedStrings.xml');
	const sharedStrings = sharedStringsXml ? parseSharedStrings(sharedStringsXml) : [];
	return parseSheetXml(sheetXml, sharedStrings);
}
