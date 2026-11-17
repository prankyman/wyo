import { fireEvent, render, screen } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { describe, expect, it, vi } from 'vitest';
import ButtonWithHelp from './ButtonWithHelp.svelte';

vi.mock('$lib/i18n/i18n.js', async () => (await import('$test/support/i18n')).createI18nMock({ locale: 'ru' }));
vi.mock('$lib/i18n/i18n', async () => (await import('$test/support/i18n')).createI18nMock({ locale: 'ru' }));

/** A button with its «?» beside it. */

const children = createRawSnippet(() => ({ render: () => '<span>Новая запись</span>' }));

const draw = (props: Record<string, unknown> = {}) =>
    render(ButtonWithHelp, {
        props: {
            help: 'Что делает кнопка',
            helpTestId: 'record-help',
            helpLabel: 'Новая запись',
            dataTest: 'pallet',
            children,
            ...props
        }
    });

describe('ButtonWithHelp', () => {
    it('draws the button and its «?» on one line, the «?» named after the button, and passes the press on', async () => {
        const onclick = vi.fn();
        draw({ onclick });
        const button = screen.getByTestId('pallet');
        expect(button).toHaveTextContent('Новая запись');
        expect(button.parentElement).toContainElement(screen.getByTestId('record-help-button'));
        expect(screen.getByTestId('record-help-button')).toHaveAttribute('aria-label', 'Новая запись');

        await fireEvent.click(button);
        expect(onclick).toHaveBeenCalledOnce();
    });

    it('shows the help text when the «?» is pressed, and not before', async () => {
        draw();
        expect(screen.queryByTestId('record-help-tooltip')).toBeNull();
        await fireEvent.click(screen.getByTestId('record-help-button'));
        expect(screen.getByTestId('record-help-tooltip')).toHaveTextContent('Что делает кнопка');
    });

    it('keeps the button’s own look: variant, size and pending are the Button’s', () => {
        draw({ variant: 'secondary', size: 'lg', pending: true });
        const button = screen.getByTestId('pallet');
        expect(button).toHaveAttribute('aria-busy', 'true');
        expect(button.className).toContain('min-h-14');
    });

    it('lets a full-width button take the place the «?» leaves, and puts the row’s classes on the row', () => {
        draw({ fullWidth: true, rowClass: 'col-span-2' });
        const button = screen.getByTestId('pallet');
        expect(button).toHaveClass('w-full', 'min-w-0', 'flex-1');
        expect(button.parentElement).toHaveClass('flex', 'col-span-2');
    });

    it('does not stretch the button without fullWidth', () => {
        draw();
        expect(screen.getByTestId('pallet')).not.toHaveClass('w-full');
    });
});
