import CollapsibleSectionInnerFormHarness from '$test/harnesses/ui/CollapsibleSectionInnerFormHarness.svelte';
import CollapsibleSectionTestHarness from '$test/harnesses/ui/CollapsibleSectionTestHarness.svelte';
import CollapsibleSection from '$lib/components/ui/CollapsibleSection.svelte';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('$lib/i18n/i18n.js', async () => (await import('$test/support/i18n')).createI18nMock());
vi.mock('$lib/i18n/i18n', async () => (await import('$test/support/i18n')).createI18nMock());

/**
 * One collapsible section of a screen: a native `<details>` whose heading row
 * opens and closes it, closed on arrival unless the screen asks for it open, opened by its address fragment, kept open
 * when the person opened it before the page's script started, and opened by a refused field inside it.
 */

const IDS = 'client-section-ids';
const BANK = 'client-section-bank';

describe('CollapsibleSection', () => {
    afterEach(() => {
        window.location.hash = '';
        document.body.innerHTML = '';
        vi.restoreAllMocks();
    });

    it('arrives closed with its arrow, title and summary in a 44 px heading row that opens and closes it', async () => {
        render(CollapsibleSectionTestHarness, { props: { lang: 'ru', withSummary: true } });

        const section = screen.getByTestId(IDS);
        const heading = screen.getByTestId(`${IDS}-heading`);
        expect(section.tagName).toBe('DETAILS');
        expect(section).toHaveAttribute('id', IDS);
        expect(section).toHaveAttribute('data-collapsible');
        expect(section).toHaveAttribute('data-tour', IDS);
        expect(section).not.toHaveAttribute('open');
        // A block brings no outer margin; the column it stands in spaces it.
        expect(section.className).not.toMatch(/(^|\s)-?m[bty]-/);
        expect(heading.tagName).toBe('SUMMARY');
        expect(heading).toHaveClass('min-h-11');
        expect(heading).toHaveAttribute('data-tour', `${IDS}-heading`);
        expect(heading.querySelector('h3')).toHaveTextContent('Identifiers');
        expect(screen.getByTestId(`${IDS}-arrow`)).toHaveAttribute('aria-hidden', 'true');
        expect(screen.getByTestId(`${IDS}-summary`)).toHaveTextContent('3 rows');
        expect(heading).toHaveTextContent('Нажмите, чтобы открыть');
        // The boxes stay in the page while the section is closed: «Сохранить» still posts them.
        expect(screen.getByTestId('client-inn')).toBeInTheDocument();

        await fireEvent.click(heading);
        await waitFor(() => expect(section).toHaveAttribute('open'));
        await waitFor(() => expect(heading).toHaveTextContent('Нажмите, чтобы закрыть'));

        await fireEvent.click(heading);
        await waitFor(() => expect(section).not.toHaveAttribute('open'));
    });

    it('draws the section’s «?» in the heading row right after the title, and a press on it neither folds nor unfolds the section', async () => {
        const body = createRawSnippet(() => ({ render: () => `<p data-test="reports-body">The reports.</p>` }));
        render(CollapsibleSection, {
            props: {
                lang: 'ru',
                id: 'reports-list',
                title: 'Reports of the month',
                help: 'The reports this month holds.',
                children: body
            }
        });

        const section = screen.getByTestId('reports-list');
        const heading = screen.getByTestId('reports-list-heading');
        const button = screen.getByTestId('reports-list-help-button');
        expect(heading.querySelector('h2')!.nextElementSibling).toContainElement(button);
        expect(button).toHaveAttribute('aria-label', 'Reports of the month');
        expect(section).not.toHaveAttribute('open');

        await fireEvent.click(button);
        expect(screen.getByTestId('reports-list-help-tooltip')).toHaveTextContent('The reports this month holds.');
        expect(section).not.toHaveAttribute('open');

        await fireEvent.click(heading);
        await waitFor(() => expect(section).toHaveAttribute('open'));
        await fireEvent.click(button);
        expect(section).toHaveAttribute('open');
    });

    it('arrives open when the screen asks for it (a create screen)', () => {
        render(CollapsibleSectionTestHarness, { props: { lang: 'ru', open: true } });

        expect(screen.getByTestId(IDS)).toHaveAttribute('open');
        expect(screen.getByTestId(BANK)).not.toHaveAttribute('open');
    });

    it('opens and scrolls to itself when the address ends in its id', async () => {
        const scrollIntoView = vi.fn();
        Element.prototype.scrollIntoView = scrollIntoView;
        window.location.hash = `#${BANK}`;
        render(CollapsibleSectionTestHarness, { props: { lang: 'ru' } });

        await waitFor(() => expect(screen.getByTestId(BANK)).toHaveAttribute('open'));
        expect(screen.getByTestId(IDS)).not.toHaveAttribute('open');
        expect(scrollIntoView).toHaveBeenCalledTimes(1);
    });

    it('stays open when the person opened it before the page’s script started (A1a)', () => {
        // The page as the server drew it, the section opened by a press before the script took over.
        const drawn = document.createElement('details');
        drawn.id = IDS;
        drawn.open = true;
        document.body.append(drawn);

        render(CollapsibleSectionTestHarness, { props: { lang: 'ru' } });

        expect(screen.getByTestId(IDS)).toHaveAttribute('open');
        expect(screen.getByTestId(BANK)).not.toHaveAttribute('open');
    });

    it('opens at once for a box the browser’s own check refuses (A3, the `invalid` event)', () => {
        render(CollapsibleSectionTestHarness, { props: { lang: 'ru' } });

        // `invalid` does not bubble (HTML standard): the section catches it on the way down.
        screen.getByTestId('client-inn').dispatchEvent(new Event('invalid', { bubbles: false, cancelable: true }));

        expect(screen.getByTestId(IDS)).toHaveAttribute('open');
        expect(screen.getByTestId(BANK)).not.toHaveAttribute('open');
    });

    it('opens for a box the server refused and puts the focus in it (A3, `Field`’s mark)', async () => {
        const scrollIntoView = vi.fn();
        Element.prototype.scrollIntoView = scrollIntoView;
        const { rerender } = render(CollapsibleSectionTestHarness, { props: { lang: 'ru' } });
        expect(screen.getByTestId(BANK)).not.toHaveAttribute('open');

        await rerender({ lang: 'ru', refusal: 'The account has 20 digits.' });

        await waitFor(() => expect(screen.getByTestId(BANK)).toHaveAttribute('open'));
        expect(screen.getByTestId('client-account-error')).toHaveTextContent('The account has 20 digits.');
        await waitFor(() => expect(screen.getByTestId('client-account')).toHaveFocus());
        expect(scrollIntoView).toHaveBeenCalledTimes(1);
        expect(screen.getByTestId(IDS)).not.toHaveAttribute('open');
    });

    it('opens for a box refused before it was closed, when its form is sent again (A3)', async () => {
        const { rerender } = render(CollapsibleSectionTestHarness, {
            props: { lang: 'ru', refusal: 'Twenty digits.' }
        });
        // The refusal was drawn while the section was open, then the person closed it.
        await waitFor(() => expect(screen.getByTestId(BANK)).toHaveAttribute('open'));
        await fireEvent.click(screen.getByTestId(`${BANK}-heading`));
        await waitFor(() => expect(screen.getByTestId(BANK)).not.toHaveAttribute('open'));
        await rerender({ lang: 'ru', refusal: 'Twenty digits.' });

        await fireEvent.submit(screen.getByTestId('the-form'));

        await waitFor(() => expect(screen.getByTestId(BANK)).toHaveAttribute('open'));
    });

    it('puts the cursor in a refused box when the section is a form of its own and its send is refused', async () => {
        const { rerender } = render(CollapsibleSectionInnerFormHarness, { props: { lang: 'ru' } });
        await fireEvent.submit(screen.getByTestId('the-section-form'));
        await rerender({ lang: 'ru', refusal: 'Ten or twelve digits.' });

        await waitFor(() => expect(screen.getByTestId('client-inn')).toHaveFocus());
    });

    it('puts the cursor in a box the server refuses after its last timed look, while the send waits for its answer', async () => {
        vi.useFakeTimers();
        try {
            const { rerender } = render(CollapsibleSectionInnerFormHarness, { props: { lang: 'ru' } });
            await fireEvent.submit(screen.getByTestId('the-section-form'));
            await vi.advanceTimersByTimeAsync(2_000);
            expect(screen.getByTestId('client-inn')).not.toHaveFocus();
            await rerender({ lang: 'ru', refusal: 'Ten or twelve digits.' });
            await vi.advanceTimersByTimeAsync(0);
            expect(screen.getByTestId('client-inn')).toHaveFocus();
        } finally {
            vi.useRealTimers();
        }
    });

    it('leaves the cursor in the box being typed in when a refusal appears after the person started typing again', async () => {
        const { rerender } = render(CollapsibleSectionInnerFormHarness, { props: { lang: 'ru' } });
        await fireEvent.submit(screen.getByTestId('the-section-form'));
        const typing = screen.getByTestId('the-section-submit');
        typing.focus();
        await fireEvent.input(screen.getByTestId('client-inn'));
        await rerender({ lang: 'ru', refusal: 'Ten or twelve digits.' });
        await new Promise((resolve) => setTimeout(resolve, 700));

        expect(screen.getByTestId('client-inn')).not.toHaveFocus();
        expect(typing).toHaveFocus();
    });

    it('leaves the cursor where it is when a refusal is drawn after the send stopped waiting', async () => {
        vi.useFakeTimers();
        try {
            const { rerender } = render(CollapsibleSectionInnerFormHarness, { props: { lang: 'ru' } });
            await fireEvent.submit(screen.getByTestId('the-section-form'));
            await vi.advanceTimersByTimeAsync(16_000);
            await rerender({ lang: 'ru', refusal: 'Ten or twelve digits.' });
            await vi.advanceTimersByTimeAsync(0);
            expect(screen.getByTestId('client-inn')).not.toHaveFocus();
        } finally {
            vi.useRealTimers();
        }
    });
});
