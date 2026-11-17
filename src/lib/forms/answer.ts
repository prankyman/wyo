import { t } from '$lib/i18n/i18n';
import type { TLocale } from '$lib/i18n/types';

/** The values a message key is filled with. */
export type TAnswerParams = Record<string, string | number>;

/**
 * The message a form answers with: a success names what was done in a short code the page words itself (`'rebuild'`,
 * `'added'`), with any figures it reports; a refusal is a message key with its values, never translated text. `FormAnswer`
 * draws either under the form's own button.
 */
export type TFormAnswer =
    | { saved: string; count?: number; total?: number; params?: TAnswerParams; error?: undefined }
    | { error: string; params?: TAnswerParams; saved?: undefined };

/** The form's message as an answer, or nothing when the form has not answered (or answered with something else). */
export function answerOf(message: unknown): TFormAnswer | undefined {
    if (!message || typeof message !== 'object') return undefined;
    const value = message as Record<string, unknown>;
    if (typeof value.error === 'string' || typeof value.saved === 'string') return value as TFormAnswer;
    return undefined;
}

/**
 * The two lines `FormAnswer` draws: the refusal worded from its key, or the success worded by the page (`savedText`
 * receives the answer, so a count can go into the text). A page that has one fixed success text passes it as a string.
 */
export function answerLines(
    message: unknown,
    lang: TLocale,
    savedText: string | ((answer: Extract<TFormAnswer, { saved: string }>) => string | null)
): { saved: string | null; error: string | null } {
    const answer = answerOf(message);
    if (!answer) return { saved: null, error: null };
    if (answer.error !== undefined) return { saved: null, error: t(answer.error, lang, answer.params) };
    return { saved: typeof savedText === 'string' ? savedText : savedText(answer), error: null };
}

/**
 * The object an action's answer carries next to its key (`{ saved, value }`): what the wyo server returned, which a page
 * reads as its own state type; null when the answer carries none.
 */
export function answerValue<TValue extends object>(message: unknown): TValue | null {
    const value = typeof message === 'object' && message !== null ? (message as { value?: unknown }).value : undefined;
    return typeof value === 'object' && value !== null ? (value as TValue) : null;
}

/**
 * A button's answer worded as one line: the refusal or the success, each from its own key (a button whose answer is a saved
 * KEY rather than a page text). No answer is null.
 */
export function answerOutcome(
    answer: TFormAnswer | undefined,
    lang: TLocale
): { kind: 'saved' | 'error'; text: string } | null {
    if (!answer) return null;
    if (answer.error !== undefined) return { kind: 'error', text: t(answer.error, lang, answer.params) };
    return { kind: 'saved', text: t(answer.saved, lang, answer.params) };
}

/** The first message of a superforms error tree, depth first in the schema's field order. */
export function firstError(errors: unknown): string | undefined {
    if (typeof errors === 'string') return errors;
    if (Array.isArray(errors)) {
        for (const item of errors) {
            const found = firstError(item);
            if (found) return found;
        }
        return undefined;
    }
    if (errors && typeof errors === 'object') {
        for (const value of Object.values(errors)) {
            const found = firstError(value);
            if (found) return found;
        }
    }
    return undefined;
}
