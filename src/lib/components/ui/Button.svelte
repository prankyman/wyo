<script lang="ts">
    import SpinnerGapIcon from 'phosphor-svelte/lib/SpinnerGapIcon';
    import { formGuardOf } from './form-guard.svelte';
    import { isNavigatingTo } from './page-change.svelte';
    import { buttonClasses } from './press';
    import type { IButtonProps } from './types';

    /**
     * Every button of the application, and every link that looks like one (constitution, press feedback and single submission).
     *
     * - Pressed: it shrinks and darkens under the finger or the mouse (`press.ts`).
     * - Working: a spinner, `aria-busy`, and it cannot fire again. It is working when
     *   `pending` is set (an action the screen runs itself), when it submitted a `GuardedForm`
     *   that has not had its answer yet, or — as a link — while SvelteKit is on its way to its
     *   `href`. The other submit buttons of a submitting form are blocked too, without a spinner.
     * - Working never sets `disabled`: a disabled submitter is left out of the form data, so a
     *   button's `name`/`value` would not reach the server. It is `aria-disabled` instead, and a
     *   click in the capture phase — before SvelteKit's link handling and before the form is
     *   submitted — is swallowed.
     */
    let {
        variant = 'primary',
        size = 'md',
        fullWidth = false,
        type = 'button',
        name,
        value,
        form,
        formaction,
        formmethod,
        formnovalidate,
        disabled = false,
        pending = false,
        href,
        target,
        rel,
        download,
        hreflang,
        dataTest,
        ariaLabel,
        id,
        class: className,
        onclick,
        icon,
        children,
        element = $bindable(),
        ...rest
    }: IButtonProps = $props();

    const isLink = $derived(href !== undefined);
    /** A download or a new window leaves this page where it is: nothing to wait for. */
    const leavesPageAlone = $derived(Boolean(download) || Boolean(target && target !== '_self'));

    const guard = $derived.by(() => {
        // Read so a changed `form` attribute re-resolves the owner.
        void form;
        // By tag, not `instanceof`: this also runs on the server, where DOM classes do not exist.
        return element?.tagName === 'BUTTON' ? formGuardOf((element as HTMLButtonElement).form) : undefined;
    });
    const submittedByThis = $derived(Boolean(guard?.submitting) && guard?.submitter === element);
    const formBusy = $derived(Boolean(guard?.submitting) && type === 'submit');
    const openingHref = $derived(isLink && !leavesPageAlone && !disabled && isNavigatingTo(href));

    const working = $derived(!disabled && (pending || submittedByThis || openingHref));
    const blocked = $derived(!disabled && (working || formBusy));

    /** `download={true}` is the bare attribute; `false` must leave it out, not write "false". */
    const downloadAttribute = $derived(download === true ? '' : download || undefined);

    const classes = $derived(
        buttonClasses({ variant, size, fullWidth, iconOnly: Boolean(icon) && !children, extra: className })
    );

    /** Capture phase: runs before SvelteKit's link handler and before the form is submitted. */
    function swallowWhileBlocked(event: MouseEvent): void {
        if (!blocked) return;
        event.preventDefault();
        event.stopImmediatePropagation();
    }
</script>

{#snippet content()}
    {#if working}
        <SpinnerGapIcon
            class="size-4 shrink-0 motion-safe:animate-spin"
            aria-hidden="true"
            data-test="button-spinner"
        />
    {:else if icon}
        {@render icon()}
    {/if}
    {#if children}{@render children()}{/if}
{/snippet}

{#if isLink}
    <!-- A link that cannot be followed has no href: it leaves the tab order and cannot be opened. -->
    <a
        bind:this={element}
        href={disabled ? undefined : href}
        {target}
        {rel}
        download={downloadAttribute}
        {hreflang}
        {id}
        class={classes}
        data-test={dataTest}
        aria-label={ariaLabel}
        aria-disabled={disabled || blocked || undefined}
        aria-busy={working || undefined}
        onclickcapture={swallowWhileBlocked}
        {onclick}
        {...rest}
    >
        {@render content()}
    </a>
{:else}
    <button
        bind:this={element}
        {type}
        {name}
        {value}
        {form}
        {formaction}
        {formmethod}
        {formnovalidate}
        {disabled}
        {id}
        class={classes}
        data-test={dataTest}
        aria-label={ariaLabel}
        aria-disabled={blocked || undefined}
        aria-busy={working || undefined}
        onclickcapture={swallowWhileBlocked}
        {onclick}
        {...rest}
    >
        {@render content()}
    </button>
{/if}
