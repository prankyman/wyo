import { render, screen } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { describe, expect, it } from 'vitest';
import FormButtons from './FormButtons.svelte';

/** The buttons of a form as a row of their own under the fields, spanning every column of a grid form. */
const snippet = (html: string) => createRawSnippet(() => ({ render: () => html }));

describe('FormButtons', () => {
    it('spans the whole grid, keeps the buttons in one row at the left and draws the answer on a line under them', () => {
        render(FormButtons, {
            dataTest: 'filters-buttons',
            class: 'mt-2',
            children: snippet('<button type="submit" data-test="apply">Показать</button>'),
            below: snippet('<p data-test="answer">Сохранено</p>')
        });

        const row = screen.getByTestId('filters-buttons');
        expect(row).toHaveAttribute('data-form-buttons');
        expect(row).toHaveClass('col-span-full', 'mt-2');
        const buttons = screen.getByTestId('apply').parentElement;
        expect(buttons).toHaveClass('flex', 'flex-wrap', 'items-center');
        expect(buttons).not.toHaveClass('justify-end');
        expect(screen.getByTestId('answer').parentElement).toBe(row);
    });

    it('draws no answer line when the form gives none', () => {
        render(FormButtons, { children: snippet('<button type="submit">OK</button>') });

        expect(document.querySelector('[data-form-buttons]')?.children).toHaveLength(1);
    });
});
