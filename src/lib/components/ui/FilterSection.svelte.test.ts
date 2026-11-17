import { fireEvent, render, screen, within } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { describe, expect, it, vi } from 'vitest';
import FilterSection from './FilterSection.svelte';
import type { IFilterChip } from './types';

vi.mock('$lib/i18n/i18n.js', async () => (await import('$test/support/i18n')).createI18nMock());
vi.mock('$lib/i18n/i18n', async () => (await import('$test/support/i18n')).createI18nMock());

/**
 * The «Фильтр» section of a list: closed on arrival, its heading counts the filters in use, and the filters
 * in use stay visible as removable chips under the heading — closed or open — with «Сбросить все».
 */

const boxes = createRawSnippet(() => ({ render: () => '<input data-test="filter-box" />' }));

const render_ = (
    props: Partial<{ chips: IFilterChip[]; resetHref: string; onResetAll: () => void; open: boolean }> = {}
) => render(FilterSection, { props: { lang: 'ru', chips: [], children: boxes, ...props } });

const CLIENT: IFilterChip = { key: 'client', label: 'Client: Daisy Ltd', href: '/items?binId=b1' };
const BIN: IFilterChip = { key: 'bin', label: 'Bin: R-01', href: '/items?clientId=c1' };

describe('FilterSection', () => {
    it('with no filter in use: arrives closed, heading without a number, no chips, no reset', () => {
        render_();

        const section = screen.getByTestId('filter-section');
        expect(section.tagName).toBe('DETAILS');
        expect(section).not.toHaveAttribute('open');
        // A block brings no outer margin; the column it stands in spaces it.
        expect(screen.getByTestId('filter-section-root').className).not.toMatch(/(^|\s)-?m[bty]-/);
        expect(screen.getByTestId('filter-section-heading')).toHaveTextContent('Фильтр');
        expect(screen.getByTestId('filter-section-heading')).not.toHaveTextContent('·');
        expect(screen.queryByTestId('filter-section-chips')).toBeNull();
        expect(screen.queryByTestId('filter-section-reset')).toBeNull();
        // The boxes stay in the page while the section is closed, so a closed section still sends them.
        expect(screen.getByTestId('filter-box')).toBeInTheDocument();
    });

    it('with one filter in use: the heading says «Фильтр · 1» and its chip is there while the section is closed', () => {
        render_({ chips: [CLIENT], resetHref: '/items' });

        expect(screen.getByTestId('filter-section')).not.toHaveAttribute('open');
        expect(screen.getByTestId('filter-section-heading')).toHaveTextContent('Фильтр · 1');
        const chip = screen.getByRole('link', { name: 'Убрать фильтр «Client: Daisy Ltd»' });
        expect(chip).toHaveTextContent('Client: Daisy Ltd');
        expect(chip).toHaveAttribute('href', '/items?binId=b1');
        expect(chip).toHaveClass('min-h-11');
    });

    it('with several filters in use: counts them all, one chip each in a labelled list, and keeps them when the section opens', async () => {
        render_({ chips: [CLIENT, BIN], resetHref: '/items' });

        expect(screen.getByTestId('filter-section-heading')).toHaveTextContent('Фильтр · 2');
        const list = screen.getByRole('list', { name: 'Применённые фильтры' });
        expect(within(list).getAllByRole('listitem')).toHaveLength(2);

        await fireEvent.click(screen.getByTestId('filter-section-heading'));

        expect(screen.getByTestId('filter-section')).toHaveAttribute('open');
        expect(screen.getByTestId('filter-section-chip-client')).toBeInTheDocument();
        expect(screen.getByTestId('filter-section-chip-bin')).toBeInTheDocument();
        expect(screen.getByTestId('filter-section-reset')).toBeInTheDocument();
    });

    it('arrives open when the screen asks for it', () => {
        render_({ open: true });

        expect(screen.getByTestId('filter-section')).toHaveAttribute('open');
    });

    it('removes one filter: a chip of the screen’s own state calls its onRemove', async () => {
        const onRemove = vi.fn();
        render_({ chips: [{ key: 'client', label: 'Client: Daisy Ltd', onRemove }, BIN], onResetAll: vi.fn() });

        const chip = screen.getByRole('button', { name: 'Убрать фильтр «Client: Daisy Ltd»' });
        expect(chip.tagName).toBe('BUTTON');
        await fireEvent.click(chip);

        expect(onRemove).toHaveBeenCalledTimes(1);
    });

    it('resets all: a link to the address without the filters, or a button that calls onResetAll', async () => {
        const { unmount } = render_({ chips: [CLIENT], resetHref: '/items?itemId=s1' });
        const link = screen.getByTestId('filter-section-reset');
        expect(link.tagName).toBe('A');
        expect(link).toHaveAttribute('href', '/items?itemId=s1');
        expect(link).toHaveTextContent('Сбросить все');
        unmount();

        const onResetAll = vi.fn();
        render_({ chips: [CLIENT], onResetAll });
        const button = screen.getByTestId('filter-section-reset');
        expect(button.tagName).toBe('BUTTON');
        await fireEvent.click(button);
        expect(onResetAll).toHaveBeenCalledTimes(1);
    });
});
