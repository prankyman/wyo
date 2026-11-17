import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import FormControlTestHarness from '$test/harnesses/ui/FormControlTestHarness.svelte';
import TextareaTestHarness from '$test/harnesses/ui/TextareaTestHarness.svelte';
import Textarea from './Textarea.svelte';

describe('Textarea', () => {
    it('renders bound value', () => {
        render(TextareaTestHarness, { initialData: { note: 'hello world' }, name: 'note', lang: 'ru' });
        const textarea = screen.getByRole('textbox');
        expect(textarea).toHaveValue('hello world');
    });

    it('does not render a counter when maxlength is not set', () => {
        render(TextareaTestHarness, { initialData: { note: 'hi' }, name: 'note', lang: 'ru' });
        expect(screen.queryByTestId('note-counter')).not.toBeInTheDocument();
    });

    it('renders the counter at 2/2000, unstyled and without an error, while under maxlength', () => {
        render(TextareaTestHarness, { initialData: { note: 'hi' }, name: 'note', lang: 'ru', maxlength: 2000 });

        expect(screen.getByTestId('note-counter-count')).toHaveTextContent('2');
        expect(screen.getByTestId('note-counter')).toHaveTextContent('2/2000');
        expect(screen.getByTestId('note-counter-count')).not.toHaveClass('text-stop-600');
        expect(screen.queryByTestId('note-error')).not.toBeInTheDocument();
    });

    it('turns the counter red when over maxlength', () => {
        render(TextareaTestHarness, {
            initialData: { note: 'a'.repeat(2001) },
            name: 'note',
            lang: 'ru',
            maxlength: 2000
        });
        expect(screen.getByTestId('note-counter-count')).toHaveClass('text-stop-600');
    });

    it('does not truncate typed input beyond maxlength and turns the counter red', async () => {
        render(TextareaTestHarness, { initialData: { note: '' }, name: 'note', lang: 'ru', maxlength: 10 });
        const textarea = screen.getByRole('textbox');
        const longValue = 'a'.repeat(15);

        await fireEvent.input(textarea, { target: { value: longValue } });

        expect(textarea).toHaveValue(longValue);
        expect(screen.getByTestId('note-counter-count')).toHaveTextContent('15');
        expect(screen.getByTestId('note-counter-count')).toHaveClass('text-stop-600');
    });

    it('does not set a native maxlength attribute on the textarea', () => {
        render(TextareaTestHarness, { initialData: { note: '' }, name: 'note', lang: 'ru', maxlength: 10 });
        expect(screen.getByRole('textbox')).not.toHaveAttribute('maxlength');
    });

    it('shows the error message, and only the error, when the field has an error and no maxlength', () => {
        render(TextareaTestHarness, {
            initialData: { note: '' },
            name: 'note',
            lang: 'ru',
            initialErrors: { note: ['auth.passwordTooLong'] }
        });

        expect(screen.getByTestId('note-error')).toBeInTheDocument();
        expect(screen.queryByTestId('note-counter')).not.toBeInTheDocument();
    });

    it('does not show an error message when the field has no error', () => {
        render(TextareaTestHarness, { initialData: { note: '' }, name: 'note', lang: 'ru' });
        expect(screen.queryByTestId('note-error')).not.toBeInTheDocument();
    });

    it('respects the disabled prop', () => {
        render(TextareaTestHarness, { initialData: { note: '' }, name: 'note', lang: 'ru', disabled: true });
        expect(screen.getByRole('textbox')).toBeDisabled();
    });

    it('applies resize-y classes instead of resize-none', () => {
        render(TextareaTestHarness, { initialData: { note: '' }, name: 'note', lang: 'ru' });
        const textarea = screen.getByRole('textbox');
        expect(textarea.className).toContain('resize-y');
        expect(textarea.className).toContain('max-h-250');
        expect(textarea.className).not.toContain('resize-none');
    });

    it('uses a custom testId for the textarea when provided', () => {
        render(TextareaTestHarness, { initialData: { note: '' }, name: 'note', lang: 'ru', testId: 'custom' });
        expect(screen.getByTestId('custom')).toBeInTheDocument();
    });

    it('renders the counter and error on the same row when both are present', () => {
        render(TextareaTestHarness, {
            initialData: { note: '' },
            name: 'note',
            lang: 'ru',
            maxlength: 2000,
            initialErrors: { note: ['auth.passwordTooLong'] }
        });
        const counter = screen.getByTestId('note-counter');
        const error = screen.getByTestId('note-error');
        expect(counter.parentElement).toBe(error.parentElement);
    });
});

describe('Textarea without a superform', () => {
    it('binds what is typed, keeps its rows and draws the interface’s text-area look', async () => {
        render(FormControlTestHarness, { props: { control: 'textarea', initial: 'Хрупкое' } });
        const box = screen.getByTestId('box');

        expect(box).toHaveValue('Хрупкое');
        expect(box).toHaveAttribute('name', 'note');
        expect(box).toHaveAttribute('rows', '4');
        expect(box).toHaveClass('rounded', 'border-ink-300', 'px-3', 'py-2', 'resize-y');
        expect(box).not.toHaveAttribute('aria-invalid');
        await fireEvent.input(box, { target: { value: 'Стекло' } });
        expect(screen.getByTestId('bound')).toHaveTextContent('Стекло');
    });

    it('belongs to a form drawn elsewhere when given its id', () => {
        render(Textarea, { props: { name: 'reason', formId: 'cancel-form', dataTest: 'reason' } });

        expect(screen.getByTestId('reason')).toHaveAttribute('form', 'cancel-form');
    });

    it('marks a refused value and merges the caller’s classes over the look', () => {
        render(FormControlTestHarness, { props: { control: 'textarea', invalid: true, class: 'text-base' } });
        const box = screen.getByTestId('box');

        expect(box).toHaveAttribute('aria-invalid', 'true');
        expect(box).toHaveClass('border-stop-600', 'text-base');
        expect(box).not.toHaveClass('text-sm');
    });
});
