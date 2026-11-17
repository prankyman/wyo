import { fireEvent, render, screen } from '@testing-library/svelte';
import { createRawSnippet, tick } from 'svelte';
import { describe, expect, it, vi } from 'vitest';
import DisclosureWithHelp from './DisclosureWithHelp.svelte';

vi.mock('$lib/i18n/i18n.js', async () => (await import('$test/support/i18n')).createI18nMock({ locale: 'ru' }));
vi.mock('$lib/i18n/i18n', async () => (await import('$test/support/i18n')).createI18nMock({ locale: 'ru' }));

/** A decision that opens a form, with its «?» beside the opening button. */

const children = createRawSnippet(() => ({ render: () => '<p data-test="inside">Вопрос и форма</p>' }));

const draw = (props: Record<string, unknown> = {}) =>
    render(DisclosureWithHelp, {
        props: {
            label: 'Отклонить',
            lang: 'ru',
            help: 'Что делает отклонение',
            helpTestId: 'decision-help',
            openerTestId: 'decision-open',
            testId: 'decision-panel',
            children,
            ...props
        }
    });

describe('DisclosureWithHelp', () => {
    it('draws the opening button with the «?» beside it, and the form only once the button is pressed', async () => {
        draw();
        expect(screen.getByTestId('decision-open')).toHaveTextContent('Отклонить');
        expect(screen.getByTestId('decision-help-button')).toHaveAttribute('aria-label', 'Отклонить');
        expect(screen.queryByTestId('inside')).toBeNull();

        await fireEvent.click(screen.getByTestId('decision-open'));
        await tick();
        expect(screen.getByTestId('decision-panel')).toContainElement(screen.getByTestId('inside'));
        expect(screen.getByTestId('decision-help-button')).toBeInTheDocument();
    });

    it('puts the tour’s anchor on the button and its «?» together, and none when there is no tour', () => {
        const { unmount } = draw({ tour: 'decision-tour' });
        const anchor = document.querySelector('[data-tour="decision-tour"]');
        expect(anchor).toContainElement(screen.getByTestId('decision-open'));
        expect(anchor).toContainElement(screen.getByTestId('decision-help-button'));
        unmount();

        draw();
        expect(document.querySelector('[data-tour]')).toBeNull();
    });
});
