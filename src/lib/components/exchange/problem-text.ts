import { messageParams } from '$lib/api/api';
import { hasMessage, t } from '$lib/i18n/i18n';
import type { TLocale } from '$lib/i18n/types';
import type { TProblem } from './types';

/**
 * The parameters of a problem's sentence as texts: a step or an identifier is written out in words and a list is joined, so the
 * sentence reads the same wherever it is drawn (`ProblemLine`, the list of hand-overs).
 */
function paramsOf(problem: TProblem, lang: TLocale): Record<string, string | number> {
    const given = problem.params ?? {};
    const filled: Record<string, string | number> = { ...messageParams(given) };
    if (typeof given.step === 'string') filled.step = t(`exchange.step.${given.step}`, lang);
    if (typeof given.identifier === 'string') filled.identifier = t(`exchange.identifier.${given.identifier}`, lang);
    for (const name of ['orders', 'postings', 'missing', 'extra']) {
        const list = given[name];
        if (Array.isArray(list)) filled[name] = list.length > 0 ? list.join(', ') : '—';
    }
    return filled;
}

/**
 * The key of the text of a problem: `problem.<code>`, or, when the problem names an identifier and wyo has words of their own for
 * that identifier (`problem.<code>.<identifier>`: a product sent in several boxes needs its number of boxes entered, which is
 * not a number to be written in a sentence about a code), those. `null` when wyo has no text for the code.
 */
function keyOf(problem: TProblem, lang: TLocale): string | null {
    const given = problem.params ?? {};
    if (typeof given.identifier === 'string' && hasMessage(`problem.${problem.code}.${given.identifier}`, lang)) {
        return `problem.${problem.code}.${given.identifier}`;
    }
    // A refusal that names the postings it concerns, or says the shipment's contents differ from the hand-over's, has words for it.
    for (const [name, suffix] of [
        ['postings', 'postings'],
        ['missing', 'differs'],
        ['extra', 'differs']
    ] as const) {
        if (Array.isArray(given[name]) && hasMessage(`problem.${problem.code}.${suffix}`, lang)) {
            return `problem.${problem.code}.${suffix}`;
        }
    }
    return hasMessage(`problem.${problem.code}`, lang) ? `problem.${problem.code}` : null;
}

/** wyo's sentence for a problem's code; a code wyo has no text for gets the general text that names the code. */
export function problemSentence(problem: TProblem, lang: TLocale): string {
    const key = keyOf(problem, lang);
    return key ? t(key, lang, paramsOf(problem, lang)) : t('problem.unknown', lang, { code: problem.code });
}

/** What a problem means and what the owner does about it: the text behind its «?». */
export function problemHelp(problem: TProblem, lang: TLocale): string {
    const key = keyOf(problem, lang);
    return key ? t(`${key}.help`, lang, paramsOf(problem, lang)) : t('problem.unknown.help', lang);
}

/** Whose words `problem.words` are: only a refusal of the fulfilment centre carries the fulfilment centre's own. */
export function wordsOwner(problem: TProblem): 'fulfilment' | 'marketplace' {
    return problem.code === 'fc_refused' ? 'fulfilment' : 'marketplace';
}
