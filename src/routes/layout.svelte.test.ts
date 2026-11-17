import { render, screen } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { describe, expect, it } from 'vitest';
import Layout from './+layout.svelte';

describe('the root layout', () => {
    it('draws the page it is given', () => {
        const children = createRawSnippet(() => ({ render: () => '<p data-test="inner">page</p>' }));

        render(Layout, { props: { children } });

        expect(screen.getByTestId('inner')).toHaveTextContent('page');
    });
});
