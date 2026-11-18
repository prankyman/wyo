import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { get: vi.fn() }
}));

import { api } from '$lib/api/api';
import type { TInstallation } from '$lib/overview/installation-cards';
import Page from './+page.svelte';

const get = vi.mocked(api.get);
const INSTALLATION: TInstallation = {
    version: '1.2.3',
    updates: { auto: true, requested: false },
    certificate: { mode: 'proxy', state: 'not_applicable' },
    backups: { state: 'none' }
};

afterEach(() => {
    get.mockReset();
});

describe('«Обзор»', () => {
    it('draws its heading with one «?», the strip of the four steps with «Сейчас», and what to do next', async () => {
        get.mockResolvedValue({ ok: true, data: INSTALLATION });

        render(Page);

        expect(screen.getByTestId('overview-heading')).toHaveTextContent('Обзор');
        expect(screen.getAllByTestId('overview-help-button')).toHaveLength(1);
        const strip = screen.getByTestId('overview-steps');
        expect(within(strip).getAllByRole('listitem')).toHaveLength(4);
        expect(screen.getByTestId('overview-steps-now')).toHaveTextContent('Сейчас: шаг 2 — ключи маркетплейсов.');
        expect(screen.getByTestId('overview-next')).toHaveTextContent('Следующий шаг — ввести ключи маркетплейсов');
        expect(await screen.findByTestId('overview-cards')).toBeInTheDocument();
    });

    it('asks for the installation once and draws a card for the version, the certificate and the last copy', async () => {
        get.mockResolvedValue({ ok: true, data: INSTALLATION });

        render(Page);

        expect(screen.getByTestId('overview-loading')).toBeInTheDocument();
        expect(await screen.findByTestId('overview-card-version')).toHaveTextContent('Установлена версия 1.2.3.');
        expect(screen.getByTestId('overview-card-certificate')).toHaveTextContent('Сертификатом занимается веб-сервер');
        expect(screen.getByTestId('overview-card-backups')).toHaveTextContent('Копий ещё нет');
        expect(get).toHaveBeenCalledExactlyOnceWith('/api/v1/installation');
        expect(screen.queryByTestId('overview-loading')).toBeNull();
    });

    it('words a refusal and reads the installation again on «Повторить»', async () => {
        get.mockResolvedValue({ ok: false, status: 500, refusal: { code: 'internal' } });
        render(Page);

        expect(await screen.findByTestId('overview-refusal')).toHaveTextContent('В программе wyo произошла ошибка.');
        get.mockResolvedValue({ ok: true, data: INSTALLATION });
        await fireEvent.click(screen.getByTestId('overview-retry'));

        await waitFor(() => expect(screen.getByTestId('overview-cards')).toBeInTheDocument());
        expect(screen.queryByTestId('overview-refusal')).toBeNull();
    });
});
