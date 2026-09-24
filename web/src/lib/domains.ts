import type { SearchParams } from './api';

// Find-mode entry points: each domain is a preset of term-search filters. Class values checked
// against LOINC 2.82 (class filter is an exact match, repeatable).
export type DomainId = 'lab' | 'panels' | 'radiology' | 'documents' | 'forms' | 'vitals' | 'all';

export type Domain = {
	id: DomainId;
	label: string;
	hint: string;
	params: SearchParams;
	// panels use /api/v1/panels/search instead of term search
	panels?: boolean;
};

export const DOMAINS: Domain[] = [
	{ id: 'lab', label: 'Lab tests', hint: 'e.g. S. creatinine, HbA1c, TSH', params: { classType: 'lab' } },
	{ id: 'panels', label: 'Lab panels & orders', hint: 'e.g. LFT, CBC, lipid panel', params: {}, panels: true },
	{ id: 'radiology', label: 'Radiology', hint: 'e.g. CT chest with contrast, USG abdomen', params: { class: 'RAD' } },
	{
		id: 'documents',
		label: 'Documents & summaries',
		hint: 'e.g. discharge summary, consult note',
		params: { class: ['DOC.ONTOLOGY', 'DOC.MISC', 'DOC.REF', 'DOC.ADMIN', 'DOCUMENT.REGULATORY'] }
	},
	{ id: 'forms', label: 'Forms & assessments', hint: 'e.g. PHQ-9, GAD-7', params: { classType: 'survey' } },
	{
		id: 'vitals',
		label: 'Vitals',
		hint: 'e.g. blood pressure, heart rate, BMI',
		params: {
			class: ['BP.ATOM', 'HRTRATE.ATOM', 'RESP.ATOM', 'BDYWGT.ATOM', 'BDYHGT.ATOM', 'BDYTMP.ATOM', 'BDYCRC.ATOM', 'BDYSURF.ATOM']
		}
	},
	{ id: 'all', label: 'Everything', hint: 'search all LOINC terms', params: {} }
];

export function domainById(id: string | null | undefined): Domain {
	return DOMAINS.find((d) => d.id === id) ?? DOMAINS[0];
}
