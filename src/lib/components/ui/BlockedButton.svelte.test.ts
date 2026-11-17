import { fireEvent, render, screen } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { describe, expect, it, vi } from 'vitest';
import BlockedButton from './BlockedButton.svelte';

vi.mock('$lib/i18n/i18n.js', async () => (await import('$test/support/i18n')).createI18nMock({ locale: 'ru' }));
vi.mock('$lib/i18n/i18n', async () => (await import('$test/support/i18n')).createI18nMock({ locale: 'ru' }));

/** A button that stays in its place, greyed out, with the reason under it. */

const children = createRawSnippet(() => ({ render: () => '<span>Взять</span>' }));
const twoParagraphs = createRawSnippet(() => ({ render: () => '<div><p>Первый</p><p>Второй</p></div>' }));

const draw = (props: Record<string, unknown> = {}) =>
    render(BlockedButton, { props: { dataTest: 'take', reasonTestId: 'take-reason', children, ...props } });

describe('BlockedButton', () => {
    it('without a reason: an ordinary button that passes the press on, and no reason box', async () => {
        const onclick = vi.fn();
        draw({ onclick });
        const button = screen.getByTestId('take');
        expect(button).toHaveTextContent('Взять');
        expect(button).not.toBeDisabled();
        expect(screen.queryByTestId('take-reason')).toBeNull();
        await fireEvent.click(button);
        expect(onclick).toHaveBeenCalledOnce();
    });

    it('an empty reason counts as none', () => {
        draw({ reason: '' });
        expect(screen.getByTestId('take')).not.toBeDisabled();
        expect(screen.queryByTestId('take-reason')).toBeNull();
    });

    it('with a reason: greyed out, the reason under it as a polite status', () => {
        draw({ reason: 'Сначала завершите шаг № 4 — у него выше приоритет', reasonTour: 'take-tour' });
        const button = screen.getByTestId('take');
        expect(button).toBeDisabled();
        const box = screen.getByTestId('take-reason');
        expect(box).toHaveTextContent('Сначала завершите шаг № 4 — у него выше приоритет');
        expect(box).toHaveAttribute('role', 'status');
        expect(box).toHaveAttribute('data-tour', 'take-tour');
    });

    it('a reason as markup blocks it too and wins over the text', () => {
        draw({ reason: 'текст', reasonContent: twoParagraphs, reasonClass: 'flex flex-col gap-2' });
        expect(screen.getByTestId('take')).toBeDisabled();
        const box = screen.getByTestId('take-reason');
        expect(box).toHaveTextContent('ПервыйВторой');
        expect(box).not.toHaveTextContent('текст');
        expect(box.className).toContain('flex-col');
    });

    it('with help: the «?» stands beside it, also while blocked', () => {
        draw({ help: 'Что делает кнопка', helpTestId: 'take-help', helpLabel: 'Взять', reason: 'нельзя' });
        expect(screen.getByTestId('take')).toBeDisabled();
        expect(screen.getByTestId('take-help-button')).toHaveAttribute('aria-label', 'Взять');
    });

    it('stays disabled when the caller disables it without a reason', () => {
        draw({ disabled: true, blockClass: 'mt-2' });
        expect(screen.getByTestId('take')).toBeDisabled();
        expect(screen.queryByTestId('take-reason')).toBeNull();
        expect(screen.getByTestId('take').closest('div.mt-2')).not.toBeNull();
    });
});
