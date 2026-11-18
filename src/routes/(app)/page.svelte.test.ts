import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { get: vi.fn() }
}));

import { api, type TApiResult } from '$lib/api/api';
import type { TInstallation } from '$lib/overview/installation-cards';
import type { TMarketplacesOverview } from '$lib/overview/marketplace-cards';
import Page from './+page.svelte';

const get = vi.mocked(api.get);
const INSTALLATION: TInstallation = {
    version: '1.2.3',
    updates: { auto: true, requested: false },
    certificate: { mode: 'proxy', state: 'not_applicable' },
    backups: { state: 'none' }
};
const NO_ACCOUNTS: TMarketplacesOverview = {
    accounts: 0,
    attention: 0,
    newOrders: 0,
    assembling: 0,
    overdue: 0,
    unknownStates: 0
};

/** The wyo server's answers to the page's two calls: the installation and the marketplaces' counts. */
function answers(
    installation: TApiResult<unknown> = { ok: true, data: INSTALLATION },
    marketplaces: TApiResult<unknown> = { ok: true, data: NO_ACCOUNTS }
): void {
    get.mockImplementation(
        async (path: string) => (path === '/api/v1/installation' ? installation : marketplaces) as never
    );
}

afterEach(() => {
    get.mockReset();
});

describe('«Обзор»', () => {
    it('draws its heading with one «?», the strip of the four steps with «Сейчас», and what to do next', async () => {
        answers();

        render(Page);

        expect(screen.getByTestId('overview-heading')).toHaveTextContent('Обзор');
        expect(screen.getAllByTestId('overview-help-button')).toHaveLength(1);
        const strip = screen.getByTestId('overview-steps');
        expect(within(strip).getAllByRole('listitem')).toHaveLength(4);
        expect(screen.getByTestId('overview-steps-now')).toHaveTextContent('Сейчас: шаг 2 — ключи маркетплейсов.');
        expect(screen.getByTestId('overview-next')).toHaveTextContent('Следующий шаг — ввести ключи маркетплейсов');
        expect(await screen.findByTestId('overview-cards')).toBeInTheDocument();
    });

    it('asks for the installation and the marketplaces once each, and draws a card for each part they tell', async () => {
        answers();

        render(Page);

        expect(screen.getByTestId('overview-loading')).toBeInTheDocument();
        expect(await screen.findByTestId('overview-card-version')).toHaveTextContent('Установлена версия 1.2.3.');
        expect(screen.getByTestId('overview-card-certificate')).toHaveTextContent('Сертификатом занимается веб-сервер');
        expect(screen.getByTestId('overview-card-backups')).toHaveTextContent('Копий ещё нет');
        expect(await screen.findByTestId('overview-card-accounts')).toHaveTextContent('Нет аккаунтов');
        expect(screen.getByTestId('overview-card-orders')).toBeInTheDocument();
        expect(screen.getByTestId('overview-card-reading')).toBeInTheDocument();
        expect(get).toHaveBeenCalledWith('/api/v1/installation');
        expect(get).toHaveBeenCalledWith('/api/v1/overview/marketplaces');
        expect(get).toHaveBeenCalledTimes(2);
        expect(screen.queryByTestId('overview-loading')).toBeNull();
    });

    it('moves the strip of steps on to the orders once an account is added', async () => {
        answers(undefined, { ok: true, data: { ...NO_ACCOUNTS, accounts: 1 } });

        render(Page);

        await waitFor(() =>
            expect(screen.getByTestId('overview-steps-now')).toHaveTextContent('Сейчас: шаг 3 — заказы.')
        );
        expect(screen.getByTestId('overview-next')).toHaveTextContent('следить за заказами в разделе «Заказы»');
    });

    it('keeps the installation’s cards and words the refusal when the marketplaces cannot be read', async () => {
        answers(undefined, { ok: false, status: 500, refusal: { code: 'internal' } });

        render(Page);

        expect(await screen.findByTestId('overview-marketplaces-refusal')).toHaveTextContent('произошла ошибка');
        expect(screen.getByTestId('overview-card-version')).toBeInTheDocument();
        expect(screen.queryByTestId('overview-card-accounts')).toBeNull();
    });

    it('words a refusal and reads the installation again on «Повторить»', async () => {
        answers({ ok: false, status: 500, refusal: { code: 'internal' } });
        render(Page);

        expect(await screen.findByTestId('overview-refusal')).toHaveTextContent('В программе wyo произошла ошибка.');
        answers();
        await fireEvent.click(screen.getByTestId('overview-retry'));

        await waitFor(() => expect(screen.getByTestId('overview-cards')).toBeInTheDocument());
        expect(screen.queryByTestId('overview-refusal')).toBeNull();
    });
});
