import { render, screen } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { describe, expect, it } from 'vitest';
import EmptyState from './EmptyState.svelte';

const action = createRawSnippet(() => ({
    render: () => `<a href="/records/new" data-test="empty-action">Make one</a>`
}));

describe('EmptyState', () => {
    it('draws the dashed box with the sentence centred in grey, and nothing under it', () => {
        render(EmptyState, { props: { message: 'No items yet.', dataTest: 'items-empty', dataTour: 'items-empty' } });

        const box = screen.getByTestId('items-empty');
        expect(box).toHaveClass(
            'rounded',
            'border',
            'border-dashed',
            'border-ink-300',
            'px-4',
            'py-8',
            'text-center',
            'text-sm',
            'text-ink-500'
        );
        expect(box).not.toHaveClass('print:hidden', 'py-6');
        expect(box).toHaveAttribute('data-tour', 'items-empty');
        expect(box.querySelector('p')).toHaveTextContent('No items yet.');
        expect(box.children).toHaveLength(1);
        // The whole text is the sentence: no stray space after it (a spec matches «Файл удалён …» to the end).
        expect(box.textContent).toBe('No items yet.');
    });

    it('is a lower box left off the printed page when asked, with the person’s next step under the sentence', () => {
        render(EmptyState, {
            props: {
                message: 'No items yet.',
                compact: true,
                noPrint: true,
                class: 'mt-3',
                dataTest: 'items-empty',
                children: action
            }
        });

        const box = screen.getByTestId('items-empty');
        expect(box).toHaveClass('py-6', 'print:hidden', 'mt-3');
        expect(box).not.toHaveClass('py-8');
        expect(screen.getByTestId('empty-action')).toHaveAttribute('href', '/records/new');
    });

    it('draws the small dashed line where a thing used to be', () => {
        render(EmptyState, {
            props: { message: 'The file was deleted on 1 October.', variant: 'inline', dataTest: 'file-gone' }
        });

        const line = screen.getByTestId('file-gone');
        expect(line).toHaveClass(
            'rounded',
            'border-dashed',
            'border-ink-300',
            'px-3',
            'py-2',
            'break-words',
            'text-ink-500'
        );
        expect(line).not.toHaveClass('px-4', 'py-8', 'py-6', 'text-center');
    });

    it('draws the plain grey line without a box where a box would be too much', () => {
        render(EmptyState, {
            props: {
                message: 'No codes.',
                variant: 'plain',
                dataTest: 'codes-empty',
                'data-empty-kind': 'not-read'
            } as never
        });

        const line = screen.getByTestId('codes-empty');
        expect(line).toHaveAttribute('data-empty-kind', 'not-read');
        expect(line).toHaveClass('text-sm', 'text-ink-500');
        expect(line).not.toHaveClass('border-dashed', 'px-4', 'py-8', 'text-center');
        expect(line).toHaveTextContent('No codes.');
    });
});
