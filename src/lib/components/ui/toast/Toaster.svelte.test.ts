import { toast } from '$lib/stores/toast.svelte';
import { render, screen } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import Toaster from './Toaster.svelte';

// A language that exists only for this test, to prove the `lang` prop reaches the text system.
const TEST_LANGUAGE = vi.hoisted(() => ({ test: { 'common.close': 'Close' } }));
vi.mock('$lib/i18n/i18n', async () => (await import('$test/support/i18n')).createI18nMock({ texts: TEST_LANGUAGE }));

describe('Toaster Component', () => {
    beforeEach(() => {
        toast.clear();
    });

    it('renders no toasts when empty', () => {
        render(Toaster);
        expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });

    it('renders a success toast', () => {
        toast.success('Success message');
        render(Toaster);

        expect(screen.getByText('Success message')).toBeInTheDocument();
    });

    it('renders an error toast', () => {
        toast.error('Error message');
        render(Toaster);

        expect(screen.getByText('Error message')).toBeInTheDocument();
    });

    it('renders a warning toast', () => {
        toast.warning('Warning message');
        render(Toaster);

        expect(screen.getByText('Warning message')).toBeInTheDocument();
    });

    it('renders an info toast', () => {
        toast.info('Info message');
        render(Toaster);

        expect(screen.getByText('Info message')).toBeInTheDocument();
    });

    it('renders toast with description', () => {
        toast.success('Title', 'Description');
        render(Toaster);

        expect(screen.getByText('Title')).toBeInTheDocument();
        expect(screen.getByText('Description')).toBeInTheDocument();
    });

    it('renders multiple toasts', () => {
        toast.success('First');
        toast.error('Second');
        toast.warning('Third');
        render(Toaster);

        expect(screen.getByText('First')).toBeInTheDocument();
        expect(screen.getByText('Second')).toBeInTheDocument();
        expect(screen.getByText('Third')).toBeInTheDocument();
    });

    it('removes toast when close button is clicked', () => {
        toast.success('First');
        toast.error('Second');
        const { container } = render(Toaster, { lang: 'ru' });

        const closeButtons = container.querySelectorAll('button[aria-label="Закрыть"]');
        expect(closeButtons).toHaveLength(2);

        (closeButtons[0] as HTMLElement).click();

        // After clicking, the toast should be removed from the store
        expect(toast.toasts).toHaveLength(1);
        expect(toast.toasts[0].title).toBe('Second');
    });

    // The close cross is a 44 × 44 button around a 16 px icon, named and findable.
    it('gives each toast a close button named «Закрыть» with a 44 px tap box', () => {
        toast.success('First');
        toast.error('Second');
        render(Toaster, { lang: 'ru' });

        const closeButtons = screen.getAllByTestId('toast-close');
        expect(closeButtons).toHaveLength(2);
        for (const button of closeButtons) {
            expect(button).toHaveAccessibleName('Закрыть');
            expect(button.classList.contains('size-11')).toBe(true);
        }
    });

    // The close button is named in the reader's language, not in one language for everybody.
    it('names the close button in the language it is given', () => {
        toast.success('First');
        render(Toaster, { lang: 'test' });

        expect(screen.getByTestId('toast-close')).toHaveAccessibleName('Close');
    });

    it('applies correct variant classes', () => {
        toast.success('Success');
        toast.error('Error');
        toast.warning('Warning');
        toast.info('Info');
        const { container } = render(Toaster);

        const toasts = container.querySelectorAll('[class*="bg-"]');
        expect(toasts).toHaveLength(4);
    });
});
