import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { components } from '$lib/api/wyo-api';

vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { post: vi.fn() }
}));

import { api } from '$lib/api/api';
import BackupsSection from './BackupsSection.svelte';

type TBackup = components['schemas']['Backup'];

const post = vi.mocked(api.post);
const COPY = (name: string, size: number, createdAt: string): TBackup => ({ name, size, createdAt });
const BACKUPS = [
    COPY('wyo-2026-11-17.db', 1_572_864, '2026-11-17T12:00:00'),
    COPY('wyo-2026-11-16.db', 1_048_576, '2026-11-16T12:00:00')
];

afterEach(() => {
    post.mockReset();
});

describe('BackupsSection', () => {
    it('is a folded section that lists the copies newest first with their day and size, and warns about the key file', () => {
        render(BackupsSection, { props: { lang: 'ru', backups: BACKUPS, onCreated: vi.fn() } });

        const section = screen.getByTestId('settings-backups') as HTMLDetailsElement;
        expect(section.open).toBe(false);
        expect(screen.getByTestId('settings-backups-heading')).toHaveTextContent('Резервные копии');
        const rows = screen.getAllByTestId('backup-row');
        expect(rows.map((row) => row.textContent)).toEqual([
            'wyo-2026-11-17.db — 17 нояб. 2026 г., 1,5 МБ',
            'wyo-2026-11-16.db — 16 нояб. 2026 г., 1 МБ'
        ]);
        expect(within(screen.getByTestId('backups-key-file')).getByText(/secret\.key/)).toBeInTheDocument();
        expect(screen.getByTestId('backups-key-file')).toHaveTextContent(
            'Файл secret.key не входит в копию. Без него ключи маркетплейсов, сохранённые в wyo, нельзя прочитать из восстановленной копии, и их придётся ввести заново.'
        );
    });

    it('says what to do when there is no copy yet', () => {
        render(BackupsSection, { props: { lang: 'ru', backups: [], onCreated: vi.fn() } });

        expect(screen.getByTestId('backups-empty')).toHaveTextContent('Копий ещё нет.');
        expect(screen.queryByTestId('backup-row')).toBeNull();
    });

    it('makes a copy on a press, tells the page and says so under the button', async () => {
        const made = COPY('wyo-2026-11-18.db', 2_097_152, '2026-11-18T12:00:00');
        post.mockResolvedValue({ ok: true, data: made });
        const onCreated = vi.fn();
        render(BackupsSection, { props: { lang: 'ru', backups: BACKUPS, onCreated } });

        await fireEvent.click(screen.getByTestId('backups-create'));

        await waitFor(() => expect(onCreated).toHaveBeenCalledExactlyOnceWith(made));
        expect(post).toHaveBeenCalledExactlyOnceWith('/api/v1/backups', {});
        expect(screen.getByTestId('backups-create-answer-saved')).toHaveTextContent('Копия сделана.');
    });

    it('words a refusal under the button and tells the page nothing', async () => {
        post.mockResolvedValue({ ok: false, status: 500, refusal: { code: 'backup_failed' } });
        const onCreated = vi.fn();
        render(BackupsSection, { props: { lang: 'ru', backups: BACKUPS, onCreated } });

        await fireEvent.click(screen.getByTestId('backups-create'));

        await waitFor(() => expect(screen.getByTestId('backups-create-answer-error')).toBeInTheDocument());
        expect(screen.getByTestId('backups-create-answer-error')).toHaveTextContent('Копия не получилась.');
        expect(onCreated).not.toHaveBeenCalled();
    });
});
