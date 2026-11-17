import { render, fireEvent, screen, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import Tabs from './Tabs.svelte';

vi.mock('$app/navigation', () => ({
    goto: vi.fn()
}));

const tabs = [
    { id: 'general', label: 'General' },
    { id: 'billing', label: 'Billing' },
    { id: 'shipping', label: 'Shipping' }
];

describe('Tabs', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('renders a button for every tab with its label', () => {
        render(Tabs, { props: { tabs, activeTab: 'general' } });

        expect(screen.getByRole('button', { name: 'General' })).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Billing' })).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Shipping' })).toBeInTheDocument();
        expect(screen.getAllByRole('button')).toHaveLength(3);
    });

    it('marks the active tab with the active class', () => {
        render(Tabs, { props: { tabs, activeTab: 'billing' } });

        expect(screen.getByRole('button', { name: 'Billing' })).toHaveClass('active');
        expect(screen.getByRole('button', { name: 'General' })).not.toHaveClass('active');
        expect(screen.getByRole('button', { name: 'Shipping' })).not.toHaveClass('active');
    });

    it('calls goto with the tab param set when a tab button is clicked', async () => {
        const { goto } = await import('$app/navigation');
        render(Tabs, { props: { tabs, activeTab: 'general' } });

        await fireEvent.click(screen.getByRole('button', { name: 'Billing' }));

        expect(goto).toHaveBeenCalledTimes(1);
        expect(goto).toHaveBeenCalledWith('http://localhost/?tab=billing', {
            replaceState: true,
            noScroll: true
        });
    });

    it('shows the pressed tab working until its page change ends, and takes no other tab meanwhile', async () => {
        // A tab moves by `goto`, which the page-change guard does not lock: the strip guards itself.
        const { goto } = await import('$app/navigation');
        let finish = (): void => {};
        vi.mocked(goto).mockImplementationOnce(() => new Promise<void>((resolve) => (finish = resolve)));
        render(Tabs, { props: { tabs, activeTab: 'general' } });
        const billing = screen.getByRole('button', { name: 'Billing' });
        const shipping = screen.getByRole('button', { name: 'Shipping' });

        await fireEvent.click(billing);
        await fireEvent.click(billing);
        await fireEvent.click(shipping);

        expect(goto).toHaveBeenCalledTimes(1);
        expect(billing).toHaveAttribute('aria-busy', 'true');
        expect(shipping).toHaveAttribute('aria-disabled', 'true');

        finish();
        await waitFor(() => expect(billing).not.toHaveAttribute('aria-busy'));
        expect(shipping).not.toHaveAttribute('aria-disabled');
        await fireEvent.click(shipping);
        expect(goto).toHaveBeenCalledTimes(2);
    });

    // Below `sm:` (640 px) the tabs lay out in wrapping rows so every one is on the screen —
    // five tabs of a connection can be about 540 px of words against 358 px of phone — and each is at
    // least 44 × 44 px; from `sm:` up the strip is the single scrolling row it always was. How the rows fall is
    // measured in a real browser by the phone sweep; jsdom lays nothing out, so the markup is proven here.
    describe('on a phone and above it', () => {
        const classesOf = (element: Element): string[] =>
            (element.getAttribute('class') ?? '').split(/\s+/).filter(Boolean);

        it('wraps the tabs into rows below sm and scrolls one strip from sm up', () => {
            render(Tabs, { props: { tabs, activeTab: 'general' } });

            const strip = screen.getByRole('button', { name: 'General' }).parentElement as HTMLElement;
            const classes = classesOf(strip);
            expect(classes).toEqual(expect.arrayContaining(['flex-wrap', 'sm:flex-nowrap', 'sm:overflow-x-auto']));
            // A scrolling strip on a phone is what hid the last tabs past the screen's edge.
            expect(classes).not.toContain('overflow-x-auto');
            expect(classes).not.toContain('flex-nowrap');
        });

        it('makes every tab at least 44 × 44 px, stretching to fill its row on a phone and not shrinking in the strip', () => {
            render(Tabs, { props: { tabs, activeTab: 'general' } });

            for (const button of screen.getAllByRole('button')) {
                expect(classesOf(button)).toEqual(
                    expect.arrayContaining([
                        'min-h-11',
                        'min-w-11',
                        'grow',
                        'sm:grow-0',
                        'sm:shrink-0',
                        'sm:whitespace-nowrap'
                    ])
                );
            }
        });

        it('marks the open tab with a filled background on a phone as well as the underline', () => {
            render(Tabs, { props: { tabs, activeTab: 'billing' } });

            const open = screen.getByRole('button', { name: 'Billing' });
            expect(open).toHaveClass('active', 'max-sm:bg-ink-100!');
            for (const name of ['General', 'Shipping']) {
                expect(classesOf(screen.getByRole('button', { name }))).not.toContain('max-sm:bg-ink-100!');
            }
        });

        it('keeps every tab a direct child of the one strip, which the connection page scrolls to centre the open tab', () => {
            render(Tabs, { props: { tabs, activeTab: 'general' } });

            const parents = new Set(screen.getAllByRole('button').map((button) => button.parentElement));
            expect(parents.size).toBe(1);
            expect([...parents][0]).toHaveClass('tabs-list');
        });
    });

    it('keeps the open tab in the query parameter it is given', async () => {
        const { goto } = await import('$app/navigation');
        render(Tabs, { props: { tabs, activeTab: 'general', param: 'view' } });

        await fireEvent.click(screen.getByRole('button', { name: 'Billing' }));

        expect(goto).toHaveBeenCalledWith('http://localhost/?view=billing', { replaceState: true, noScroll: true });
    });

    it('draws links to the address hrefOf gives, the open one marked as the current page, and moves nothing by goto', async () => {
        const { goto } = await import('$app/navigation');
        render(Tabs, {
            props: { tabs, activeTab: 'billing', hrefOf: (id: string) => `/settings/${id}`, dataTest: 'set' }
        });

        expect(screen.queryAllByRole('button')).toHaveLength(0);
        expect(screen.getAllByRole('link').map((link) => link.getAttribute('href'))).toEqual([
            '/settings/general',
            '/settings/billing',
            '/settings/shipping'
        ]);
        expect(screen.getByTestId('set-tab-billing')).toHaveAttribute('aria-current', 'page');
        expect(screen.getByTestId('set-tab-billing')).toHaveClass('active');
        expect(screen.getByTestId('set-tab-general')).not.toHaveAttribute('aria-current');

        await fireEvent.click(screen.getByTestId('set-tab-general'));
        expect(goto).not.toHaveBeenCalled();
    });

    it('draws links to the page address with the parameter set when the variant is link and no hrefOf is given', () => {
        render(Tabs, { props: { tabs, activeTab: 'general', variant: 'link', param: 'section' } });

        expect(screen.getByRole('link', { name: 'Shipping' })).toHaveAttribute('href', '/?section=shipping');
    });

    it('draws buttons when the variant is button even though hrefOf is given', () => {
        render(Tabs, { props: { tabs, activeTab: 'general', variant: 'button', hrefOf: (id: string) => `/${id}` } });

        expect(screen.queryAllByRole('link')).toHaveLength(0);
        expect(screen.getAllByRole('button')).toHaveLength(3);
    });

    it('calls onselect with the pressed tab instead of moving the page', async () => {
        const { goto } = await import('$app/navigation');
        const onselect = vi.fn();
        render(Tabs, { props: { tabs, activeTab: 'general', onselect } });

        await fireEvent.click(screen.getByRole('button', { name: 'Shipping' }));

        expect(onselect).toHaveBeenCalledExactlyOnceWith('shipping');
        expect(goto).not.toHaveBeenCalled();
    });

    it('shows a count after the label of the tab that has one', () => {
        render(Tabs, {
            props: {
                tabs: [
                    { id: 'general', label: 'General', count: 4 },
                    { id: 'billing', label: 'Billing', count: 0 },
                    { id: 'shipping', label: 'Shipping' }
                ],
                activeTab: 'general'
            }
        });

        expect(screen.getAllByRole('button').map((button) => button.textContent?.trim())).toEqual([
            'General · 4',
            'Billing · 0',
            'Shipping'
        ]);
    });

    it('draws a «?» beside the tabs that have a help text, named after the tab', () => {
        render(Tabs, {
            props: {
                tabs: [
                    { id: 'general', label: 'General', help: 'What general is.' },
                    { id: 'billing', label: 'Billing' }
                ],
                activeTab: 'general',
                dataTest: 'set'
            }
        });

        expect(screen.getByTestId('set-tab-general-help-button')).toBeInTheDocument();
        expect(screen.queryByTestId('set-tab-billing-help-button')).toBeNull();
        expect(screen.getByTestId('set-tab-billing').parentElement).toHaveClass('tabs-list');
    });

    it('is a navigation landmark with its own test id and tour anchor only when they are given', () => {
        const named = render(Tabs, {
            props: { tabs, activeTab: 'general', ariaLabel: 'Sections', stripTest: 'set-tabs', dataTour: 'set-tour' }
        });
        const nav = screen.getByRole('navigation', { name: 'Sections' });
        expect(nav).toHaveAttribute('data-test', 'set-tabs');
        expect(nav).toHaveAttribute('data-tour', 'set-tour');
        named.unmount();

        render(Tabs, { props: { tabs, activeTab: 'general' } });
        expect(screen.queryByRole('navigation')).toBeNull();
    });

    it('renders no buttons when tabs list is empty', () => {
        render(Tabs, { props: { tabs: [], activeTab: 'general' } });

        expect(screen.queryAllByRole('button')).toHaveLength(0);
    });
});
