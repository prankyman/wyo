import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import ShowcaseBasics from './ShowcaseBasics.svelte';

// The section only draws the components of the library; each is tested once in its own file. This test holds that the section
// renders, with a block for each group of components and each state the group promises to show.
const SECTIONS = [
    'buttons',
    'icons',
    'help',
    'notes',
    'statuses',
    'headings',
    'navigation',
    'records',
    'filters',
    'messages'
];

describe('the basic components section of the showcase', () => {
    it('draws a block for every group of components, each with its own heading', () => {
        const { container } = render(ShowcaseBasics);

        expect(screen.getByTestId('showcase-basics')).toBeInTheDocument();
        for (const name of SECTIONS) {
            const block = screen.getByTestId(`showcase-basics-${name}`);
            expect(block.querySelector('h2'), name).not.toBeNull();
        }
        // The page's own title is the only first-level heading; the section adds none.
        expect(container.querySelectorAll('h1')).toHaveLength(0);
    });

    it('draws each look and state the groups promise: button variants, sizes and states, tones, notes, empty lists and tabs', () => {
        render(ShowcaseBasics);

        for (const variant of [
            'primary',
            'secondary',
            'danger',
            'danger-outline',
            'success',
            'warning',
            'ghost',
            'link'
        ]) {
            expect(screen.getByTestId(`showcase-basics-button-${variant}`), variant).toBeInTheDocument();
        }
        for (const id of [
            'md',
            'lg',
            'xl',
            'disabled',
            'pending',
            'as-link',
            'icon',
            'icon-only',
            'full',
            'ghost-inverse'
        ]) {
            expect(screen.getByTestId(`showcase-basics-button-${id}`), id).toBeInTheDocument();
        }
        expect(screen.getByTestId('showcase-basics-button-disabled')).toBeDisabled();
        expect(screen.getByTestId('showcase-basics-button-pending')).toHaveAttribute('aria-busy', 'true');

        for (const tone of ['ok', 'wait', 'warn', 'stop', 'info', 'off']) {
            expect(screen.getByTestId(`showcase-basics-badge-${tone}`), tone).toBeInTheDocument();
            expect(screen.getByTestId(`showcase-basics-badge-help-${tone}`), tone).toBeInTheDocument();
        }
        for (const tone of ['warning', 'error', 'success', 'info', 'neutral']) {
            expect(screen.getByTestId(`showcase-basics-note-${tone}`), tone).toHaveAttribute('data-tone', tone);
        }
        for (const empty of ['box', 'inline', 'plain']) {
            expect(screen.getByTestId(`showcase-basics-empty-${empty}`), empty).toBeInTheDocument();
        }
        expect(screen.getByTestId('showcase-basics-tabs-strip')).toBeInTheDocument();
        expect(screen.getByTestId('showcase-basics-steps-compact')).toBeInTheDocument();
        expect(screen.getByTestId('showcase-basics-steps-words')).toBeInTheDocument();
    });
});
