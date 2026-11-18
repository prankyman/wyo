import type { HandleClientError } from '@sveltejs/kit';

/**
 * An error while a page of the wyo interface loads or draws (https://svelte.dev/docs/kit/hooks#Shared-hooks-handleError).
 * SvelteKit's own default writes every one to the browser's console, an address with no page included. Such an address is an
 * expected event: the error page (`src/routes/+error.svelte`) words it, and the console stays for faults, where a person and the
 * browser tests look for them.
 */
export const handleError: HandleClientError = ({ error, status, message }) => {
    if (status !== 404) console.error(error);
    return { message };
};
