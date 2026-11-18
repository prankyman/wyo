import { goto, preloadData, pushState, replaceState } from '$app/navigation';
import { page, resetPage, setPageUrl } from '$test/support/page-state.svelte';
import { stubHistoryBack } from '$test/support/navigation';
import { fireEvent, render, screen } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import RowDetailsPanel from './RowDetailsPanel.svelte';

vi.mock('$app/state', async () => {
    const state = await import('$test/support/page-state.svelte');
    return { page: state.page, navigating: state.navigating };
});
vi.mock('$app/navigation', async () => (await import('$test/support/navigation')).pageStateNavigationModule);
vi.mock('$lib/i18n/i18n.js', async () => (await import('$test/support/i18n')).createI18nMock());
vi.mock('$lib/i18n/i18n', async () => (await import('$test/support/i18n')).createI18nMock());

interface IUnit {
    code: string;
    name: string;
}

const KG: IUnit = { code: 'kg', name: 'Kilogram' };
const PCS: IUnit = { code: 'pcs', name: 'Piece' };

const body = createRawSnippet((unit: () => IUnit) => ({
    render: () => `<p data-test="unit-body">${unit().name}</p>`
}));

function renderPanel(props: { fromLoad?: IUnit | null; refresh?: (id: string) => IUnit | null | undefined } = {}) {
    const anchor = document.createElement('a');
    anchor.setAttribute('href', '/units?edit=kg');
    anchor.dataset.test = 'row';
    document.body.append(anchor);
    return render(RowDetailsPanel<IUnit>, {
        props: {
            lang: 'ru',
            param: 'edit',
            fromLoad: props.fromLoad ?? null,
            idOf: (unit: IUnit) => unit.code,
            pick: (data: Record<string, unknown>) => data.editing as IUnit | null,
            refresh: props.refresh,
            listHref: '/units',
            title: (unit: IUnit) => unit.code,
            dataTest: 'unit-edit',
            children: body
        }
    });
}

