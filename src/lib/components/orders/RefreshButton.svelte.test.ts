import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { post: vi.fn() }
}));

import { api } from '$lib/api/api';
import RefreshButton from './RefreshButton.svelte';

const post = vi.mocked(api.post);
const NOW = '2026-11-18T12:00:00';

beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(new Date(NOW));
});
afterEach(() => {
    vi.useRealTimers();
    post.mockReset();
});

/** «Обновить» on the order list: reads the orders at once, answered at most once in 5 minutes. */
describe('RefreshButton', () => {
    it('starts the reading, says so, tells the page, and then stays blocked for the minutes the wyo server names', async () => {
        post.mockResolvedValue({ ok: true, data: { lastRefreshAt: NOW, nextAllowedAt: '2026-11-18T12:05:00' } });
        const onRefreshed = vi.fn();
        render(RefreshButton, { props: { lang: 'ru', refresh: {}, canRefresh: true, onRefreshed } });
        expect(screen.getByTestId('orders-refresh')).not.toBeDisabled();

        await fireEvent.click(screen.getByTestId('orders-refresh'));

        await waitFor(() => expect(onRefreshed).toHaveBeenCalledOnce());
        expect(post).toHaveBeenCalledExactlyOnceWith('/api/v1/sync/refresh', {});
        expect(screen.getByTestId('orders-refresh-answer-saved')).toHaveTextContent('Чтение заказов началось');
        expect(screen.getByTestId('orders-refresh')).toBeDisabled();
        expect(screen.getByTestId('orders-refresh-reason')).toHaveTextContent('Обновить можно через 5 мин.');
    });

    it('is drawn blocked with the minutes left when the page opens inside the 5 minutes', () => {
        render(RefreshButton, {
            props: { lang: 'ru', refresh: { nextAllowedAt: '2026-11-18T12:02:30' }, canRefresh: true }
        });

        expect(screen.getByTestId('orders-refresh')).toBeDisabled();
        expect(screen.getByTestId('orders-refresh-reason')).toHaveTextContent('Обновить можно через 3 мин.');
        expect(screen.getByTestId('orders-refresh-help-button')).toBeInTheDocument();
    });

    it('takes the time from a refusal `too_soon` when another browser pressed it first', async () => {
        post.mockResolvedValue({
            ok: false,
            status: 429,
            refusal: { code: 'too_soon', params: { nextAllowedAt: '2026-11-18T12:04:00' } }
        });
        render(RefreshButton, { props: { lang: 'ru', refresh: {}, canRefresh: true } });

        await fireEvent.click(screen.getByTestId('orders-refresh'));

        await waitFor(() =>
            expect(screen.getByTestId('orders-refresh-reason')).toHaveTextContent('Обновить можно через 4 мин.')
        );
        expect(screen.getByTestId('orders-refresh-answer-error')).toHaveTextContent('не чаще раза в 5 минут');
    });

    it('is blocked with its reason when no account works', () => {
        render(RefreshButton, { props: { lang: 'ru', refresh: {}, canRefresh: false } });

        expect(screen.getByTestId('orders-refresh')).toBeDisabled();
        expect(screen.getByTestId('orders-refresh-reason')).toHaveTextContent('нет ни одного работающего аккаунта');
    });
});
