import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { TInstallation } from '$lib/overview/installation-cards';

vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { post: vi.fn() }
}));

import { api } from '$lib/api/api';
import UpdatesStatus from './UpdatesStatus.svelte';

const post = vi.mocked(api.post);
const UPDATES: TInstallation['updates'] = { auto: true, requested: false };

afterEach(() => {
    post.mockReset();
});

describe('UpdatesStatus', () => {
    it('says the installed version and the last look, or that no look was made yet', () => {
        const { unmount } = render(UpdatesStatus, {
            props: {
                lang: 'ru',
                version: '1.2.3',
                updates: { ...UPDATES, lastCheckAt: '2026-11-17T12:00:00' },
                onRequested: vi.fn()
            }
        });
        expect(screen.getByTestId('updates-version')).toHaveTextContent('Установлена версия: 1.2.3.');
        expect(screen.getByTestId('updates-last-look')).toHaveTextContent('Последняя проверка: 17 нояб. 2026 г.');
        unmount();

        render(UpdatesStatus, { props: { lang: 'ru', version: '1.2.3', updates: UPDATES, onRequested: vi.fn() } });
        expect(screen.getByTestId('updates-last-look')).toHaveTextContent('Проверка ещё не выполнялась.');
        expect(screen.queryByTestId('updates-waiting')).toBeNull();
    });

    it('names a newer version that waits', () => {
        render(UpdatesStatus, {
            props: {
                lang: 'ru',
                version: '1.2.3',
                updates: { ...UPDATES, waitingVersion: '1.3.0' },
                onRequested: vi.fn()
            }
        });

        expect(screen.getByTestId('updates-waiting')).toHaveTextContent('Ждёт установки версия: 1.3.0.');
    });

    it.each([
        [
            'updates-check',
            '/api/v1/updates/check',
            'Запрос принят. Проверка занимает до минуты; обновите страницу, чтобы увидеть результат.'
        ],
        [
            'updates-install',
            '/api/v1/updates/install',
            'Запрос принят. Установка занимает около двух минут: wyo перезапустится, и страница на это время может не отвечать.'
        ]
    ])('presses «%s»: sends the request, says what happens next and tells the page', async (testId, path, answer) => {
        post.mockResolvedValue({ ok: true, data: { requested: 'check' } });
        const onRequested = vi.fn();
        render(UpdatesStatus, { props: { lang: 'ru', version: '1.2.3', updates: UPDATES, onRequested } });

        await fireEvent.click(screen.getByTestId(testId));

        await waitFor(() => expect(onRequested).toHaveBeenCalledOnce());
        expect(post).toHaveBeenCalledExactlyOnceWith(path, {});
        expect(screen.getByTestId(`${testId}-answer-saved`)).toHaveTextContent(answer);
    });

    it('words a refusal under the button that was pressed and does not tell the page', async () => {
        post.mockResolvedValue({ ok: false, status: 500, refusal: { code: 'update_request_failed' } });
        const onRequested = vi.fn();
        render(UpdatesStatus, { props: { lang: 'ru', version: '1.2.3', updates: UPDATES, onRequested } });

        await fireEvent.click(screen.getByTestId('updates-check'));

        await waitFor(() => expect(screen.getByTestId('updates-check-answer-error')).toBeInTheDocument());
        expect(screen.getByTestId('updates-check-answer-error')).toHaveTextContent(
            'Не удалось передать запрос обновлятору.'
        );
        expect(onRequested).not.toHaveBeenCalled();
    });

    it('cannot be pressed again while the updater has not answered the earlier request', () => {
        render(UpdatesStatus, {
            props: { lang: 'ru', version: '1.2.3', updates: { ...UPDATES, requested: true }, onRequested: vi.fn() }
        });

        expect(screen.getByTestId('updates-check')).toBeDisabled();
        expect(screen.getByTestId('updates-install')).toBeDisabled();
        expect(screen.getByTestId('updates-requested')).toHaveTextContent('Запрос уже принят');
    });
});
