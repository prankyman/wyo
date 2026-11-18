import { fireEvent, render, screen, within } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { describe, expect, it, vi } from 'vitest';
import DataList from './DataList.svelte';
import type { IDataListColumn, IDataListRow } from './types';

/** The words of a language that exists only for this test, so the list's own texts are read through its keys. */
vi.mock('$lib/i18n/i18n', async () =>
    (await import('$test/support/i18n')).createI18nMock({
        texts: {
            en: {
                'list.code': 'Code',
                'list.name': 'Name',
                'list.qty': 'Quantity',
                'list.note': 'Note',
                'list.select': 'Select card {name}',
                'list.tickAllPage': 'Tick all on this page',
                'app.tickAll.tick': 'Tick all shown',
                'app.tickAll.untick': 'Clear ticks'
            }
        }
    })
);

const columns: IDataListColumn[] = [
    { key: 'code', labelKey: 'list.code', emphasis: 'primary' },
    { key: 'name', labelKey: 'list.name', emphasis: 'secondary' },
    { key: 'qty', labelKey: 'list.qty', align: 'end' },
    { key: 'note', labelKey: 'list.note', desktopOnly: true }
];

const rows: IDataListRow[] = [
    { id: 'r1', href: '/items/r1', cells: { code: 'DEMO', name: 'Demo row', qty: 12, note: 'desk only' } },
    { id: 'r2', cells: { code: 'OLD', name: 'Retired', qty: null, note: null }, muted: true }
];

const renderList = (props: Partial<Record<string, unknown>> = {}) =>
    render(DataList, { props: { lang: 'en', columns, rows, ...props } as never });

