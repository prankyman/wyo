import { fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('$app/state', async () => {
    const state = await import('$test/support/page-state.svelte');
    return { page: state.page, navigating: state.navigating };
});

import { resetPage, setPageStatus, setPageUrl } from '$test/support/page-state.svelte';
import ErrorPage from './+error.svelte';

/** status, big figure above the heading, heading, explanation. */
const statusCases: [number, string, string, string][] = [
    [
        404,
        '404',
        'Страница не найдена',
        'Такой страницы здесь нет. Возможно, в адресе опечатка. Вернитесь на главную страницу и найдите нужное в меню.'
    ],
    [
        500,
        'Ошибка',
        'Что-то пошло не так',
        'Страница не открылась из-за ошибки в программе. Обновите страницу; если ошибка повторяется, откройте главную страницу и повторите действие.'
    ]
];

describe('+error.svelte', () => {
    afterEach(() => resetPage());

    it.each(statusCases)(
        'status %i gets its own headline, heading and explanation',
        (status, headline, title, body) => {
            setPageStatus(status);

            const { container } = render(ErrorPage);

            expect(container.querySelector('[data-test="error-status"]')).toHaveTextContent(headline);
            expect(container.querySelector('[data-test="error-title"]')).toHaveTextContent(title);
            expect(container.querySelector('[data-test="error-body"]')).toHaveTextContent(body);
        }
    );

    it('leads home, in the language of the page', () => {
        setPageStatus(404);
        setPageUrl('/settings/nothing');

        const { container } = render(ErrorPage);

        expect(container.querySelector('[data-test="error-home"]')).toHaveAttribute('href', '/');
    });

    it('goes back one step in history when the back button is pressed', async () => {
        const backSpy = vi.spyOn(window.history, 'back').mockImplementation(() => {});
        setPageStatus(500);
        render(ErrorPage);

        await fireEvent.click(screen.getByTestId('error-back'));

        expect(backSpy).toHaveBeenCalledTimes(1);
        backSpy.mockRestore();
    });
});
