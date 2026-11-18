import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import NewPasswordFields from './NewPasswordFields.svelte';
import type { IPasswordBoxIds } from './types';

vi.mock('$lib/i18n/i18n.js', async () => (await import('$test/support/i18n')).createI18nMock({ locale: 'ru' }));

const box = (name: string): IPasswordBoxIds => ({
    id: `${name}-id`,
    test: `${name}-box`,
    errorTest: `${name}-refusal`
});
const ids = { current: box('cur'), next: box('new'), confirm: box('rep') };
const REFUSALS = {
    current: ['errors.invalid'],
    next: ['errors.invalid'],
    confirm: ['errors.invalid']
};

describe('NewPasswordFields', () => {
    it('draws the new password and its confirmation, each under its label with its own id and test id', () => {
        const { container } = render(NewPasswordFields, { props: { lang: 'ru', ids } });

        expect(container.querySelectorAll('input[type="password"]')).toHaveLength(2);
        const fresh = screen.getByTestId('new-box');
        expect(fresh).toHaveAttribute('id', 'new-id');
        expect(fresh).toHaveAttribute('name', 'newPassword');
        expect(fresh).toHaveAttribute('autocomplete', 'new-password');
        expect(fresh).toBeRequired();
        expect(container.querySelector('label[for="new-id"]')).not.toBeNull();
        const confirm = screen.getByTestId('rep-box');
        expect(confirm).toHaveAttribute('name', 'confirmNewPassword');
        expect(confirm).toHaveAttribute('autocomplete', 'new-password');
        expect(screen.queryByTestId('cur-box')).toBeNull();
    });

    it('draws the current-password box first when asked', () => {
        const { container } = render(NewPasswordFields, { props: { lang: 'ru', ids, withCurrent: true } });

        const boxes = [...container.querySelectorAll('input')];
        expect(boxes.map((input) => input.name)).toEqual(['currentPassword', 'newPassword', 'confirmNewPassword']);
        expect(screen.getByTestId('cur-box')).toHaveAttribute('autocomplete', 'current-password');
        expect(screen.getByTestId('cur-box')).toHaveAttribute('id', 'cur-id');
    });

    it('draws no current-password box when asked for it but given no ids for it', () => {
        const { container } = render(NewPasswordFields, {
            props: { lang: 'ru', ids: { next: ids.next, confirm: ids.confirm }, withCurrent: true }
        });

        expect(container.querySelectorAll('input')).toHaveLength(2);
    });

    it.each([
        ['currentPassword', 'cur-box'],
        ['newPassword', 'new-box'],
        ['confirmNewPassword', 'rep-box']
    ])('tells the caller that %s was left', async (name, testId) => {
        const validate = vi.fn();
        render(NewPasswordFields, { props: { lang: 'ru', ids, withCurrent: true, validate } });

        await fireEvent.focusOut(screen.getByTestId(testId));

        expect(validate).toHaveBeenCalledExactlyOnceWith(name);
    });

    it('typing changes the bound values', async () => {
        render(NewPasswordFields, { props: { lang: 'ru', ids, withCurrent: true } });

        await fireEvent.input(screen.getByTestId('new-box'), { target: { value: 'Abc' } });

        expect(screen.getByTestId('new-box')).toHaveValue('Abc');
    });

    it('line look: keeps a line free under each box and fills it with the refusal', async () => {
        const { container, rerender } = render(NewPasswordFields, { props: { lang: 'ru', ids, withCurrent: true } });
        expect(container.querySelectorAll('span.min-h-4')).toHaveLength(3);
        expect(screen.queryByTestId('new-refusal')).toBeNull();

        await rerender({ lang: 'ru', ids, withCurrent: true, errors: REFUSALS });

        for (const name of ['cur', 'new', 'rep']) {
            expect(screen.getByTestId(`${name}-refusal`)).toHaveTextContent('Значение не подходит.');
        }
    });

    it('text look: draws nothing without a refusal and the bare message with one', async () => {
        const { container, rerender } = render(NewPasswordFields, { props: { lang: 'ru', ids, errorLook: 'text' } });
        expect(container.querySelectorAll('span.min-h-4')).toHaveLength(0);
        expect(screen.queryByTestId('new-refusal')).toBeNull();

        await rerender({ lang: 'ru', ids, errorLook: 'text', errors: REFUSALS });

        expect(screen.getByTestId('new-refusal')).toHaveClass('text-xs', 'text-stop-700');
        expect(screen.getByTestId('rep-refusal')).toHaveTextContent('Значение не подходит.');
    });

    it('field look: hands the refusal to the field, which draws it as an alert', async () => {
        const { container, rerender } = render(NewPasswordFields, { props: { lang: 'ru', ids, errorLook: 'field' } });
        expect(screen.queryByTestId('new-refusal')).toBeNull();

        await rerender({ lang: 'ru', ids, errorLook: 'field', errors: REFUSALS });

        expect(screen.getByTestId('new-refusal')).toHaveAttribute('role', 'alert');
        expect(container.querySelectorAll('span.min-h-4')).toHaveLength(0);
    });

    it('puts a «?» beside each box that has a help text, and none beside a box that has not', () => {
        render(NewPasswordFields, {
            props: {
                lang: 'ru',
                ids,
                withCurrent: true,
                helps: { next: 'Как придумать пароль.', current: 'Какой пароль нужен.' }
            }
        });

        expect(screen.getAllByTestId('field-help-button')).toHaveLength(2);
        expect(screen.getAllByTestId('field-help-button').map((button) => button.getAttribute('aria-label'))).toEqual([
            'Какой пароль нужен.',
            'Как придумать пароль.'
        ]);
    });
});
