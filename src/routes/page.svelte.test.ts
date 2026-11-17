import { render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { messages } from '$lib/i18n/messages';
import Page from './+page.svelte';

/** A fetch that answers every call with `body` and `status`. */
function answering(status: number, body: unknown) {
    const stub = vi.fn(async () => new Response(JSON.stringify(body), { status }));
    vi.stubGlobal('fetch', stub);
    return stub;
}

afterEach(() => {
    vi.unstubAllGlobals();
});

describe('the start page', () => {
    it('shows the program’s name and the version the wyo server reports', async () => {
        const stub = answering(200, { name: 'wyo', version: '1.2.3', mode: 'release' });

        render(Page);

        expect(screen.getByTestId('start-loading')).toBeInTheDocument();
        expect(await screen.findByTestId('start-version')).toHaveTextContent(
            messages.ru['start.version'].replace('{version}', '1.2.3')
        );
        expect(screen.getByTestId('start-name')).toHaveTextContent('wyo');
        expect(stub).toHaveBeenCalledExactlyOnceWith('/api/v1/meta', expect.objectContaining({ method: 'GET' }));
    });

    it('says so when the wyo server cannot be reached', async () => {
        vi.stubGlobal(
            'fetch',
            vi.fn(async () => {
                throw new TypeError('Failed to fetch');
            })
        );

        render(Page);

        expect(await screen.findByTestId('start-refusal')).toHaveTextContent(messages.ru['errors.network']);
        expect(screen.queryByTestId('start-version')).not.toBeInTheDocument();
    });

    it('words a refusal of the wyo server from its code', async () => {
        answering(404, { error: { code: 'not_found' } });

        render(Page);

        expect(await screen.findByTestId('start-refusal')).toHaveTextContent(messages.ru['errors.not_found']);
    });
});
