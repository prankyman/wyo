import { render, screen } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { describe, expect, it } from 'vitest';
import Layout from './+layout.svelte';

describe('the layout of the showcase page', () => {
    it('draws the page as the main part of the document with the side gutter of every page', () => {
        const children = createRawSnippet(() => ({ render: () => '<p data-test="inner">витрина</p>' }));

        render(Layout, { props: { children } });

        const main = screen.getByRole('main');
        expect(main).toContainElement(screen.getByTestId('inner'));
        expect(main).toHaveClass('px-4');
    });
});
