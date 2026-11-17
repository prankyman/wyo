import { render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import SubmitWithAnswer from './SubmitWithAnswer.svelte';

vi.mock('$lib/i18n/i18n.js', async () => (await import('$test/support/i18n')).createI18nMock({ locale: 'ru' }));
vi.mock('$lib/i18n/i18n', async () => (await import('$test/support/i18n')).createI18nMock({ locale: 'ru' }));

/** A submit button with the refusal of its form under it. */

const draw = (props: Record<string, unknown>) =>
    render(SubmitWithAnswer, {
        props: { lang: 'ru', label: 'Далее', dataTest: 'step-next', answer: undefined, ...props }
    });

describe('SubmitWithAnswer', () => {
    it('draws a submit button with the given words and name, and no line while the form has not answered', () => {
        draw({});
        const button = screen.getByTestId('step-next');
        expect(button).toHaveTextContent('Далее');
        expect(button).toHaveAttribute('type', 'submit');
        expect(screen.queryByRole('alert')).toBeNull();
        expect(screen.queryByRole('status')).toBeNull();
    });

    it('words the refusal from the answer’s message key under the button, named after the button by default', () => {
        draw({ answer: { error: 'errors.not_found' } });
        expect(screen.getByTestId('step-next-error')).toHaveTextContent('Такого адреса в программе wyo нет.');
    });

    it('names the refusal after answerTest when it is given, and draws no line for a success', () => {
        const { unmount } = draw({ answer: { error: 'errors.not_found' }, answerTest: 'step' });
        expect(screen.getByTestId('step-error')).toBeInTheDocument();
        expect(screen.queryByTestId('step-next-error')).toBeNull();
        unmount();

        draw({ answer: { saved: 'saved' }, answerTest: 'step' });
        expect(screen.queryByTestId('step-error')).toBeNull();
        expect(screen.queryByTestId('step-saved')).toBeNull();
    });
});
