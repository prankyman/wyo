import { fireEvent, render, screen } from '@testing-library/svelte';
import { tick } from 'svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import Harness from '$test/harnesses/forms/PageSearchTestHarness.svelte';
import { resetPage, setPageUrl } from '$test/support/page-state.svelte';

vi.mock('$app/navigation', async () => (await import('$test/support/navigation')).navigationModule);
vi.mock('$app/state', async () => {
    const state = await import('$test/support/page-state.svelte');
    return { page: state.page, navigating: state.navigating };
});

afterEach(() => resetPage());

/** A list's filter whose boxes follow the list's address. */
describe('pageSearch', () => {
    it('starts from the page data and follows it when the address changes', async () => {
        setPageUrl('/app/list?status=open');
        const { rerender } = render(Harness, { props: { status: 'open' } });
        expect(screen.getByTestId('harness-status')).toHaveValue('open');

        // SvelteKit hands the new data and the new address over in one update.
        await rerender({ status: 'done' });
        setPageUrl('/app/list?status=done');
        await tick();

        expect(screen.getByTestId('harness-status')).toHaveValue('done');
    });

    it('keeps a choice not sent yet when the page re-reads its data on the same address', async () => {
        setPageUrl('/app/list?status=open');
        const { rerender } = render(Harness, { props: { status: 'open' } });
        const box = screen.getByTestId('harness-status');
        await fireEvent.input(box, { target: { value: 'draft' } });

        // A live notice: the data is read again, the address stays.
        await rerender({ status: 'open' });

        expect(box).toHaveValue('draft');
    });

    it("puts the address's values back after the back button", async () => {
        setPageUrl('/app/list?status=done');
        const { rerender } = render(Harness, { props: { status: 'done' } });
        await fireEvent.input(screen.getByTestId('harness-status'), { target: { value: 'draft' } });

        await rerender({ status: 'open' });
        setPageUrl('/app/list?status=open');
        await tick();

        expect(screen.getByTestId('harness-status')).toHaveValue('open');
    });
});
