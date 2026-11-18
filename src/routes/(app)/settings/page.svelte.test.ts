import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { get: vi.fn(), post: vi.fn(), put: vi.fn() }
}));

import { api } from '$lib/api/api';
import type { TInstallation } from '$lib/overview/installation-cards';
import Page from './+page.svelte';

const get = vi.mocked(api.get);
const post = vi.mocked(api.post);

const SETTINGS = {
    groups: [
        { id: 'updates', settings: [{ key: 'updates.auto', kind: 'boolean', value: true, defaultValue: true }] },
        {
            id: 'security',
            settings: [{ key: 'security.idleMinutes', kind: 'integer', value: 60, defaultValue: 60, min: 5, max: 720 }]
        }
    ]
};
const INSTALLATION: TInstallation = {
    version: '1.2.3',
    updates: { auto: true, requested: false },
    certificate: { mode: 'proxy', state: 'not_applicable' },
    backups: { state: 'ok' }
};
const BACKUPS = { backups: [{ name: 'wyo-2026-11-17.db', size: 1_048_576, createdAt: '2026-11-17T12:00:00' }] };

/** Answers each read of the page: the settings, the installation and the copies. */
function answer(installation: TInstallation = INSTALLATION) {
    get.mockImplementation(async (path: string) => {
        if (path === '/api/v1/settings') return { ok: true, data: SETTINGS };
        if (path === '/api/v1/installation') return { ok: true, data: installation };
        return { ok: true, data: BACKUPS };
    });
}

afterEach(() => {
    get.mockReset();
    post.mockReset();
});

describe('«Настройки»', () => {
    it('draws its heading with one «?» and three folded sections: updates, copies, security', async () => {
        answer();

        render(Page);

        expect(screen.getByTestId('settings-heading')).toHaveTextContent('Настройки');
        expect(screen.getAllByTestId('settings-help-button')).toHaveLength(1);
        expect(screen.getByTestId('settings-loading')).toBeInTheDocument();
        const sections = await screen.findByTestId('settings-sections');
        const headings = within(sections)
            .getAllByRole('heading', { level: 2 })
            .map((heading) => heading.textContent?.trim());
        expect(headings).toEqual(['Обновления', 'Резервные копии', 'Вход и безопасность']);
        expect((screen.getByTestId('settings-updates') as HTMLDetailsElement).open).toBe(false);
        expect(get.mock.calls.map((call) => call[0]).sort()).toEqual([
            '/api/v1/backups',
            '/api/v1/installation',
            '/api/v1/settings'
        ]);
    });

    it('hands «Обновления» the version and the copies their list', async () => {
        answer();
        render(Page);

        await screen.findByTestId('settings-sections');

        expect(screen.getByTestId('updates-version')).toHaveTextContent('Установлена версия: 1.2.3.');
        expect(screen.getAllByTestId('backup-row')).toHaveLength(1);
    });

    it('reads the installation again after a request to the updater, and shows the copy it has just made', async () => {
        answer();
        post.mockImplementation(async (path: string) =>
            path === '/api/v1/backups'
                ? { ok: true, data: { name: 'wyo-2026-11-18.db', size: 2_097_152, createdAt: '2026-11-18T12:00:00' } }
                : { ok: true, data: { requested: 'check' } }
        );
        render(Page);
        await screen.findByTestId('settings-sections');

        answer({ ...INSTALLATION, updates: { auto: true, requested: true } });
        await fireEvent.click(screen.getByTestId('updates-check'));
        await waitFor(() => expect(screen.getByTestId('updates-requested')).toBeInTheDocument());

        await fireEvent.click(screen.getByTestId('backups-create'));
        await waitFor(() => expect(screen.getAllByTestId('backup-row')).toHaveLength(2));
        expect(screen.getAllByTestId('backup-row')[0]).toHaveTextContent('wyo-2026-11-18.db');
    });

    it('words a refusal of any of the three reads and reads again on «Повторить»', async () => {
        get.mockResolvedValue({ ok: false, status: 500, refusal: { code: 'internal' } });
        render(Page);

        expect(await screen.findByTestId('settings-refusal')).toHaveTextContent('В программе wyo произошла ошибка.');
        answer();
        await fireEvent.click(screen.getByTestId('settings-retry'));

        expect(await screen.findByTestId('settings-sections')).toBeInTheDocument();
        expect(screen.queryByTestId('settings-refusal')).toBeNull();
    });
});