describe('RowDetailsPanel', () => {
    beforeEach(() => {
        setPageUrl('/units');
        vi.mocked(preloadData).mockResolvedValue({ type: 'loaded', status: 200, data: { editing: KG } });
    });

    afterEach(() => {
        document.body.innerHTML = '';
        resetPage();
        vi.clearAllMocks();
    });

    it('pads the details of the row on every side, as the heading of the panel is padded', () => {
        renderPanel({ fromLoad: KG });

        const content = screen.getByTestId('unit-edit-content');
        expect(content).toHaveClass('p-4');
        expect(content).toContainElement(screen.getByTestId('unit-body'));
    });

    it('draws nothing while no row is opened', () => {
        renderPanel();
        expect(screen.queryByTestId('unit-edit-panel')).not.toBeInTheDocument();
    });

    it('opens the row the load returned for a direct address, and replaces the address by the list on close', async () => {
        renderPanel({ fromLoad: KG });

        expect(screen.getByTestId('unit-edit-panel')).toBeInTheDocument();
        expect(screen.getByRole('heading')).toHaveTextContent('kg');
        expect(screen.getByTestId('unit-body')).toHaveTextContent('Kilogram');

        await fireEvent.click(screen.getByTestId('unit-edit-close'));
        expect(goto).toHaveBeenCalledWith('/units', {
            noScroll: true,
            keepFocus: true,
            replaceState: true
        });
    });

    it('opens a pressed row without a page load, and goes back in the history on close', async () => {
        const back = stubHistoryBack();
        renderPanel();

        await fireEvent.click(screen.getByTestId('row'));
        await vi.waitFor(() => expect(screen.getByTestId('unit-body')).toHaveTextContent('Kilogram'));
        expect(pushState).toHaveBeenCalledWith('/units?edit=kg', {
            details: { param: 'edit', id: 'kg', value: KG }
        });
        expect(goto).not.toHaveBeenCalled();

        await fireEvent.click(screen.getByTestId('unit-edit-close'));
        expect(back).toHaveBeenCalledOnce();
        back.mockRestore();
    });

    it("ignores a row pressed in another panel's parameter", () => {
        page.state = { details: { param: 'other', id: 'other-1', value: { code: 'other-1', name: 'Other' } } };
        renderPanel();
        expect(screen.queryByTestId('unit-edit-panel')).not.toBeInTheDocument();
    });

    it("prefers the load's copy when it is the same row", () => {
        page.state = { details: { param: 'edit', id: 'kg', value: { code: 'kg', name: 'Stale' } } };
        renderPanel({ fromLoad: KG });
        expect(screen.getByTestId('unit-body')).toHaveTextContent('Kilogram');
    });

    it("shows the pressed row when the load's copy is another one", () => {
        page.state = { details: { param: 'edit', id: 'pcs', value: PCS } };
        renderPanel({ fromLoad: KG });
        expect(screen.getByTestId('unit-body')).toHaveTextContent('Piece');
    });

    it('reads the pressed row as the page data has it now, and closes when it is gone', () => {
        page.state = { details: { param: 'edit', id: 'kg', value: { code: 'kg', name: 'Stale' } } };
        const { unmount } = renderPanel({ refresh: (id) => (id === 'kg' ? KG : null) });
        expect(screen.getByTestId('unit-body')).toHaveTextContent('Kilogram');
        unmount();

        renderPanel({ refresh: () => undefined });
        expect(screen.queryByTestId('unit-edit-panel')).not.toBeInTheDocument();
    });

    it("reads the pressed row's own address again when a save re-read the page, and shows that copy", async () => {
        window.history.pushState({}, '', '/units?edit=kg');
        const entry = { param: 'edit', id: 'kg', value: { code: 'kg', name: 'Stale' } };
        page.state = { details: entry };
        renderPanel();
        expect(preloadData).not.toHaveBeenCalled();

        page.data = { units: [] };
        await vi.waitFor(() =>
            expect(replaceState).toHaveBeenCalledWith('/units?edit=kg', {
                details: { ...entry, value: KG }
            })
        );
        expect(preloadData).toHaveBeenCalledWith('/units?edit=kg');
        window.history.pushState({}, '', '/');
    });

    it('leaves the panel as it is when the re-read finds nothing, or another row is open by then', async () => {
        window.history.pushState({}, '', '/units?edit=kg');
        page.state = { details: { param: 'edit', id: 'kg', value: KG } };
        renderPanel();

        vi.mocked(preloadData).mockResolvedValueOnce({ type: 'loaded', status: 200, data: { editing: null } });
        page.data = { units: [] };
        await vi.waitFor(() => expect(preloadData).toHaveBeenCalledOnce());

        vi.mocked(preloadData).mockResolvedValueOnce({ type: 'redirect', location: '/' });
        page.data = { units: [1] };
        await vi.waitFor(() => expect(preloadData).toHaveBeenCalledTimes(2));

        // Another row is opened while the read is on its way: its answer is dropped.
        let answer: (value: Awaited<ReturnType<typeof preloadData>>) => void = () => undefined;
        vi.mocked(preloadData).mockReturnValueOnce(new Promise((resolve) => (answer = resolve)));
        page.data = { units: [2] };
        await vi.waitFor(() => expect(preloadData).toHaveBeenCalledTimes(3));
        page.state = { details: { param: 'edit', id: 'pcs', value: PCS } };
        answer({ type: 'loaded', status: 200, data: { editing: KG } });
        await Promise.resolve();
        expect(replaceState).not.toHaveBeenCalled();
        window.history.pushState({}, '', '/');
    });

    it("does not re-read for another panel's row", async () => {
        page.state = { details: { param: 'other', id: 'other-1', value: {} } };
        renderPanel();
        page.data = { units: [] };
        await Promise.resolve();
        expect(preloadData).not.toHaveBeenCalled();
    });

    it('puts a pressed row back when a re-read of the page cleared its entry and the address still names it', async () => {
        window.history.pushState({}, '', '/units?edit=kg');
        const entry = { param: 'edit', id: 'kg', value: KG };
        page.state = { details: entry };
        renderPanel();

        // `invalidateAll` after a successful answer: the page state is cleared, the data is new.
        page.state = {};
        page.data = { units: [] };
        await vi.waitFor(() => expect(replaceState).toHaveBeenCalledWith('/units?edit=kg', { details: entry }));
        expect(preloadData).toHaveBeenCalledWith('/units?edit=kg');
        window.history.pushState({}, '', '/');
    });

    it('lets the panel go when its row left the address (closed by «назад»)', async () => {
        window.history.pushState({}, '', '/units');
        page.state = { details: { param: 'edit', id: 'kg', value: KG } };
        renderPanel();

        page.state = {};
        await Promise.resolve();
        expect(replaceState).not.toHaveBeenCalled();
        expect(screen.queryByTestId('unit-edit-panel')).not.toBeInTheDocument();
        window.history.pushState({}, '', '/');
    });
});
