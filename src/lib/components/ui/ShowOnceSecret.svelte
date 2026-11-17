<script lang="ts">
    import type { Snippet } from 'svelte';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import CopyButton from './CopyButton.svelte';
    import { COPY_BOX_CLASS } from './field-classes';
    import HeadingWithHelp from './HeadingWithHelp.svelte';
    import HelpTooltip from './HelpTooltip.svelte';
    import NoteBox from './NoteBox.svelte';
    import Stack from './Stack.svelte';

    /**
     * The box of a secret shown ONCE: a key, a token, a signing secret. The interface keeps only a fingerprint of such a
     * secret, so the box says it is shown this one time and draws every value in a box that can be selected, each with its copy
     * button and the words of its answer. What is the caller's own — the install commands, the steps to take with the secret —
     * is its `children`, drawn under the values.
     */
    interface IItem {
        /** What the value is, drawn over it; a value with no label is named by the box's title. */
        label?: string;
        value: string;
        /** `data-test` of the value's box; its copy button is `<testId>-copy` and its «?» `<testId>-help`. `<dataTest>-value-<n>` when left out. */
        testId?: string;
        /** The text of a «?» beside the label, already translated. */
        help?: string;
        /** The copy button's words, already translated; the shared «Скопировать» when left out. */
        copyLabel?: string;
    }

    interface IProps {
        lang: TLocale;
        /** The values, in the order they are drawn. */
        items: IItem[];
        /** The note that they are shown once and cannot be read again, already translated. */
        note: string;
        /** A heading over the note; with `help` it carries a «?» (`<dataTest>-help`). */
        title?: string;
        help?: string;
        /** `success` status by default; `none` when the page announces the change itself. */
        role?: 'status' | 'none';
        /** The box is `<dataTest>`, its note `<noteTestId>`, `<dataTest>-once` when left out. */
        dataTest: string;
        noteTestId?: string;
        /** The caller's own content, drawn under the values. */
        children?: Snippet;
    }

    let { lang, items, note, title, help, role, dataTest, noteTestId, children }: IProps = $props();
</script>

<NoteBox tone="success" {role} {dataTest} class="p-3">
    <Stack>
        {#if title}
            <HeadingWithHelp text={title} {help} look="section" testId="{dataTest}-help" />
        {/if}
        <p class="text-sm font-semibold text-stop-800" data-test={noteTestId ?? `${dataTest}-once`}>{note}</p>
        {#each items as item, index (index)}
            {@const valueTestId = item.testId ?? `${dataTest}-value-${index}`}
            <Stack>
                {#if item.label}
                    <div class="flex items-center gap-1 text-sm text-ink-800">
                        {item.label}
                        {#if item.help}
                            <HelpTooltip tooltip={item.help} ariaLabel={item.label} testId="{valueTestId}-help" />
                        {/if}
                    </div>
                {/if}
                <code class={COPY_BOX_CLASS} data-test={valueTestId}>{item.value}</code>
                {#key item.value}
                    <CopyButton
                        text={item.value}
                        label={item.copyLabel ?? t('app.ui.showOnce.copy', lang)}
                        copiedLabel={t('app.ui.showOnce.copied', lang)}
                        failedLabel={t('app.ui.showOnce.copyFailed', lang)}
                        dataTest="{valueTestId}-copy"
                    />
                {/key}
            </Stack>
        {/each}
        {@render children?.()}
    </Stack>
</NoteBox>
