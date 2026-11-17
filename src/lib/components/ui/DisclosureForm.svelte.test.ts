import { flushTicks } from '$test/support/superform-answer';
import DisclosureFormHelperFormTestHarness from '$test/harnesses/ui/DisclosureFormHelperFormTestHarness.svelte';
import DisclosureFormTestHarness from '$test/harnesses/ui/DisclosureFormTestHarness.svelte';
import { fireEvent, render, screen } from '@testing-library/svelte';
import { tick } from 'svelte';
import { writable } from 'svelte/store';
import { describe, expect, it, vi } from 'vitest';

/** What every form of the helper-form harness handed to superforms: the library itself is not run here. */
const library = vi.hoisted(() => ({ options: [] as Record<string, unknown>[] }));
vi.mock('sveltekit-superforms', async (original) => ({
    ...(await original<typeof import('sveltekit-superforms')>()),
    superForm: (_form: unknown, options: Record<string, unknown>) => {
        library.options.push(options);
        return {
            form: writable({}),
            errors: writable({}),
            message: writable(undefined),
            submitting: writable(false),
            formId: writable('form'),
            options: {},
            enhance: () => ({ destroy() {} }),
            reset: () => {},
            validate: () => undefined
        };
    }
}));
vi.mock('$app/navigation', async () => (await import('$test/support/navigation')).navigationModule);

/** Answers one of the harness's forms with a success, as superforms reports it. */
function succeed(options: Record<string, unknown> | undefined): void {
    (options?.onUpdated as (event: unknown) => void)({ form: { valid: true, message: undefined, errors: {} } });
}

/**
 * A form opened by a press. The policy test (`disclosure-form-policy.test.ts`)
 * makes every such form of the interface this component, so no screen tests its opener, its «Закрыть» or its question.
 */

async function openByPress(): Promise<void> {
    await fireEvent.click(screen.getByTestId('edit'));
    await flushTicks();
}

