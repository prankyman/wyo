import { render, screen } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('$app/state', async () => {
    const state = await import('$test/support/page-state.svelte');
    return { page: state.page, navigating: state.navigating };
});
vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    setSignedOutHandler: vi.fn()
}));

import { setSignedOutHandler } from '$lib/api/api';
import { sendToSignIn } from '$lib/api/sign-in-redirect';
import { resetPage, setPageUrl } from '$test/support/page-state.svelte';
import Layout from './+layout.svelte';

const children = createRawSnippet(() => ({ render: () => '<p data-test="inner">page</p>' }));

afterEach(() => {
    resetPage();
    document.documentElement.lang = '';
    vi.mocked(setSignedOutHandler).mockClear();
});

describe('the root layout', () => {
    it('draws the page it is given and sets the language of the document from the address', () => {
        setPageUrl('/settings');

        render(Layout, { props: { children } });

        expect(screen.getByTestId('inner')).toHaveTextContent('page');
        expect(document.documentElement.lang).toBe('ru');
    });

    it('hands every signed-out answer to the sign-in redirect while it is open, and takes it back when it closes', () => {
        const { unmount } = render(Layout, { props: { children } });
        expect(setSignedOutHandler).toHaveBeenLastCalledWith(sendToSignIn);

        unmount();
        expect(setSignedOutHandler).toHaveBeenLastCalledWith(null);
    });
});
