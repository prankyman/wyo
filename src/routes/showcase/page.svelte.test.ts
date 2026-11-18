import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import { messages } from '$lib/i18n/messages';
import Page from './+page.svelte';

// The page itself only grows: the components of the library are drawn on it by the plans that copy them, each with its own
// states, and each tested once in its own file. This test holds the page's frame.
describe('the showcase page', () => {
    it('has its heading and draws the sections of the library: the basics, the forms and the lists', () => {
        render(Page);

        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(messages.ru['showcase.title']);
        const showcase = screen.getByTestId('showcase');
        for (const section of ['showcase-basics', 'showcase-forms', 'showcase-lists']) {
            expect(showcase, section).toContainElement(screen.getByTestId(section));
        }
    });
});
