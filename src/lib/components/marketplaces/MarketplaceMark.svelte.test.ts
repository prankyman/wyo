import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import {
    MARKETPLACE_CODE,
    MARKETPLACE_NAME,
    SECOND_MARKETPLACE,
    SECOND_MARKETPLACE_CODE,
    SECOND_MARKETPLACE_NAME,
    THIRD_MARKETPLACE,
    THIRD_MARKETPLACE_CODE,
    THIRD_MARKETPLACE_NAME
} from '$test/support/marketplaces';
import MarketplaceMark from './MarketplaceMark.svelte';

/** A marketplace's colour mark and its name, from the adapter's description — never from a name written in a screen. */
describe('MarketplaceMark', () => {
    it.each([
        ['the mark the adapter names', 'plum', 'bg-mark-plum'],
        ['a neutral grey for a mark the theme does not know', 'unknown-mark', 'bg-ink-400'],
        ['a neutral grey when the adapter is not known yet', undefined, 'bg-ink-400']
    ])('draws the name with %s', (_case, mark, dotClass) => {
        render(MarketplaceMark, { props: { lang: 'ru', code: MARKETPLACE_CODE, mark, dataTest: 'mark' } });

        expect(screen.getByTestId('mark')).toHaveTextContent(MARKETPLACE_NAME);
        expect(screen.getByTestId('mark-dot')).toHaveClass(dotClass);
        expect(screen.getByTestId('mark-dot')).toHaveAttribute('aria-hidden', 'true');
    });

    it('draws the second adapter with its own mark and name', () => {
        render(MarketplaceMark, {
            props: { lang: 'ru', code: SECOND_MARKETPLACE_CODE, mark: SECOND_MARKETPLACE.mark, dataTest: 'mark' }
        });

        expect(screen.getByTestId('mark')).toHaveTextContent(SECOND_MARKETPLACE_NAME);
        expect(screen.getByTestId('mark-dot')).toHaveClass('bg-mark-azure');
    });

    it('draws the third adapter with its own mark and name', () => {
        render(MarketplaceMark, {
            props: { lang: 'ru', code: THIRD_MARKETPLACE_CODE, mark: THIRD_MARKETPLACE.mark, dataTest: 'mark' }
        });

        expect(screen.getByTestId('mark')).toHaveTextContent(THIRD_MARKETPLACE_NAME);
        expect(screen.getByTestId('mark-dot')).toHaveClass('bg-mark-yellow');
    });

    it('shows the code itself for an adapter this interface has no name for', () => {
        render(MarketplaceMark, { props: { lang: 'ru', code: 'later-one', dataTest: 'mark' } });

        expect(screen.getByTestId('mark')).toHaveTextContent('later-one');
    });
});
