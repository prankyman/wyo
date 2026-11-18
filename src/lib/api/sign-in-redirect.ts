import { goto } from '$app/navigation';
import { page } from '$app/state';
import { getLocale, localizePath, stripLocale } from '$lib/i18n/i18n';
import { session } from '$lib/stores/session.svelte';

/** The address of the sign-in page, before the language prefix. */
export const SIGN_IN_PATH = '/auth/login';

/**
 * What happens when any call of the wyo server is answered `signed_out` (`setSignedOutHandler`, `api.ts`): the interface forgets
 * the person and opens the sign-in page, whatever page and whatever call it was — a press, the bell's background read, or the
 * idle sign-out. On the sign-in page itself nothing moves.
 */
export function sendToSignIn(): void {
    session.signedOut();
    if (stripLocale(page.url.pathname).path === SIGN_IN_PATH) return;
    void goto(localizePath(SIGN_IN_PATH, getLocale()));
}
