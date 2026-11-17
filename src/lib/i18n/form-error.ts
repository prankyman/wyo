import type { TLocale } from './types';
import { t } from './i18n';

/**
 * Turns whatever superforms put in an `errors` store into one translated line.
 *
 * A zod message and a refusal of the wyo server are i18n KEYS, not text, so they
 * have to go through `t()` before they are shown. The shape varies — a bare string, an
 * array of strings, or a nested `{ _errors }` object — hence the narrowing below.
 */
export function tError(error: unknown, lang: TLocale, params?: Record<string, string | number>): string {
    if (!error) return '';
    if (typeof error === 'string') return t(error, lang, params);
    if (Array.isArray(error) && error.length > 0) {
        const first = error[0];
        if (typeof first === 'string') return t(first, lang, params);
    }
    if (typeof error === 'object' && error !== null && '_errors' in error) {
        const errors = (error as { _errors?: string[] })._errors;
        if (errors && errors.length > 0) return t(errors[0], lang, params);
    }
    return '';
}
