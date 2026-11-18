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
    if (Array.isArray(given.orders)) filled.orders = given.orders.join(', ');
    return filled;
}

/** wyo's sentence for a problem's code; a code wyo has no text for gets the general text that names the code. */
export function problemSentence(problem: TProblem, lang: TLocale): string {
    return hasMessage(`problem.${problem.code}`, lang)
        ? t(`problem.${problem.code}`, lang, paramsOf(problem, lang))
        : t('problem.unknown', lang, { code: problem.code });
}

/** What a problem means and what the owner does about it: the text behind its «?». */
export function problemHelp(problem: TProblem, lang: TLocale): string {
    return hasMessage(`problem.${problem.code}`, lang)
        ? t(`problem.${problem.code}.help`, lang, paramsOf(problem, lang))
        : t('problem.unknown.help', lang);
}

/** Whose words `problem.words` are: only a refusal of the fulfilment centre carries the fulfilment centre's own. */
export function wordsOwner(problem: TProblem): 'fulfilment' | 'marketplace' {
    return problem.code === 'fc_refused' ? 'fulfilment' : 'marketplace';
}
