import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import HelpLinkList from './HelpLinkList.svelte';

const LINKS = [
    { label: 'Первая статья', href: 'https://example.org/first-article' },
    { label: 'Вторая статья', href: 'https://example.org/second-article' }
];

describe('HelpLinkList', () => {
    it('draws nothing for an empty list', () => {
        render(HelpLinkList, { props: { links: [], testId: 'links' } });

        expect(screen.queryByTestId('links')).not.toBeInTheDocument();
    });

    it('draws one link that opens in a new tab, named `<testId>-link` by default', () => {
        render(HelpLinkList, { props: { links: [LINKS[0]], testId: 'links' } });

        const link = screen.getByTestId('links-link');
        expect(link).toHaveTextContent('Первая статья');
        expect(link).toHaveAttribute('href', 'https://example.org/first-article');
        expect(link).toHaveAttribute('target', '_blank');
        expect(link).toHaveAttribute('rel', 'noopener noreferrer');
        // A link on a line of its own is a whole tap target, 44 px tall.
        expect(link).toHaveClass('inline-flex', 'min-h-11', 'items-center');
    });

    it('draws several links in their order, under the given link test id', () => {
        render(HelpLinkList, { props: { links: LINKS, testId: 'links', linkTestId: 'term-link' } });

        const links = screen.getAllByTestId('term-link');
        expect(links.map((link) => link.getAttribute('href'))).toEqual(LINKS.map((link) => link.href));
        expect(links.map((link) => link.textContent)).toEqual(LINKS.map((link) => link.label));
        expect(screen.queryByTestId('links-link')).not.toBeInTheDocument();
    });

    it('draws white links on the amber popup and amber links on a white card, with the given classes', () => {
        const { unmount } = render(HelpLinkList, {
            props: { links: [LINKS[0]], testId: 'amber', tone: 'onAmber', class: 'mt-2' }
        });
        expect(screen.getByTestId('amber-link')).toHaveClass('text-white');
        expect(screen.getByTestId('amber')).toHaveClass('mt-2');
        unmount();

        render(HelpLinkList, { props: { links: [LINKS[0]], testId: 'white' } });
        expect(screen.getByTestId('white-link')).toHaveClass('text-warn-700');
    });
});
