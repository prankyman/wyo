import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import ConfirmationModal from './ConfirmationModal.svelte';
import FormHarness from '$test/harnesses/ui/ConfirmationModalFormTestHarness.svelte';

describe('ConfirmationModal', () => {
    it('renders nothing when open is false', () => {
        render(ConfirmationModal, {
            open: false,
            title: 'Test Title',
            message: 'Test Message'
        });

        const modal = document.body.querySelector('[role="dialog"]');
        expect(modal).not.toBeInTheDocument();
    });

    it('renders an accessible dialog carrying the title, message and default buttons, and nothing else', () => {
        render(ConfirmationModal, {
            open: true,
            title: 'Delete Item',
            message: 'Are you sure you want to delete this item?'
        });

        const modal = document.body.querySelector('[role="dialog"]');
        expect(modal).toBeInTheDocument();
        expect(modal).toHaveAttribute('aria-modal', 'true');
        expect(modal).toHaveAttribute('tabindex', '-1');

        expect(screen.getByText('Delete Item')).toBeInTheDocument();
        expect(screen.getByText('Are you sure you want to delete this item?')).toBeInTheDocument();
        expect(screen.getByText('Подтвердить')).toBeInTheDocument();
        expect(screen.getByText('Отмена')).toBeInTheDocument();

        // With no children passed, the window's body holds the sentence alone; the buttons are the window's footer.
        const sentence = screen.getByText('Are you sure you want to delete this item?');
        expect(sentence.parentElement!.children).toHaveLength(1);
        expect(screen.getByText('Подтвердить').closest('[role="dialog"] > div > div:last-child')).not.toBeNull();
    });

    it('overrides both button labels when confirmText and cancelText are given', () => {
        render(ConfirmationModal, {
            open: true,
            title: 'Test',
            message: 'Test',
            confirmText: 'Delete',
            cancelText: 'Close'
        });

        expect(screen.getByText('Delete')).toBeInTheDocument();
        expect(screen.getByText('Close')).toBeInTheDocument();
    });

    it('calls onConfirm when confirm button is clicked', async () => {
        const onConfirm = vi.fn();
        render(ConfirmationModal, {
            open: true,
            title: 'Test',
            message: 'Test',
            onConfirm
        });

        const confirmButton = screen.getByText('Подтвердить');
        confirmButton.click();

        expect(onConfirm).toHaveBeenCalledTimes(1);
    });

    it('calls onCancel when the cancel button is clicked', async () => {
        const onCancel = vi.fn();
        render(ConfirmationModal, {
            open: true,
            title: 'Test',
            message: 'Test',
            onCancel
        });

        const cancelButton = screen.getByText('Отмена');
        cancelButton.click();

        expect(onCancel).toHaveBeenCalledTimes(1);
    });

    it('calls onCancel when Escape is pressed on the modal', async () => {
        const onCancel = vi.fn();
        render(ConfirmationModal, {
            open: true,
            title: 'Test',
            message: 'Test',
            onCancel
        });

        const modal = document.body.querySelector('[role="dialog"]')!;
        await fireEvent.keyDown(modal, { key: 'Escape' });

        expect(onCancel).toHaveBeenCalledTimes(1);
    });

    it('wires either button up as a submit for its own external form when configured', () => {
        render(ConfirmationModal, {
            open: true,
            title: 'Test',
            message: 'Test',
            confirmType: 'submit',
            confirmForm: 'accept-form',
            cancelType: 'submit',
            cancelForm: 'decline-form'
        });

        expect(screen.getByText('Подтвердить')).toHaveAttribute('type', 'submit');
        expect(screen.getByText('Подтвердить')).toHaveAttribute('form', 'accept-form');
        expect(screen.getByText('Отмена')).toHaveAttribute('type', 'submit');
        expect(screen.getByText('Отмена')).toHaveAttribute('form', 'decline-form');
    });

    it('does not call onCancel on Escape when closeOnEscape is false', async () => {
        const onCancel = vi.fn();
        render(ConfirmationModal, {
            open: true,
            title: 'Test',
            message: 'Test',
            onCancel,
            closeOnEscape: false
        });

        const modal = document.body.querySelector('[role="dialog"]')!;
        await fireEvent.keyDown(modal, { key: 'Escape' });

        expect(onCancel).not.toHaveBeenCalled();
    });

    it('shows the confirm working while the form it submits waits for its answer, and sends that form once', async () => {
        const sent: string[] = [];
        const record = (event: Event) => sent.push(String(new FormData(event.target as HTMLFormElement).get('taskId')));
        document.addEventListener('submit', record);
        render(FormHarness);

        const confirm = screen.getByTestId('task-cancel-modal-confirm');
        await fireEvent.click(confirm);
        await fireEvent.click(confirm);

        expect(sent).toEqual(['t-1']);
        expect(confirm).toHaveAttribute('aria-busy', 'true');
        expect(confirm).not.toBeDisabled();
        expect(screen.getByTestId('task-cancel-modal-cancel')).not.toHaveAttribute('aria-busy');
        document.removeEventListener('submit', record);
    });

    it('disables the confirm button when confirmDisabled is true', () => {
        render(ConfirmationModal, {
            open: true,
            title: 'Test',
            message: 'Test',
            confirmDisabled: true
        });

        expect(screen.getByText('Подтвердить')).toBeDisabled();
    });

    it('shows the press state while its form is sending and closes when the answer arrives', async () => {
        const { rerender } = render(ConfirmationModal, {
            open: true,
            title: 'Test',
            message: 'Test',
            confirmType: 'submit',
            confirmForm: 'some-form',
            sending: true
        });

        expect(screen.getByText('Подтвердить').closest('button')).toHaveAttribute('aria-busy', 'true');

        await rerender({ sending: false });

        expect(document.body.querySelector('[role="dialog"]')).not.toBeInTheDocument();
    });

    it('stays open when its form has not been sent yet', async () => {
        const { rerender } = render(ConfirmationModal, {
            open: true,
            title: 'Test',
            message: 'Test',
            confirmType: 'submit',
            confirmForm: 'some-form',
            sending: false
        });

        await rerender({ title: 'Changed' });

        expect(document.body.querySelector('[role="dialog"]')).toBeInTheDocument();
    });
});
