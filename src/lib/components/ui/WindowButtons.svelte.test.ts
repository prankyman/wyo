import { render, screen } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { describe, expect, it } from 'vitest';
import WindowButtons from './WindowButtons.svelte';

/**
 * The buttons of a window (constitution, "Windows"): one row in the window's bottom band, at the
 * right, «Отмена» or «Закрыть» first and the window's own action last; a figure for the press (the label total) at the left of the
 * same row, and the answer of the press on a line above the row.
 */
const snippet = (html: string) => createRawSnippet(() => ({ render: () => html }));

describe('WindowButtons', () => {
    it('keeps the buttons in one row at the right, the note at the left of that row and the answer on a line above it', () => {
        render(WindowButtons, {
            dataTest: 'print-buttons',
            children: snippet(
                '<span><button data-test="close">Закрыть</button><button data-test="print">Печать</button></span>'
            ),
            note: snippet('<span data-test="total">Всего этикеток: 24</span>'),
            above: snippet('<p data-test="answer">Принтер не отвечает</p>')
        });

        const root = screen.getByTestId('print-buttons');
        expect(root).toHaveAttribute('data-window-buttons');
        const [answerLine, row] = Array.from(root.children);
        expect(answerLine).toContainElement(screen.getByTestId('answer'));
        expect(row).toHaveClass('flex', 'flex-wrap', 'items-center', 'justify-end');
        const [note, buttons] = Array.from(row.children);
        expect(note).toContainElement(screen.getByTestId('total'));
        expect(note).toHaveClass('mr-auto');
        // The buttons are one group: on a narrow phone they wrap under the note together, never one under the other.
        expect(buttons).toHaveClass('flex', 'flex-wrap', 'justify-end');
        expect(buttons).toContainElement(screen.getByTestId('close'));
        expect(buttons).toContainElement(screen.getByTestId('print'));
        expect(screen.getByTestId('close').compareDocumentPosition(screen.getByTestId('print'))).toBe(
            Node.DOCUMENT_POSITION_FOLLOWING
        );
    });

    it('draws only the row of buttons when the window gives no note and no answer', () => {
        render(WindowButtons, { children: snippet('<button data-test="close">Закрыть</button>') });

        const root = document.querySelector('[data-window-buttons]');
        expect(root?.children).toHaveLength(1);
        expect(root?.firstElementChild).toHaveClass('justify-end');
        expect(root?.firstElementChild?.children).toHaveLength(1);
    });
});
