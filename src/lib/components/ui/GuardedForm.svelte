<script lang="ts">
    import ConfirmationModal from './ConfirmationModal.svelte';
    import { FormGuard, guardSubmit } from './form-guard.svelte';
    import type { IGuardedFormProps, IGuardedSuperForm } from './types';

    /**
     * Every `<form>` of the interface (constitution, "Press feedback and single
     * submission"). Once submitted, it cannot be submitted again until its answer is in
     * or the page is replaced, and the `Button` that submitted it shows that it is working — also a
     * `Button` outside the form that submits it through `form="<id>"`, as `ConfirmationModal` does.
     *
     * The form is a superForm in the library's single-page mode (constitution, "Forms"):
     * `<GuardedForm superform={{ enhance, submitting }}>` checks its values in the browser and sends them as JSON through
     * `src/lib/api/`; nothing is posted to a page, because the wyo interface has no server side of its own.
     *
     * With a `confirm` a press of the form's submit button opens a confirmation window instead of
     * sending, and the window's red button sends the form (`confirmForm`, `sending`) — a destructive press asks first
     * without a hand-written window on the page.
     *
     * Usage and the reasons are in `docs/library-layout.md` and `form-guard.svelte.ts`.
     */
    let {
        method = 'POST',
        id,
        class: className,
        dataTest,
        superform,
        confirm,
        element = $bindable(),
        children,
        ...rest
    }: IGuardedFormProps = $props();

    const guard = new FormGuard();
    // The window's red button sends the form by its id, so a form with a window always has one.
    const generatedId = `guarded-form-${Math.random().toString(36).slice(2, 9)}`;
    const formId = $derived(id ?? (confirm ? generatedId : undefined));
    let confirmOpen = $state(false);
    let sending = $state(false);
    $effect(() => {
        if (!superform) return;
        return superform.submitting.subscribe((value) => (sending = value));
    });

    /** Applies the superForm's own `enhance` to the form; a form without a superForm stays a plain form. */
    function enhancer(form: HTMLFormElement, superform: IGuardedSuperForm | undefined) {
        let applied: { destroy(): void } | undefined;
        function apply(next: IGuardedSuperForm | undefined): void {
            applied?.destroy();
            applied = next?.enhance(form);
        }
        apply(superform);
        return {
            update: apply,
            destroy: () => applied?.destroy()
        };
    }

    interface IAskFirst {
        enabled: boolean;
        isOpen(): boolean;
        open(): void;
    }

    /**
     * Stops the press of the form's own button before the guard, the superform and the browser see it, and opens the
     * confirmation window; the window's button sends the form while the window is open. It is declared before
     * `guardSubmit`, so its capture listener runs first and `stopImmediatePropagation` keeps the others from starting.
     */
    function askFirst(form: HTMLFormElement, initial: IAskFirst) {
        let options = initial;
        function onSubmit(event: SubmitEvent): void {
            if (!options.enabled || options.isOpen()) return;
            event.preventDefault();
            event.stopImmediatePropagation();
            options.open();
        }
        form.addEventListener('submit', onSubmit, { capture: true });
        return {
            update(next: IAskFirst): void {
                options = next;
            },
            destroy: () => form.removeEventListener('submit', onSubmit, { capture: true })
        };
    }
</script>

<form
    bind:this={element}
    {method}
    id={formId}
    class={className}
    data-test={dataTest}
    aria-busy={guard.submitting || undefined}
    data-submitting={guard.submitting || undefined}
    use:askFirst={{ enabled: Boolean(confirm), isOpen: () => confirmOpen, open: () => (confirmOpen = true) }}
    use:guardSubmit={{
        guard,
        enhanced: Boolean(superform),
        submitting: superform?.submitting
    }}
    use:enhancer={superform}
    {...rest}
>
    {@render children?.()}
</form>

{#if confirm}
    <ConfirmationModal
        bind:open={confirmOpen}
        title={confirm.title}
        message={confirm.message}
        confirmText={confirm.confirmText}
        confirmType="submit"
        confirmForm={formId}
        {sending}
        dataTest={dataTest ? `${dataTest}-modal` : undefined}
    />
{/if}