describe('DataList', () => {
    it('ticks rows in both layouts into one list, outside the card’s link', async () => {
        const { container } = renderList({ selectLabelKey: 'list.select', selected: [] });

        const table = container.querySelector('[data-test="data-list-table"]') as HTMLElement;
        expect(table.querySelectorAll('thead th')).toHaveLength(5);
        const boxes = container.querySelectorAll<HTMLInputElement>('[data-test="data-list-select"][data-row-id="r1"]');
        expect(boxes).toHaveLength(2);
        expect(boxes[0]).toHaveAttribute('aria-label', 'Select card DEMO');
        expect(boxes[1].closest('a')).toBeNull();

        await fireEvent.click(boxes[1]);
        expect(boxes[0].checked).toBe(true);
    });

    it('ticks and unticks every row shown from the heading box, in the table and above the cards, keeping other ticks', async () => {
        const { container } = renderList({ selectLabelKey: 'list.select', selected: ['elsewhere'] });
        const head = screen.getByTestId('data-list-tick-all') as HTMLInputElement;
        const overCards = screen.getByTestId('data-list-cards-tick-all') as HTMLInputElement;
        const row = (id: string) =>
            container.querySelector<HTMLInputElement>(`[data-test="data-list-select"][data-row-id="${id}"]`)!;
        expect(head.closest('th')).not.toBeNull();
        expect(overCards.closest('[data-test="data-list-cards"]')).toBeNull();
        expect(head).toHaveAccessibleName('Tick all shown');

        await fireEvent.click(head);
        expect(row('r1').checked).toBe(true);
        expect(row('r2').checked).toBe(true);
        expect(head).toBeChecked();
        expect(overCards).toBeChecked();
        expect(head).toHaveAccessibleName('Clear ticks');

        await fireEvent.click(row('r2'));
        expect(head.indeterminate).toBe(true);
        expect(overCards.indeterminate).toBe(true);

        await fireEvent.click(overCards);
        expect(row('r2').checked).toBe(true);
        await fireEvent.click(head);
        expect(row('r1').checked).toBe(false);
        expect(row('r2').checked).toBe(false);
        expect(head).not.toBeChecked();
        expect(head.indeterminate).toBe(false);
    });

    it('names the heading box by the page when asked to, and draws no box without ticks', () => {
        renderList({
            selectLabelKey: 'list.select',
            selected: [],
            tickAllKey: 'list.tickAllPage'
        });
        expect(screen.getByTestId('data-list-tick-all')).toHaveAccessibleName('Tick all on this page');
    });

    // On a phone the heading box stands at the right of the list's own line, beside what the page puts there.
    it("puts the page's line above the list and, on a phone, the heading box at its right end, over the cards' boxes", () => {
        const toolbar = createRawSnippet(() => ({
            render: () => `<button data-test="page-actions">Изменить</button>`
        }));
        renderList({ selectLabelKey: 'list.select', selected: [], toolbar, toolbarClass: 'mb-3' });

        const line = screen.getByTestId('data-list-toolbar');
        // The box stands at the bottom of the line, right over the cards, however many lines the page's buttons wrap to.
        expect(line).toHaveClass('flex', 'items-end', 'mb-3');
        expect(line.firstElementChild).toContainElement(screen.getByTestId('page-actions'));
        const boxPlace = screen
            .getByTestId('data-list-cards-tick-all')
            .closest('[data-test="data-list-toolbar-tick"]')!;
        expect(line).toContainElement(boxPlace as HTMLElement);
        expect(boxPlace).toHaveClass('ml-auto', 'shrink-0', 'md:hidden');
    });

    it('draws no heading box on a list without ticks', () => {
        renderList();
        expect(screen.queryByTestId('data-list-tick-all')).toBeNull();
        expect(screen.queryByTestId('data-list-cards-tick-all')).toBeNull();
    });

    it('renders the same rows as a table and as cards, from one column definition', () => {
        const { container } = renderList();

        const table = container.querySelector('[data-test="data-list-table"]') as HTMLElement;
        const cards = container.querySelector('[data-test="data-list-cards"]') as HTMLElement;
        expect(table).not.toBeNull();
        expect(cards).not.toBeNull();
        expect(table.querySelectorAll('thead th')).toHaveLength(4);
        expect(table.querySelectorAll('tbody tr')).toHaveLength(2);
        expect(cards.querySelectorAll('[data-test="data-list-card"]')).toHaveLength(2);
    });

    it('makes the primary column the link that opens the record from anywhere on its row and its card', () => {
        const { container } = renderList();

        const link = container.querySelector('[data-test="data-list-link"]');
        expect(link).toHaveAttribute('href', '/items/r1');
        expect(link).toHaveTextContent('DEMO');
        // Its hit area is stretched over the whole row (ClickableRow + StretchedLink).
        expect(link).toHaveAttribute('data-stretched-link');
        const [linkedRow, plainRow] = container.querySelectorAll('[data-test="data-list-row"]');
        expect(linkedRow).toHaveClass('clickable-surface');
        expect(plainRow).not.toHaveClass('clickable-surface');

        const [firstCard, secondCard] = container.querySelectorAll('[data-test="data-list-card"]');
        const cardLink = within(firstCard as HTMLElement).getByTestId('data-list-card-link');
        expect(cardLink).toHaveTextContent('DEMO');
        expect(cardLink).toHaveTextContent('Demo row');
        expect(cardLink).toHaveAttribute('data-stretched-link');
        expect(firstCard.tagName).toBe('LI');
        expect(firstCard).toHaveAttribute('data-row-id', 'r1');
        expect(firstCard).toHaveClass('clickable-surface');
        expect(secondCard).not.toHaveClass('clickable-surface');
    });

    it('leaves a row with no href unlinked rather than rendering a dead anchor', () => {
        const { container } = renderList();

        const secondCard = container.querySelectorAll('[data-test="data-list-card"]')[1] as HTMLElement;
        expect(within(secondCard).queryByTestId('data-list-card-link')).toBeNull();
        expect(secondCard).toHaveTextContent('OLD');
    });

    it('shows an em dash for a missing value instead of an empty cell', () => {
        // An empty cell reads as "this column does not apply"; a dash reads as "nothing recorded".
        const { container } = renderList();

        const secondRow = container.querySelectorAll('[data-test="data-list-row"]')[1] as HTMLElement;
        expect(secondRow).toHaveTextContent('—');
    });

    it('keeps a desktop-only column out of the phone card but in the table', () => {
        const { container } = renderList();

        const firstCard = container.querySelectorAll('[data-test="data-list-card"]')[0] as HTMLElement;
        expect(firstCard).not.toHaveTextContent('desk only');
        expect(container.querySelector('[data-test="data-list-table"]')).toHaveTextContent('desk only');
    });

    it('right-aligns a numeric column so figures line up on their digits', () => {
        const { container } = renderList();

        const cells = container.querySelectorAll('[data-test="data-list-row"] td');
        expect(cells[2]).toHaveClass('text-right');
        expect(cells[2]).toHaveClass('tabular-nums');
    });

    it('greys a deactivated row without spending a column on saying so', () => {
        const { container } = renderList();

        const rowsEls = container.querySelectorAll('[data-test="data-list-row"]');
        expect(rowsEls[0]).toHaveClass('text-ink-800');
        expect(rowsEls[1]).toHaveClass('text-ink-400');
    });

    it('draws a row that needs attention in the stop colour, in the table and in the card', () => {
        const { container } = renderList({ rows: [{ ...rows[0], alert: true }, rows[1]] });

        const tableRows = container.querySelectorAll('[data-test="data-list-row"]');
        expect(tableRows[0]).toHaveClass('text-stop-700');
        expect(tableRows[1]).not.toHaveClass('text-stop-700');
        expect(container.querySelectorAll('.text-stop-700')).toHaveLength(2);
    });

    it('draws a row’s badge beside its primary cell in the table and in the card, linked or not, and nowhere on a row without one', () => {
        const { container } = renderList({
            rows: [
                { ...rows[0], badge: 'Set' },
                { ...rows[1], badge: 'Kit' }
            ]
        });

        const badges = Array.from(container.querySelectorAll('[data-test="data-list-badge"]'));
        // Two rows in the table and the same two in the cards.
        expect(badges.map((badge) => badge.textContent?.trim())).toEqual(['Set', 'Kit', 'Set', 'Kit']);
        const firstRow = container.querySelectorAll('[data-test="data-list-row"]')[0] as HTMLElement;
        expect(within(firstRow).getByText('Set')).toBeInTheDocument();
        expect(within(firstRow).getByTestId('data-list-link')).toHaveTextContent('DEMO');

        const plain = renderList().container;
        expect(plain.querySelector('[data-test="data-list-badge"]')).toBeNull();
    });

    it('shows the empty message and neither layout when there is nothing to list', () => {
        const { container } = renderList({ rows: [] });

        expect(container.querySelector('[data-test="data-list-empty"]')).toBeInTheDocument();
        expect(container.querySelector('[data-test="data-list-table"]')).toBeNull();
        expect(container.querySelector('[data-test="data-list-cards"]')).toBeNull();
    });

    it('puts the tour anchor on both layouts of the rows, and nowhere while the list is empty', () => {
        const { container } = renderList({ dataTour: 'rows-list' });
        const anchored = container.querySelectorAll('[data-tour="rows-list"]');
        expect(anchored).toHaveLength(2);
        expect(anchored[0]).toContainElement(container.querySelector('[data-test="data-list-table"]') as HTMLElement);
        expect(anchored[1]).toBe(container.querySelector('[data-test="data-list-cards"]'));

        const empty = renderList({ rows: [], dataTour: 'rows-list' }).container;
        expect(empty.querySelector('[data-tour]')).toBeNull();
    });
});

