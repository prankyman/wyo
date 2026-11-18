import { screen, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { accountOf, journalEntryOf, MARKETPLACE, MARKETPLACE_NAME } from '$test/support/marketplaces';
import { renderPageWith } from '$test/support/render-page';
import { resetPage, setPageUrl } from '$test/support/page-state.svelte';
import type { TJournalEntry } from '$lib/components/journal/types';
import type { TJournalFilter } from '$lib/validation/list-filters';

vi.mock('$app/state', async () => {
    const state = await import('$test/support/page-state.svelte');
    return { page: state.page, navigating: state.navigating };
});
vi.mock('$app/navigation', async () => (await import('$test/support/navigation')).navigationModule);

import Page from './+page.svelte';

const NO_FILTER: TJournalFilter = { account: '', outcome: '', from: '', to: '' };
const FAILED = journalEntryOf({
    id: 502,
    outcome: 'server_error',
    status: 503,
    errorCode: 'service_unavailable',
    errorText: 'The service is temporarily unavailable'
});

function pageData(
    change: { filter?: Partial<TJournalFilter>; entries?: TJournalEntry[]; opened?: TJournalEntry | null } = {}
) {
    const entries = change.entries ?? [journalEntryOf(), FAILED];
    return {
        filter: { ...NO_FILTER, ...change.filter },
        list: { entries, total: entries.length, page: 1, pageSize: 50 },
        refusal: null,
        accounts: [accountOf()],
        marketplaces: [MARKETPLACE],
        opened: change.opened ?? null
    };
}

beforeEach(() => setPageUrl('/journal'));
afterEach(() => resetPage());

describe('«Журнал запросов»', () => {
    it('draws its heading with one «?» saying it holds no key, and a row for each request, newest first as read', () => {
        renderPageWith(Page, pageData());

        expect(screen.getByTestId('journal-heading')).toHaveTextContent('Журнал запросов');
        expect(screen.getByTestId('journal-help-button').getAttribute('aria-label')).toBe('Журнал запросов');
        const rows = within(screen.getByTestId('journal-list-table')).getAllByTestId('journal-list-row');
        expect(rows.map((row) => row.getAttribute('data-entry-id'))).toEqual(['501', '502']);
        expect(within(rows[0]).getByTestId('journal-list-link')).toHaveAttribute('href', '/journal?entry=501');
        expect(rows[0]).toHaveTextContent('GET /api/v3/orders/new');
        expect(rows[0]).toHaveTextContent('Новые заказы');
        expect(rows[0]).toHaveTextContent('312 мс');
        expect(within(rows[0]).getByTestId('journal-list-mark')).toHaveTextContent(MARKETPLACE_NAME);
        expect(within(rows[1]).getByTestId('journal-list-outcome')).toHaveTextContent('Ошибка сервера');
        expect(rows[1]).toHaveTextContent('service_unavailable');
    });

    it('says the journal is empty and why, when nothing was asked yet', () => {
        renderPageWith(Page, pageData({ entries: [] }));

        expect(screen.getByTestId('journal-empty')).toHaveTextContent(
            'ещё не отправила маркетплейсу ни одного запроса'
        );
    });

    it('shows the filters in use as chips and says what was filtered when nothing is found', () => {
        setPageUrl('/journal?outcome=key_refused&from=2026-11-01');
        renderPageWith(Page, pageData({ filter: { outcome: 'key_refused', from: '2026-11-01' }, entries: [] }));

        expect(screen.getByTestId('journal-filter-chip-outcome')).toHaveAttribute('href', '/journal?from=2026-11-01');
        expect(screen.getByTestId('journal-filter-outcome')).toHaveValue('key_refused');
        expect(screen.getByTestId('journal-empty-filtered')).toHaveTextContent(
            'По фильтру ничего не найдено: «Итог: Ключ не принят», «С 01 нояб. 2026 г.».'
        );
    });

    it('opens the whole row the address names beside the list, its error text with it', () => {
        setPageUrl('/journal?entry=502');
        renderPageWith(Page, pageData({ opened: FAILED }));

        expect(screen.getByTestId('journal-details-panel')).toHaveTextContent('Запрос GET /api/v3/orders/new');
        expect(screen.getByTestId('journal-entry-facts-errorText')).toHaveTextContent(
            'The service is temporarily unavailable'
        );
        expect(screen.getByTestId('journal-entry-facts-status')).toHaveTextContent('503');
    });
});
