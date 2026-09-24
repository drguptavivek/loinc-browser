import { writable } from 'svelte/store';
import { toCSV } from './copy';

export type BasketItem = {
	loincNum: string;
	longCommonName: string;
	exampleUcum?: string;
	localName?: string;
	addedAt: number;
};

const STORAGE_KEY = 'loinc.basket.v1';

function loadBasket(): BasketItem[] {
	try {
		const raw = localStorage.getItem(STORAGE_KEY);
		if (!raw) return [];
		const parsed = JSON.parse(raw);
		return Array.isArray(parsed) ? parsed : [];
	} catch {
		return [];
	}
}

function saveBasket(items: BasketItem[]): void {
	try {
		localStorage.setItem(STORAGE_KEY, JSON.stringify(items));
	} catch {
		// ignore quota/availability errors
	}
}

export const basket = writable<BasketItem[]>(loadBasket());

basket.subscribe(saveBasket);

export function addToBasket(item: Omit<BasketItem, 'addedAt'> & { addedAt?: number }): void {
	basket.update((items) => {
		if (items.some((i) => i.loincNum === item.loincNum)) return items;
		return [...items, { ...item, addedAt: item.addedAt ?? Date.now() }];
	});
}

export function removeFromBasket(loincNum: string): void {
	basket.update((items) => items.filter((i) => i.loincNum !== loincNum));
}

export function clearBasket(): void {
	basket.set([]);
}

export function setLocalName(loincNum: string, name: string): void {
	basket.update((items) =>
		items.map((i) => (i.loincNum === loincNum ? { ...i, localName: name } : i))
	);
}

export function basketCSV(items: BasketItem[]): string {
	return toCSV(
		items.map((i) => ({
			loinc_code: i.loincNum,
			long_common_name: i.longCommonName,
			example_ucum: i.exampleUcum,
			local_name: i.localName
		})),
		['loinc_code', 'long_common_name', 'example_ucum', 'local_name']
	);
}
