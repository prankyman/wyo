import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import SchemeBadge from './SchemeBadge.svelte';

/** Who packs an order: the seller (FBS, Express) or the marketplace (FBO, FBY: read-only), with a «?» that says it. */
describe('SchemeBadge', () => {
    it.each([
        ['fbs', 'FBS', 'bg-brand-50', 'собираете и упаковываете сами'],
        ['fbo', 'FBO', 'bg-ink-100', 'только для сведения'],
        ['express', 'Экспресс', 'bg-brand-50', 'курьер маркетплейса забирает у вас'],
        ['fby', 'FBY', 'bg-ink-100', 'только для сведения']
    ] as const)('draws %s as «%s»', (scheme, label, toneClass, help) => {
        render(SchemeBadge, { props: { lang: 'ru', scheme, testId: 'scheme' } });

        expect(screen.getByTestId('scheme')).toHaveTextContent(label);
        expect(screen.getByTestId('scheme')).toHaveClass(toneClass);
        expect(screen.getByTestId('scheme-help-button').getAttribute('aria-label')).toContain(help);
    });
});