describe('DisclosureForm', () => {
    it('hides its opener while open, puts the cursor in the first field and «Закрыть» beside the main button', async () => {
        render(DisclosureFormTestHarness);
        const opener = screen.getByTestId('edit');
        expect(opener).toHaveTextContent('Изменить');
        expect(opener).toHaveAttribute('aria-expanded', 'false');
        expect(opener).toHaveAttribute('aria-controls');
        expect(screen.queryByTestId('edit-form')).toBeNull();

        await openByPress();
        expect(screen.queryByTestId('edit')).toBeNull();
        expect(document.activeElement).toBe(screen.getByTestId('title'));
        const close = screen.getByTestId('edit-form-close');
        expect(close).toHaveTextContent('Закрыть');
        expect(close.parentElement).toBe(screen.getByTestId('succeed').parentElement);
    });

    it('closes at once when nothing was changed and gives the cursor back to the opener', async () => {
        const onClose = vi.fn();
        render(DisclosureFormTestHarness, { props: { onClose } });
        await openByPress();
        await fireEvent.click(screen.getByTestId('edit-form-close'));
        await flushTicks();
        expect(screen.queryByTestId('edit-form')).toBeNull();
        expect(screen.queryByTestId('edit-form-unsaved')).toBeNull();
        expect(document.activeElement).toBe(screen.getByTestId('edit'));
        expect(onClose).toHaveBeenCalledOnce();
    });

    it('asks before dropping a change it reads from the page, and «Вернуться к форме» keeps the form', async () => {
        render(DisclosureFormTestHarness);
        await openByPress();
        const box = screen.getByTestId('title') as HTMLInputElement;
        box.value = 'Yak';
        await fireEvent.input(box);
        (screen.getByTestId('hidden') as HTMLInputElement).value = 'p2';

        await fireEvent.click(screen.getByTestId('edit-form-close'));
        await flushTicks();
        const question = screen.getByTestId('edit-form-unsaved');
        expect(question).toHaveTextContent('Изменения не сохранены. Закрыть без сохранения?');
        expect(screen.getByTestId('edit-form-unsaved-confirm')).toHaveTextContent('Закрыть без сохранения');
        await fireEvent.click(screen.getByTestId('edit-form-unsaved-cancel'));
        await flushTicks();
        expect(screen.getByTestId('title')).toHaveValue('Yak');

        await fireEvent.click(screen.getByTestId('edit-form-close'));
        await flushTicks();
        await fireEvent.click(screen.getByTestId('edit-form-unsaved-confirm'));
        await flushTicks();
        expect(screen.queryByTestId('edit-form')).toBeNull();
    });

    it('closes at once when «Закрыть» comes before the opening values are read', async () => {
        render(DisclosureFormTestHarness);
        await fireEvent.click(screen.getByTestId('edit'));
        await tick();
        await fireEvent.click(screen.getByTestId('edit-form-close'));
        await flushTicks();
        expect(screen.queryByTestId('edit-form-unsaved')).toBeNull();
        expect(screen.queryByTestId('edit-form')).toBeNull();
    });

    it('sees an unticked box as a change too', async () => {
        render(DisclosureFormTestHarness);
        await openByPress();
        await fireEvent.click(screen.getByTestId('active'));
        await fireEvent.click(screen.getByTestId('edit-form-close'));
        await flushTicks();
        expect(screen.getByTestId('edit-form-unsaved')).toBeInTheDocument();
    });

    it('an edit form closes after a successful save and says so; a refusal keeps it open with what was typed', async () => {
        render(DisclosureFormTestHarness, { props: { after: 'close' } });
        await openByPress();
        const box = screen.getByTestId('title') as HTMLInputElement;
        box.value = 'Yak';
        await fireEvent.input(box);
        await fireEvent.click(screen.getByTestId('refuse'));
        await flushTicks();
        expect(screen.getByTestId('title')).toHaveValue('Yak');

        await fireEvent.click(screen.getByTestId('succeed'));
        await flushTicks();
        expect(screen.queryByTestId('edit-form')).toBeNull();
        expect(screen.getByTestId('edit-form-saved')).toHaveTextContent('Сохранено.');
        await openByPress();
        expect(screen.queryByTestId('edit-form-saved')).toBeNull();
    });

    it('an add form stays open after a success, and what it shows then counts as unchanged', async () => {
        render(DisclosureFormTestHarness, { props: { after: 'stay' } });
        await openByPress();
        const box = screen.getByTestId('title') as HTMLInputElement;
        box.value = '';
        await fireEvent.input(box);
        await fireEvent.click(screen.getByTestId('succeed'));
        await flushTicks();
        expect(screen.getByTestId('edit-form')).toBeInTheDocument();
        await fireEvent.click(screen.getByTestId('edit-form-close'));
        await flushTicks();
        expect(screen.queryByTestId('edit-form-unsaved')).toBeNull();
        expect(screen.queryByTestId('edit-form')).toBeNull();
    });

    it('stays open with what was typed when a helper form inside it succeeds; its own form still closes it', async () => {
        library.options = [];
        render(DisclosureFormHelperFormTestHarness);
        await openByPress();
        const box = screen.getByTestId('title') as HTMLInputElement;
        box.value = 'Yak';
        await fireEvent.input(box);
        const [helper, own] = library.options;
        expect(helper).toMatchObject({ dataType: 'json' });

        succeed(helper);
        await flushTicks();
        expect(screen.getByTestId('edit-form')).toBeInTheDocument();
        expect(screen.getByTestId('title')).toHaveValue('Yak');
        expect(screen.queryByTestId('edit-form-saved')).toBeNull();

        succeed(own);
        await flushTicks();
        expect(screen.queryByTestId('edit-form')).toBeNull();
        expect(screen.getByTestId('edit-form-saved')).toHaveTextContent('Сохранено.');
    });

    it('opens from code, and draws its own «Закрыть» under a form that placed none', async () => {
        render(DisclosureFormTestHarness, { props: { withClose: false } });
        await fireEvent.click(screen.getByTestId('open-from-code'));
        await flushTicks();
        expect(screen.getByTestId('edit-form')).toBeInTheDocument();
        expect(screen.getAllByText('Закрыть')).toHaveLength(1);
        await fireEvent.click(screen.getByTestId('edit-form-close'));
        await flushTicks();
        expect(screen.getByTestId('edit')).toBeInTheDocument();
    });

    it('draws nothing while closed without an opener of its own, so an empty block takes no gap in its column', async () => {
        const { container } = render(DisclosureFormTestHarness, { props: { showOpener: false } });
        expect(container.querySelector('[data-disclosure-form]')).toBeNull();

        await fireEvent.click(screen.getByTestId('open-from-code'));
        await flushTicks();
        expect(container.querySelector('[data-disclosure-form]')).toHaveAttribute('data-disclosure-form', 'open');
        expect(screen.getByTestId('edit-form')).toBeInTheDocument();
    });

    it('draws its opener as a medium button of its own width by default', () => {
        render(DisclosureFormTestHarness);
        const opener = screen.getByTestId('edit');
        expect(opener).not.toHaveClass('w-full');
        expect(opener.parentElement).toHaveClass('flex-wrap');
    });

    it('draws a large opener that fills its place when asked, «Сохранено.» still beside it', () => {
        const small = render(DisclosureFormTestHarness);
        const mediumHeight = screen.getByTestId('edit').className;
        small.unmount();

        render(DisclosureFormTestHarness, { props: { size: 'lg', fullWidth: true, saved: true } });
        const opener = screen.getByTestId('edit');
        expect(opener).toHaveClass('w-full');
        expect(opener.className).not.toBe(mediumHeight);
        expect(opener.parentElement).not.toHaveClass('flex-wrap');
        expect(screen.getByTestId('edit-form-saved')).toHaveTextContent('Сохранено.');
    });

    it('says «Сохранено.» when the screen closed the form itself and sets saved; opening again takes it away', async () => {
        render(DisclosureFormTestHarness);
        expect(screen.queryByTestId('edit-form-saved')).toBeNull();
        await openByPress();
        await fireEvent.click(screen.getByTestId('close-from-code'));
        await flushTicks();
        expect(screen.getByTestId('edit-form-saved')).toHaveTextContent('Сохранено.');

        await openByPress();
        expect(screen.queryByTestId('edit-form-saved')).toBeNull();
    });
});
