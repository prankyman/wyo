<script lang="ts">
    import type { Snippet } from 'svelte';
    import HelpTooltip from './HelpTooltip.svelte';
    import type { IHelpLink } from './types';

    /**
     * One labelled field: the label in normal letters, its
     * «?», the control the caller passes, and the refusal under it. Every labelled field of the interface is drawn
     * through it, so the label's look is decided here once and is not a prop.
     */
    interface IProps {
        /** The field's name as the person reads it, already translated. */
        label?: string;
        /** A label with more than words in it (a sentence with a link, two lines) — instead of `label`. */
        labelContent?: Snippet;
        /** The text behind the «?» beside the label; without it there is no «?». */
        help?: string;
        /** Links under the «?»'s text, each opening in a new tab. */
        helpLinks?: IHelpLink[];
        /**
         * Short marks drawn on the label's row after the label and its «?», outside the `<label>` (a setting's «по
         * умолчанию» and «не сохранено»): on a row of their own they would push the box away from its label. Only with `for`.
         */
        aside?: Snippet;
        /** The refusal of the field, already worded; nothing is drawn while it is empty. */
        error?: string | null;
        /**
         * The control's `id`: the label points at it, so a tap on the label puts the caret in the box. Without it the
         * label holds the control.
         */
        for?: string;
        /**
         * `stacked` — the label above the box (the default); `inline` — a tick box or a radio first, its words beside it,
         * the row at least 44 px tall; `row` — the label and the box on one line.
         */
        layout?: 'stacked' | 'inline' | 'row';
        /** The label is read by a screen reader but not drawn (a box whose purpose the screen already shows). */
        labelHidden?: boolean;
        /** The control cannot be used: the label is greyed with it. */
        disabled?: boolean;
        /** `<testId>-label`, `<testId>-help` and `<testId>-error` name the parts; each can be given on its own. */
        testId?: string;
        labelTestId?: string;
        helpTestId?: string;
        errorTestId?: string;
        /** `data-test` and `data-tour` of the whole field. */
        dataTest?: string;
        dataTour?: string;
        /** Classes of the whole field: its place in a grid (`md:col-span-2`), its width (`w-32`), its spacing. */
        class?: string;
        children: Snippet;
    }

    let {
        label,
        labelContent,
        help,
        helpLinks,
        aside,
        error = null,
        for: htmlFor,
        layout = 'stacked',
        labelHidden = false,
        disabled = false,
        testId,
        labelTestId,
        helpTestId,
        errorTestId,
        dataTest,
        dataTour,
        class: className = '',
        children
    }: IProps = $props();

    const labelId = $derived(labelTestId ?? (testId ? `${testId}-label` : undefined));
    const helpId = $derived(helpTestId ?? (testId ? `${testId}-help` : 'field-help'));
    const errorId = $derived(errorTestId ?? (testId ? `${testId}-error` : undefined));
    const dim = $derived(disabled ? 'opacity-50' : '');
    /** The look of a label above or beside a box: small, grey, in the letters it is written in. */
    const nameClass = $derived(`flex min-w-0 items-center text-xs text-ink-600 ${labelHidden ? 'sr-only' : ''} ${dim}`);
</script>

{#snippet name()}
    {#if labelContent}{@render labelContent()}{:else}{label}{/if}
{/snippet}

{#snippet helpButton()}
    {#if help}
        <HelpTooltip tooltip={help} links={helpLinks} ariaLabel={help} testId={helpId} />
    {/if}
{/snippet}

{#snippet refusal()}
    {#if error}
        <span role="alert" class="text-xs text-stop-700" data-test={errorId}>{error}</span>
    {/if}
{/snippet}

<!--
    A field showing a refusal carries `data-field-error`: a closed section around it opens and the first such box is
    focused. The gap between the label and its box is `gap-1.5` on every field ("Forms", rule 8).
-->
{#if layout === 'inline'}
    <div
        class="flex flex-col gap-1 {className}"
        data-field-error={error ? '' : undefined}
        data-test={dataTest}
        data-tour={dataTour}
    >
        <div class="flex items-center">
            <label
                for={htmlFor}
                class="flex min-h-11 min-w-11 items-center gap-2 text-sm text-ink-700 {disabled
                    ? 'cursor-not-allowed'
                    : ''} {dim}"
                data-test={labelId}
            >
                {@render children()}
                <span class={labelHidden ? 'sr-only' : 'min-w-0'}>{@render name()}</span>
            </label>
            {@render helpButton()}
        </div>
        {@render refusal()}
    </div>
{:else if htmlFor}
    <div
        class="flex {layout === 'row' ? 'flex-wrap items-center gap-2' : 'flex-col gap-1.5'} {className}"
        data-field-error={error ? '' : undefined}
        data-test={dataTest}
        data-tour={dataTour}
    >
        {#if aside}
            <div class="flex flex-wrap items-center gap-1">
                <label for={htmlFor} class={nameClass} data-test={labelId}>
                    {@render name()}
                    {@render helpButton()}
                </label>
                {@render aside()}
            </div>
        {:else}
            <label for={htmlFor} class={nameClass} data-test={labelId}>
                {@render name()}
                {@render helpButton()}
            </label>
        {/if}
        {@render children()}
        {@render refusal()}
    </div>
{:else}
    <div
        class="flex flex-col gap-1.5 {className}"
        data-field-error={error ? '' : undefined}
        data-test={dataTest}
        data-tour={dataTour}
    >
        <!--
            The label holds the control. The «?» comes after the control in the page's order, because a label names the
            first control inside it and the «?» is a button; it is still drawn beside the words, by its place in the grid
            (stacked) or its `order` in the row. The row wraps, so a long label puts the box under it on a phone.
        -->
        {#if layout === 'row'}
            <label class="flex flex-wrap items-center gap-x-2 gap-y-1.5">
                <span class="{nameClass} order-1" data-test={labelId}>{@render name()}</span>
                <div class="order-3 flex min-w-0 flex-col">{@render children()}</div>
                {#if help}
                    <span class="order-2 flex">{@render helpButton()}</span>
                {/if}
            </label>
        {:else}
            <label class="grid grid-cols-[auto_1fr] items-center gap-y-1.5">
                <span class="{nameClass} col-start-1 row-start-1" data-test={labelId}>{@render name()}</span>
                <div class="col-span-2 col-start-1 row-start-2 flex min-w-0 flex-col">{@render children()}</div>
                {#if help}
                    <span class="col-start-2 row-start-1 flex">{@render helpButton()}</span>
                {/if}
            </label>
        {/if}
        {@render refusal()}
    </div>
{/if}
