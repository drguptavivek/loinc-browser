export type CopyTerm = {
	loincNum: string;
	longCommonName: string;
	exampleUcum?: string;
};

export type CopyFormatId = 'code' | 'codeDisplay' | 'fhir' | 'hl7v2';

export const COPY_FORMATS: { id: CopyFormatId; label: string }[] = [
	{ id: 'code', label: 'Code' },
	{ id: 'codeDisplay', label: 'Code | name' },
	{ id: 'fhir', label: 'FHIR Coding' },
	{ id: 'hl7v2', label: 'HL7 v2 CWE' }
];

export function asCode(t: CopyTerm): string {
	return t.loincNum;
}

export function asCodeDisplay(t: CopyTerm): string {
	return `${t.loincNum} | ${t.longCommonName}`;
}

function escapeHL7(text: string): string {
	return text
		.replace(/\\/g, '\\E\\')
		.replace(/\|/g, '\\F\\')
		.replace(/\^/g, '\\S\\')
		.replace(/&/g, '\\T\\')
		.replace(/~/g, '\\R\\');
}

export function asHL7v2CWE(t: CopyTerm): string {
	return `${t.loincNum}^${escapeHL7(t.longCommonName)}^LN`;
}

export function asFHIRCoding(t: CopyTerm, version?: string): string {
	const coding: Record<string, string> = { system: 'http://loinc.org' };
	if (version) coding.version = version;
	coding.code = t.loincNum;
	coding.display = t.longCommonName;
	return JSON.stringify(coding, null, 2);
}

export function formatTerm(t: CopyTerm, id: CopyFormatId, version?: string): string {
	switch (id) {
		case 'code':
			return asCode(t);
		case 'codeDisplay':
			return asCodeDisplay(t);
		case 'fhir':
			return asFHIRCoding(t, version);
		case 'hl7v2':
			return asHL7v2CWE(t);
	}
}

function csvField(value: string | number | undefined): string {
	let s = value === undefined ? '' : String(value);
	// Uploaded test masters round-trip into spreadsheets: neutralise cells a spreadsheet would run as a formula.
	if (/^[=+\-@\t\r]/.test(s)) s = `'${s}`;
	return /[",\r\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

export function toCSV(rows: Record<string, string | number | undefined>[], columns: string[]): string {
	const lines = [columns.join(',')];
	for (const row of rows) {
		lines.push(columns.map((col) => csvField(row[col])).join(','));
	}
	return lines.join('\r\n');
}

export async function copyText(text: string): Promise<boolean> {
	try {
		if (navigator?.clipboard?.writeText) {
			await navigator.clipboard.writeText(text);
			return true;
		}
	} catch {
		// fall through to legacy fallback
	}
	try {
		const textarea = document.createElement('textarea');
		textarea.value = text;
		textarea.style.position = 'fixed';
		textarea.style.opacity = '0';
		document.body.appendChild(textarea);
		textarea.select();
		const ok = document.execCommand('copy');
		document.body.removeChild(textarea);
		return ok;
	} catch {
		return false;
	}
}

export function downloadText(filename: string, text: string, mime = 'text/csv'): void {
	const blob = new Blob([text], { type: mime });
	const url = URL.createObjectURL(blob);
	const a = document.createElement('a');
	a.href = url;
	a.download = filename;
	document.body.appendChild(a);
	a.click();
	document.body.removeChild(a);
	URL.revokeObjectURL(url);
}
