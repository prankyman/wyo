import { render, screen, within } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import { MARKETPLACE_CODE } from '$test/support/marketplaces';
import KeyHelpSteps from './KeyHelpSteps.svelte';

/** How a seller makes the key a marketplace's adapter asks for: a folded section with the strip of steps and each step in full. */
describe('KeyHelpSteps', () => {
    it('draws the adapter’s steps as a strip and as a numbered list, with the introduction, in a section open on arrival', () => {
        render(KeyHelpSteps, { props: { lang: 'ru', code: MARKETPLACE_CODE, id: 'key-help', open: true } });

        const section = screen.getByTestId('key-help') as HTMLDetailsElement;
        expect(section.open).toBe(true);
        expect(screen.getByTestId('key-help-heading')).toHaveTextContent('Как получить ключ');
        expect(screen.getByTestId('key-help-help-button')).toBeInTheDocument();
        expect(screen.getByTestId('key-help-intro')).toHaveTextContent('действует 180 дней');
        expect(within(screen.getByTestId('key-help-strip')).getAllByRole('listitem')).toHaveLength(5);
        const steps = within(screen.getByTestId('key-help-list')).getAllByRole('listitem');
        expect(steps).toHaveLength(5);
        expect(steps[0]).toHaveTextContent('Интеграции по API');
        expect(steps[0]).toHaveTextContent('/api-integrations');
        expect(steps[4]).toHaveTextContent('Скопировать и закрыть');
    });

    it('draws nothing for an adapter whose steps the interface has no texts for', () => {
        const { container } = render(KeyHelpSteps, { props: { lang: 'ru', code: 'later-one', id: 'key-help' } });

        expect(container.querySelector('[data-test="key-help"]')).toBeNull();
    });
});
