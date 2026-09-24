import { getVersion } from './api';

// loincVersion() caches /api/version's loincVersion at module scope, so every caller across the
// page (TermCard, FindMode) shares one fetch instead of one per component instance.
let versionPromise: Promise<string | undefined> | null = null;

export function loincVersion(): Promise<string | undefined> {
	if (!versionPromise) {
		versionPromise = getVersion()
			.then((v) => v.loincVersion)
			.catch(() => undefined);
	}
	return versionPromise;
}
