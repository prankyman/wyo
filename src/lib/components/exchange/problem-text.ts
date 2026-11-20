import { messageParams } from '$lib/api/api';
import { hasMessage, t } from '$lib/i18n/i18n';
import { accountTextKey } from '$lib/marketplaces/refusals';
import type { TLocale } from '$lib/i18n/types';
import type { TProblem } from './types';

/**
 * A list a problem names (orders, postings): the wyo server sends it as a list or as one text of numbers joined by commas; an empty
 * text is an empty list. `null` when the parameter is neither.
 */
function listOf(value: unknown): string[] | null {
    if (Array.isArray(value)) return value.map(String);
    if (typeof value !== 'string') return null;
    return value
        .split(',')
        .map((item) => item.trim())
        .filter((item) => item !== '');
}

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
        const list = listOf(given[name]);
        if (list) filled[name] = list.length > 0 ? list.join(', ') : '—';
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
        if (listOf(given[name]) && hasMessage(`problem.${problem.code}.${suffix}`, lang)) {
            return `problem.${problem.code}.${suffix}`;
        }
    }
    return hasMessage(`problem.${problem.code}`, lang) ? `problem.${problem.code}` : null;
}

/**
 * The key of the sentence of a refusal of the marketplace that has words of its own in the marketplace's family
 * (`marketplace.<marketplace>.refusal.<code>`, found by `accountTextKey`): a label size the marketplace does not make names the
 * marketplaces' own sizes. `null` when the problem is not a refusal, the order's marketplace is not known, or the family has no text for
 * the refusal's code, and then the general sentence of the problem stands.
 */
function refusalKeyOf(problem: TProblem, marketplace: string | undefined, lang: TLocale): string | null {
    if (problem.code !== 'marketplace_refused' || !problem.marketplaceCode || !marketplace) return null;
    const key = accountTextKey('refusal', problem.marketplaceCode, marketplace, lang);
    return key !== null && hasMessage(key, lang) ? key : null;
}

/**
 * wyo's sentence for a problem's code; a code wyo has no text for gets the general text that names the code. `marketplace` is the
 * code of the marketplace the order or the hand-over belongs to: a refusal it has its own words for is worded by them.
 */
export function problemSentence(problem: TProblem, lang: TLocale, marketplace?: string): string {
    const own = refusalKeyOf(problem, marketplace, lang);
    if (own) return t(own, lang, paramsOf(problem, lang));
    const key = keyOf(problem, lang);
    return key ? t(key, lang, paramsOf(problem, lang)) : t('problem.unknown', lang, { code: problem.code });
}

/** What a problem means and what the owner does about it: the text behind its «?». A refusal worded by the marketplace's family is explained by its `.help` when it has one, and by the general text of a refusal when it has not. */
export function problemHelp(problem: TProblem, lang: TLocale, marketplace?: string): string {
    const own = refusalKeyOf(problem, marketplace, lang);
    if (own) {
        return hasMessage(`${own}.help`, lang)
            ? t(`${own}.help`, lang, paramsOf(problem, lang))
            : t('problem.marketplace_refused.help', lang);
    }
    const key = keyOf(problem, lang);
    return key ? t(`${key}.help`, lang, paramsOf(problem, lang)) : t('problem.unknown.help', lang);
}

/** Whose words `problem.words` are: only a refusal of the fulfilment centre carries the fulfilment centre's own. */
export function wordsOwner(problem: TProblem): 'fulfilment' | 'marketplace' {
    return problem.code === 'fc_refused' ? 'fulfilment' : 'marketplace';
}
