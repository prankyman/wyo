import { hasMessage, t } from '$lib/i18n/i18n';
import type { TLocale } from '$lib/i18n/types';
import type { THandover } from './types';

/** How a batch that the marketplace makes by itself leaves, worded, with what the owner does about it when the marketplace's family says. */
export interface IBatchKind {
    text: string;
    help?: string;
}

/**
 * The kind of a hand-over's batch (`Handover.batchKind`), in the words of the marketplace's own family of texts
 * (`marketplace.<code>.batchKind.<WORD>`, with `.help` beside it). `null` for a hand-over whose batches wyo made itself. A word
 * the family has no text for is given as the marketplace sent it, with no «?», so that a new kind is shown at once, not hidden.
 */
export function batchKindOf(handover: Pick<THandover, 'marketplace' | 'batchKind'>, lang: TLocale): IBatchKind | null {
    const word = handover.batchKind;
    if (!word) return null;
    const key = `marketplace.${handover.marketplace}.batchKind.${word}`;
    if (!hasMessage(key, lang)) return { text: word };
    return hasMessage(`${key}.help`, lang)
        ? { text: t(key, lang), help: t(`${key}.help`, lang) }
        : { text: t(key, lang) };
}
