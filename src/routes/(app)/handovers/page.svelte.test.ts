import { screen, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { handoverDetailsOf, handoverOf, exchangeProblemOf } from '$test/support/external-api';
import { resetPage, setPageUrl } from '$test/support/page-state.svelte';
import { renderPageWith } from '$test/support/render-page';
import type { THandover, THandoverDetails } from '$lib/components/handovers/types';

vi.mock('$app/state', async () => {
    const state = await import('$test/support/page-state.svelte');
    return { page: state.page, navigating: state.navigating };
});
vi.mock('$app/navigation', async () => (await import('$test/support/navigation')).navigationModule);

import Page from './+page.svelte';

function pageData(change: { handovers?: THandover[]; opened?: THandoverDetails | null; refusal?: string | null } = {}) {
    return {
        list: change.refusal ? null : { handovers: change.handovers ?? [handoverOf()] },
        refusal: change.refusal ?? null,
        opened: change.opened ?? null
    };
}

beforeEach(() => setPageUrl('/handovers'));
afterEach(() => resetPage());

describe('«Передачи»', () => {
    it('draws its heading with one «?» and a row for each hand-over: number, day, state, batches, orders and the problem, linked to its details', () => {
        renderPageWith(
            Page,
            pageData({
                handovers: [
                    handoverOf(),
                    handoverOf({
                        id: 32,
                        number: 'HO-2026-0043',
                        state: 'problem',
                        batches: [],
                        problem: exchangeProblemOf({
                            code: 'shipping_point_needed',
                            params: undefined,
                            words: undefined,
                            marketplaceCode: undefined
                        })
                    })
                ]
            })
        );

        expect(screen.getByTestId('handovers-heading')).toHaveTextContent('Передачи');
        expect(screen.getAllByTestId('handovers-help-button')).toHaveLength(1);
        const rows = within(screen.getByTestId('handovers-list-table')).getAllByTestId('handovers-list-row');
        expect(rows.map((row) => row.getAttribute('data-handover-id'))).toEqual(['31', '32']);
        expect(within(rows[0]).getByTestId('handovers-list-link')).toHaveAttribute('href', '/handovers?handover=31');
        expect(rows[0]).toHaveTextContent('HO-2026-0042');
        expect(rows[0]).toHaveTextContent('19 нояб. 2026');
        expect(rows[0]).toHaveTextContent('GI-1001');
        expect(within(rows[0]).getByTestId('handovers-list-state')).toHaveTextContent('идёт');
        expect(rows[1]).toHaveTextContent('Не выбран пункт сдачи партии.');
        expect(within(rows[1]).getByTestId('handovers-list-state')).toHaveTextContent('проблема');
    });

    it('says why the list is empty and what makes hand-overs appear', () => {
        renderPageWith(Page, pageData({ handovers: [] }));

        expect(screen.getByTestId('handovers-empty')).toHaveTextContent('Передач пока нет');
        expect(screen.queryByTestId('handovers-list-table')).toBeNull();
    });

    it('opens the hand-over the address names beside the list, drawn by HandoverDetails', () => {
        setPageUrl('/handovers?handover=31');
        renderPageWith(Page, pageData({ opened: handoverDetailsOf() }));

        expect(screen.getByTestId('handovers-details-panel')).toHaveTextContent('Передача HO-2026-0042');
        expect(screen.getByTestId('handover-details')).toBeInTheDocument();
    });

    it('words the refusal when the list cannot be read', () => {
        renderPageWith(Page, pageData({ refusal: 'internal' }));

        expect(screen.getByTestId('handovers-refusal')).toHaveTextContent('В программе wyo произошла ошибка.');
        expect(screen.getByTestId('handovers-retry')).toBeInTheDocument();
    });
});
