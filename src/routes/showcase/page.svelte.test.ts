import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import { messages } from '$lib/i18n/messages';
import Page from './+page.svelte';

// The page itself only grows: the components of the library are drawn on it by the plans that copy them, each with its own
// states, and each tested once in its own file. This test holds the page's frame.
describe('the showcase page', () => {
    it('has its heading', () => {
        render(Page);

        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(messages.ru['showcase.title']);
        expect(screen.getByTestId('showcase')).toBeInTheDocument();
    });
});
