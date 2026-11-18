import { refusalParams, type TApiResult } from '$lib/api/api';
import { refusalKey, type TFamily } from '$lib/api/refusal-key';
import { getLocale, t } from '$lib/i18n/i18n';

/**
 * One press that is a call of the wyo server's API with no form around it — «Сделать копию сейчас», «Проверить сейчас», «Выйти» —
 * and the answer it draws: it is working while the call runs (the button's `pending`), says the success in words of the page, or
 * words the refusal from its code (`errors.<code>`). A second press while a call runs does nothing. The answer is cleared the
 * moment the button is pressed again, so «Сохранено» never stands beside a refusal just received.
 */
export class ApiAction {
    pending = $state(false);
    saved = $state<string | null>(null);
    error = $state<string | null>(null);

    /**
     * Runs `call`; on success the answer is the text of `savedKey` (none when it is left out: the page moves on, as after a
     * sign-out), on a refusal the refusal's text — from the family of texts of its place first (`family`, see `refusalKey`).
     * Resolves with the call's result, or null when a call was already running and nothing was started.
     */
    async run<T>(
        call: () => Promise<TApiResult<T>>,
        savedKey?: string,
        family?: TFamily
    ): Promise<TApiResult<T> | null> {
        if (this.pending) return null;
        this.pending = true;
        this.saved = null;
        this.error = null;
        try {
            const result = await call();
            const lang = getLocale();
            if (result.ok) this.saved = savedKey === undefined ? null : t(savedKey, lang);
            else this.error = t(refusalKey(result.refusal.code, family), lang, refusalParams(result.refusal));
            return result;
        } finally {
            this.pending = false;
        }
    }
}