// A press of the row's own, after its cells in the table and under the card's rows, outside the row's link.
describe('DataList row action', () => {
    const rowAction = createRawSnippet((row: () => IDataListRow) => ({
        render: () => `<span data-test="row-action">${row().id}</span>`
    }));
    const actionRows: IDataListRow[] = [
        { id: 'r-1', href: '/items/r-1', cells: { code: 'A' } },
        { id: 'r-2', cells: { code: 'B' } }
    ];
    const actionColumns: IDataListColumn[] = [{ key: 'code', labelKey: 'app.list.empty', emphasis: 'primary' }];

    it('draws the action once per row in the table and once per card, outside the row’s link', () => {
        const { container } = render(DataList, {
            props: { lang: 'ru', columns: actionColumns, rows: actionRows, rowAction }
        });

        const table = container.querySelector('[data-test="data-list-table"]') as HTMLElement;
        expect(
            within(table)
                .getAllByTestId('row-action')
                .map((node) => node.textContent)
        ).toEqual(['r-1', 'r-2']);
        expect(table.querySelectorAll('th')).toHaveLength(2);
        const cards = container.querySelector('[data-test="data-list-cards"]') as HTMLElement;
        expect(within(cards).getAllByTestId('row-action')).toHaveLength(2);
        for (const action of within(cards).getAllByTestId('row-action')) expect(action.closest('a')).toBeNull();
    });

    it('draws no action column without one', () => {
        const { container } = render(DataList, { props: { lang: 'ru', columns: actionColumns, rows: actionRows } });

        expect(container.querySelectorAll('[data-test="data-list-table"] th')).toHaveLength(1);
    });
});

