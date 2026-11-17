import { render, screen } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { describe, expect, it } from 'vitest';
import NoteBox from './NoteBox.svelte';

const words = createRawSnippet(() => ({
    render: () => `<span data-test="note-words">Nothing was shipped yet.</span>`
}));

describe('NoteBox', () => {
    it.each([
        ['warning', 'border-warn-200 bg-warn-50 text-warn-800', null],
        ['error', 'border-stop-200 bg-stop-50 text-stop-800', 'alert'],
        ['success', 'border-ok-200 bg-ok-50 text-ok-800', 'status'],
        ['info', 'border-brand-200 bg-brand-50 text-brand-800', null],
        ['neutral', 'border-ink-200 bg-ink-50 text-ink-700', null]
    ] as const)('draws a %s note in its own colours with the live role of its tone', (tone, classes, role) => {
        render(NoteBox, { props: { tone, dataTest: 'note', children: words } });

        const box = screen.getByTestId('note');
        for (const token of classes.split(' ')) expect(box).toHaveClass(token);
        expect(box).toHaveClass('rounded', 'border', 'px-4', 'py-2', 'text-sm');
        expect(box).toHaveAttribute('data-tone', tone);
        if (role) expect(box).toHaveAttribute('role', role);
        else expect(box).not.toHaveAttribute('role');
        expect(screen.getByTestId('note-words')).toHaveTextContent('Nothing was shipped yet.');
    });

    it('takes a role, a small text, a tour anchor and the classes of its place over the defaults', () => {
        render(NoteBox, {
            props: {
                tone: 'warning',
                role: 'alert',
                size: 'xs',
                dataTest: 'note',
                dataTour: 'note-tour',
                class: 'mb-4',
                children: words
            }
        });

        const box = screen.getByTestId('note');
        expect(box).toHaveAttribute('role', 'alert');
        expect(box).toHaveClass('text-xs', 'mb-4');
        expect(box).not.toHaveAttribute('aria-live');
        expect(box).not.toHaveClass('text-sm');
        expect(box).toHaveAttribute('data-tour', 'note-tour');
    });

    it('marks an aside as a note', () => {
        render(NoteBox, { props: { tone: 'warning', role: 'note', dataTest: 'note', children: words } });

        expect(screen.getByTestId('note')).toHaveAttribute('role', 'note');
    });

    it('draws a success with no live role when the note stood on the page from the start, and passes a state attribute to the box', () => {
        render(NoteBox, {
            props: {
                tone: 'success',
                role: 'none',
                dataTest: 'note',
                'data-confirmed': 'true',
                children: words
            } as never
        });

        expect(screen.getByTestId('note')).not.toHaveAttribute('role');
        expect(screen.getByTestId('note')).toHaveAttribute('data-confirmed', 'true');
    });
});
