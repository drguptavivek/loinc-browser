import { writable } from 'svelte/store';
import type { SearchParams } from './api';
import type { CopyFormatId } from './copy';

export type Setup = {
	// '' = English (no lang= sent)
	lang: string;
	commonCodesOnly: boolean;
	orderableOnly: boolean;
	copyFormat: CopyFormatId;
};

const STORAGE_KEY = 'loinc.setup.v1';

const defaults: Setup = { lang: '', commonCodesOnly: false, orderableOnly: false, copyFormat: 'code' };

function loadSetup(): Setup {
	try {
		const raw = localStorage.getItem(STORAGE_KEY);
		if (!raw) return { ...defaults };
		const parsed = JSON.parse(raw);
		return {
			lang: typeof parsed.lang === 'string' ? parsed.lang : defaults.lang,
			commonCodesOnly: typeof parsed.commonCodesOnly === 'boolean' ? parsed.commonCodesOnly : defaults.commonCodesOnly,
			orderableOnly: typeof parsed.orderableOnly === 'boolean' ? parsed.orderableOnly : defaults.orderableOnly,
			copyFormat: typeof parsed.copyFormat === 'string' ? parsed.copyFormat : defaults.copyFormat,
		};
	} catch {
		return { ...defaults };
	}
}

function saveSetup(value: Setup): void {
	try {
		localStorage.setItem(STORAGE_KEY, JSON.stringify(value));
	} catch {
		// ignore quota/availability errors
	}
}

export const setup = writable<Setup>(loadSetup());

setup.subscribe(saveSetup);

// setupParams turns the saved setup into the SearchParams fields it maps to, omitting anything
// left at its default so callers can spread it over their own params.
export function setupParams(s: Setup): SearchParams {
	const params: SearchParams = {};
	if (s.lang) params.lang = s.lang;
	if (s.commonCodesOnly) params.commonCodes = true;
	if (s.orderableOnly) params.universalLabOrders = true;
	return params;
}