// The options a list may need — a «?» heading, a badge column, a custom cell, a legend, the `sm` switch, cell ids and extra
// row attributes.
describe('DataList options', () => {
    interface IExtraRow extends IDataListRow {
        word: string;
    }
    const extraColumns: IDataListColumn<IExtraRow>[] = [
        { key: 'product', labelKey: 'list.name', emphasis: 'primary', custom: true },
        { key: 'qty', labelKey: 'list.qty', helpKey: 'app.list.empty', align: 'end' },
        {
            key: 'state',
            labelKey: 'list.note',
            helpKey: 'app.list.empty',
            badge: { label: (row) => row.word, colorClass: () => 'bg-wait-50', dataTest: 'state-badge' }
        },
        { key: 'extra', labelKey: 'list.code', custom: true }
    ];
    const extraRows: IExtraRow[] = [
        {
            id: 'x1',
            word: 'Late',
            cells: { qty: '5 pcs' },
            attributes: { 'data-item': 'X-ONE' }
        }
    ];
    const cell = createRawSnippet((row: () => IExtraRow, column: () => IDataListColumn<IExtraRow>) => ({
        render: () => `<span data-test="custom-${column().key}">${row().id}</span>`
    }));
    const renderExtra = (props: Partial<Record<string, unknown>> = {}) =>
        render(DataList, { props: { lang: 'en', columns: extraColumns, rows: extraRows, cell, ...props } as never });

    it('gives a column with a help text a «?» in its heading, and a column without one none', () => {
        const { container } = renderExtra();

        const heads = container.querySelectorAll('thead th');
        expect(within(heads[1] as HTMLElement).getByTestId('data-list-qty-help-button')).toBeInTheDocument();
        expect(within(heads[3] as HTMLElement).queryByRole('button')).toBeNull();
    });

    it('draws a badge column as a badge in the table and in the card', () => {
        const { container } = renderExtra();

        const badges = container.querySelectorAll('[data-test="state-badge"]');
        expect(Array.from(badges).map((badge) => badge.textContent)).toEqual(['Late', 'Late']);
        expect(container.querySelector('[data-test="data-list-cards"] [data-test="state-badge"]')).not.toBeNull();
    });

    it('draws a custom column with the cell snippet, and makes a custom primary column the card’s heading', () => {
        const { container } = renderExtra();

        const table = within(container.querySelector('[data-test="data-list-table"]') as HTMLElement);
        expect(table.getByTestId('custom-product')).toHaveTextContent('x1');
        expect(table.getByTestId('custom-extra')).toBeInTheDocument();
        const card = within(container.querySelector('[data-test="data-list-card"]') as HTMLElement);
        expect(card.getByTestId('custom-product')).toHaveTextContent('x1');
        // The other custom column is a labelled line of the card.
        expect(card.getByTestId('custom-extra')).toBeInTheDocument();
    });

    it('draws a custom secondary column with the cell snippet under the card’s heading too', () => {
        const columns: IDataListColumn<IExtraRow>[] = [
            { key: 'code', labelKey: 'list.code', emphasis: 'primary' },
            { key: 'name', labelKey: 'list.name', emphasis: 'secondary', custom: true }
        ];
        const { container } = render(DataList, {
            props: { lang: 'en', columns, rows: [{ ...extraRows[0], cells: { code: 'ART-1' } }], cell } as never
        });

        const card = within(container.querySelector('[data-test="data-list-card"]') as HTMLElement);
        expect(card.getByTestId('custom-name')).toHaveTextContent('x1');
    });

    it('lists the columns that have a help text above the cards only when asked', () => {
        const { container } = renderExtra({ legend: true });

        const legend = within(container.querySelector('[data-test="data-list-legend"]') as HTMLElement);
        expect(legend.getByTestId('data-list-legend-qty-help-button')).toBeInTheDocument();
        expect(legend.getByTestId('data-list-legend-state-help-button')).toBeInTheDocument();
        expect(renderExtra().container.querySelector('[data-test="data-list-legend"]')).toBeNull();
    });

    it.each([
        ['md', 'md:block', 'md:hidden'],
        ['sm', 'sm:block', 'sm:hidden']
    ] as const)('switches between the table and the cards at the %s width', (breakpoint, tableClass, cardsClass) => {
        const { container } = renderExtra({ breakpoint, legend: true });

        expect(container.querySelector('[data-test="data-list-table"]')?.parentElement).toHaveClass(tableClass);
        expect(container.querySelector('[data-test="data-list-cards"]')).toHaveClass(cardsClass);
        expect(container.querySelector('[data-test="data-list-legend"]')).toHaveClass(cardsClass);
    });

    it('gives table cells their own ids only when asked, and never the cards', () => {
        const { container } = renderExtra({ cellTestIds: true });

        expect(container.querySelectorAll('[data-test="data-list-qty"]')).toHaveLength(1);
        expect(container.querySelector('[data-test="data-list-table"] [data-test="data-list-qty"]')).not.toBeNull();
        expect(renderExtra().container.querySelector('[data-test="data-list-qty"]')).toBeNull();
    });

    it('puts a row’s extra attributes on its table row and its card', () => {
        const { container } = renderExtra();

        expect(container.querySelector('[data-test="data-list-row"]')).toHaveAttribute('data-item', 'X-ONE');
        expect(container.querySelector('[data-test="data-list-card"]')).toHaveAttribute('data-item', 'X-ONE');
    });
});
