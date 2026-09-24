import assert from 'node:assert';
import { existsSync, readFileSync } from 'node:fs';
import zlib from 'node:zlib';
import { readXlsxFirstSheet } from './xlsx.ts';

// --- Tiny in-memory ZIP writer (stored + one deflated entry), just enough to build a test .xlsx ---

const CRC_TABLE = (() => {
	const table = new Uint32Array(256);
	for (let n = 0; n < 256; n++) {
		let c = n;
		for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
		table[n] = c >>> 0;
	}
	return table;
})();

function crc32(data: Buffer): number {
	let crc = 0xffffffff;
	for (const byte of data) crc = CRC_TABLE[(crc ^ byte) & 0xff] ^ (crc >>> 8);
	return (crc ^ 0xffffffff) >>> 0;
}

type ZipInput = { name: string; content: Buffer; deflate?: boolean };

function buildZip(files: ZipInput[]): ArrayBuffer {
	const localChunks: Buffer[] = [];
	const centralChunks: Buffer[] = [];
	let offset = 0;

	for (const file of files) {
		const nameBuf = Buffer.from(file.name, 'utf-8');
		const method = file.deflate ? 8 : 0;
		const data = file.deflate ? zlib.deflateRawSync(file.content) : file.content;
		const crc = crc32(file.content);

		const local = Buffer.alloc(30);
		local.writeUInt32LE(0x04034b50, 0);
		local.writeUInt16LE(20, 4); // version needed
		local.writeUInt16LE(0, 6); // flags
		local.writeUInt16LE(method, 8);
		local.writeUInt16LE(0, 10); // mod time
		local.writeUInt16LE(0, 12); // mod date
		local.writeUInt32LE(crc, 14);
		local.writeUInt32LE(data.length, 18);
		local.writeUInt32LE(file.content.length, 22);
		local.writeUInt16LE(nameBuf.length, 26);
		local.writeUInt16LE(0, 28);
		localChunks.push(local, nameBuf, data);

		const central = Buffer.alloc(46);
		central.writeUInt32LE(0x02014b50, 0);
		central.writeUInt16LE(20, 4); // version made by
		central.writeUInt16LE(20, 6); // version needed
		central.writeUInt16LE(0, 8); // flags
		central.writeUInt16LE(method, 10);
		central.writeUInt16LE(0, 12);
		central.writeUInt16LE(0, 14);
		central.writeUInt32LE(crc, 16);
		central.writeUInt32LE(data.length, 20);
		central.writeUInt32LE(file.content.length, 24);
		central.writeUInt16LE(nameBuf.length, 28);
		central.writeUInt16LE(0, 30);
		central.writeUInt16LE(0, 32);
		central.writeUInt16LE(0, 34);
		central.writeUInt16LE(0, 36);
		central.writeUInt32LE(0, 38);
		central.writeUInt32LE(offset, 42);
		centralChunks.push(central, nameBuf);

		offset += local.length + nameBuf.length + data.length;
	}

	const centralStart = offset;
	const central = Buffer.concat(centralChunks);
	const eocd = Buffer.alloc(22);
	eocd.writeUInt32LE(0x06054b50, 0);
	eocd.writeUInt16LE(0, 4);
	eocd.writeUInt16LE(0, 6);
	eocd.writeUInt16LE(files.length, 8);
	eocd.writeUInt16LE(files.length, 10);
	eocd.writeUInt32LE(central.length, 12);
	eocd.writeUInt32LE(centralStart, 16);
	eocd.writeUInt16LE(0, 20);

	const buf = Buffer.concat([...localChunks, central, eocd]);
	return buf.buffer.slice(buf.byteOffset, buf.byteOffset + buf.byteLength);
}

const CONTENT_TYPES = `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/xml"/></Types>`;

const WORKBOOK_XML = `<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets></workbook>`;

const WORKBOOK_RELS = `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`;

// sharedStrings: index 0 plain "Name &amp; Co", index 1 rich text "Hb" + "A1c" runs
const SHARED_STRINGS = `<?xml version="1.0" encoding="UTF-8"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="2" uniqueCount="2">
<si><t>Name &amp; Co</t></si>
<si><r><t>Hb</t></r><r><t>A1c</t></r></si>
</sst>`;

// Row 1: A1 = shared string 0, C1 = shared string 1 (gap at B1).
// Row 2: A2 inline string, B2 number, D2 self-closing (empty), gap at C2.
function sheetXml(): string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<sheetData>
<row r="1"><c r="A1" t="s"><v>0</v></c><c r="C1" t="s"><v>1</v></c></row>
<row r="2"><c r="A2" t="inlineStr"><is><t>Sodium</t></is></c><c r="B2"><v>42</v></c><c r="D2"/></row>
</sheetData></worksheet>`;
}

async function run() {
	const zip = buildZip([
		{ name: '[Content_Types].xml', content: Buffer.from(CONTENT_TYPES) },
		{ name: 'xl/workbook.xml', content: Buffer.from(WORKBOOK_XML) },
		{ name: 'xl/_rels/workbook.xml.rels', content: Buffer.from(WORKBOOK_RELS) },
		{ name: 'xl/sharedStrings.xml', content: Buffer.from(SHARED_STRINGS) },
		{ name: 'xl/worksheets/sheet1.xml', content: Buffer.from(sheetXml()), deflate: true },
	]);

	const grid = await readXlsxFirstSheet(zip);
	assert.deepStrictEqual(grid, [
		['Name & Co', '', 'HbA1c', ''],
		['Sodium', '42', '', ''],
	]);
	console.log('xlsx.check.ts: parses stored+deflated zip, shared/rich strings, inline strings, gaps, entities');

	// Stored (uncompressed) entry variant.
	const zipStored = buildZip([
		{ name: '[Content_Types].xml', content: Buffer.from(CONTENT_TYPES) },
		{ name: 'xl/workbook.xml', content: Buffer.from(WORKBOOK_XML) },
		{ name: 'xl/_rels/workbook.xml.rels', content: Buffer.from(WORKBOOK_RELS) },
		{ name: 'xl/sharedStrings.xml', content: Buffer.from(SHARED_STRINGS) },
		{ name: 'xl/worksheets/sheet1.xml', content: Buffer.from(sheetXml()) },
	]);
	const gridStored = await readXlsxFirstSheet(zipStored);
	assert.deepStrictEqual(gridStored, grid);
	console.log('xlsx.check.ts: stored (method 0) entries parse identically');

	// Optional: a real .xlsx produced by openpyxl, when present in the scratch dir.
	const samplePath = '/private/tmp/claude-501/xlsx-test/sample.xlsx';
	if (existsSync(samplePath)) {
		const fileBuf = readFileSync(samplePath);
		const sampleGrid = await readXlsxFirstSheet(
			fileBuf.buffer.slice(fileBuf.byteOffset, fileBuf.byteOffset + fileBuf.byteLength),
		);
		console.log('xlsx.check.ts: real openpyxl sample.xlsx ->', JSON.stringify(sampleGrid));
		assert.ok(sampleGrid.length >= 4, 'expected header + 3 data rows');
	} else {
		console.log('xlsx.check.ts: skipped real-file check (no sample.xlsx in scratch dir)');
	}
}

run().then(() => console.log('xlsx.check.ts: all assertions passed'));
