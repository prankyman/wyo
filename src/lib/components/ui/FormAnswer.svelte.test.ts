import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import FormAnswer from './FormAnswer.svelte';

/** The answer of one form under its own button. */
describe('FormAnswer', () => {
    it.each([
        ['a success as a status', { saved: 'Сохранено' }, 'settings-saved', 'status', 'Сохранено'],
        [
            'a refusal as an alert, even beside a success',
            { saved: 'Сохранено', error: 'Нельзя' },
            'settings-error',
            'alert',
            'Нельзя'
        ]
    ])('draws %s', (_name, props, testId, role, text) => {
        render(FormAnswer, { ...props, dataTest: 'settings', errorTour: 'settings-error' });

        const line = screen.getByTestId(testId);
        expect(line).toHaveAttribute('role', role);
        expect(line).toHaveTextContent(text);
        expect(screen.getAllByRole(role)).toHaveLength(1);
    });

    it.each([
        ['under its button, 8 px below it', false, true],
        ['as a block of its page’s column, spaced by the column alone', true, false]
    ])('stands %s', (_name, standalone, spaced) => {
        render(FormAnswer, { error: 'Нельзя', dataTest: 'settings', standalone });

        const line = screen.getByTestId('settings-error');
        if (spaced) expect(line).toHaveClass('mt-2');
        else expect(line.className).not.toMatch(/(^|\s)-?m[bty]-/);
    });

    it('draws nothing before the form answered', () => {
        const { container } = render(FormAnswer, { dataTest: 'settings' });

        expect(container.querySelector('p')).toBeNull();
    });
});
