import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import { messages } from '$lib/i18n/messages';
import { resetPage, setPageUrl } from '$test/support/page-state.svelte';
import ShowcaseForms from './ShowcaseForms.svelte';

vi.mock('$app/state', async () => {
    const state = await import('$test/support/page-state.svelte');
    return { page: state.page, navigating: state.navigating };
});
vi.mock('$app/navigation', async () => (await import('$test/support/navigation')).navigationModule);

const text = (key: string): string => messages.ru[key];

describe('ShowcaseForms', () => {
    it('draws every group of the windows, fields and forms, the dimmed and the busy controls among them', () => {
        resetPage();
        render(ShowcaseForms);

        for (const group of [
            'fields',
            'boxes',
            'numbers',
            'answers',
            'form',
            'windows',
            'sections',
            'secret',
            'dimming'
        ]) {
            expect(screen.getByTestId(`showcase-forms-${group}`)).toBeInTheDocument();
        }
        expect(screen.getByTestId('showcase-forms-form')).toContainElement(screen.getByTestId('real-form'));
        expect(screen.getByTestId('showcase-text-disabled')).toBeDisabled();
        expect(screen.getByTestId('showcase-text-disabled')).toHaveClass('disabled:opacity-50');
        expect(screen.getByTestId('showcase-button-disabled')).toBeDisabled();
        expect(screen.getByTestId('showcase-button-busy')).toHaveAttribute('aria-busy', 'true');
        expect(screen.getByTestId('showcase-once')).toHaveTextContent(text('showcase.forms.secret.note'));
    });

    it('opens a window from its button and closes it again with Escape', async () => {
        resetPage();
        render(ShowcaseForms);
        expect(screen.queryByTestId('showcase-window')).toBeNull();

        await fireEvent.click(screen.getByTestId('showcase-open-window'));
        expect(screen.getByTestId('showcase-window')).toBeInTheDocument();
        await fireEvent.keyDown(screen.getByTestId('showcase-window'), { key: 'Escape' });
        await waitFor(() => expect(screen.queryByTestId('showcase-window')).toBeNull());
    });

    it('opens the record panel at once when the address names the record', () => {
        resetPage();
        setPageUrl('/showcase?edit=sample');
        render(ShowcaseForms);

        expect(screen.getByTestId('showcase-row-panel')).toBeInTheDocument();
    });
});
