<script lang="ts">
    import { answerOf } from '$lib/forms/answer';
    import { DRAWN_FIELD, type ISectionSuperForm } from '$lib/forms/section-form';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import type { Snippet } from 'svelte';
    import { tick, untrack } from 'svelte';
    import { get, readable } from 'svelte/store';
    import Button from './Button.svelte';
    import CollapsibleSection from './CollapsibleSection.svelte';
    import { focusFirstRefusedField } from './collapsible-section';
    import ConfirmationModal from './ConfirmationModal.svelte';
    import FormAnswer from './FormAnswer.svelte';
    import FormButtons from './FormButtons.svelte';
    import GuardedForm from './GuardedForm.svelte';
    import HelpTooltip from './HelpTooltip.svelte';

    interface IProps {
        lang: TLocale;
        /** The section's id (`CollapsibleSection`): its `data-test`, its address fragment; `<id>-save`, `<id>-close` are its buttons. */
        id: string;
        title: string;
        /** The section's «?», in the heading row right after the title (`CollapsibleSection`). */
        help?: string;
        helpTestId?: string;
        dataTour?: string;
        headingLevel?: 'h2' | 'h3' | 'h4';
        /** A short count or badge in the heading row, after the title. */
        summary?: Snippet;
        contentClass?: string;
        /**
         * Classes of the section's outer box (its margins). None by default, for the screens that space their sections with
         * the `gap` of a block around them; a screen that places its sections like every other section passes `mb-4`.
         */
        class?: string;
        /**
         * The create screen: nothing is stored yet, so the section is a part of the screen's ONE form — it
         * arrives open, draws no form and no buttons of its own, and the screen's one «Создать» sends every section.
         */
        create?: boolean;
        /** The section's own superform (the edit screen). */
        form?: ISectionSuperForm;
        /** False for a session that may only read the record: «Закрыть» only, no «Сохранить». */
        canWrite?: boolean;
        /** The save button's words; «Сохранить» by default. */
        saveLabel?: string;
        /** A «?» beside the save button, saying what the save does. */
        saveHelp?: string;
        children: Snippet;
    }

    let {
        lang,
        id,
        title,
        help,
        helpTestId,
        dataTour,
        headingLevel = 'h2',
        summary,
        contentClass = 'flex flex-col gap-4',
        class: className = '',
        create = false,
        form,
        canWrite = true,
        saveLabel,
        saveHelp,
        children
    }: IProps = $props();

    // A section's superform is made once by the screen; its stores are read here as they are.
    // svelte-ignore state_referenced_locally
    const stores = form;
    // A create screen's section has no form of its own: still stores, which never change.
    const message = stores?.message ?? readable<unknown>(undefined);
    const tainted = stores?.tainted ?? readable<unknown>(undefined);
    const submitting = stores?.submitting ?? readable(false);
    const changed = $derived(Boolean(stores?.isTainted($tainted)));
    const values = stores?.form ?? readable<Record<string, unknown>>({});

    let wrapper = $state<HTMLDivElement | undefined>();
    let asking = $state(false);
    let savedShown = $state(false);
    /**
     * What the heading row says after a save: «Сохранено.» for a section's ordinary save (`sectionSaved` answers `section`),
     * or the words of the key the action answered.
     */
    let savedKey = $state('section');
    /** The values the section holds as stored: those it was drawn with, then those of each successful save. */
    let stored: Record<string, unknown> = stores ? structuredClone(get(stores.form)) : {};

    /** A refusal is drawn under the buttons; «Сохранено» is in the heading row, because the section closes. */
    const refusal = $derived.by(() => {
        const answer = answerOf($message);
        return answer?.error !== undefined ? t(answer.error, lang, answer.params) : null;
    });

    function sectionElement(): HTMLDetailsElement | null {
        return wrapper?.closest('details') ?? null;
    }

    async function fold(): Promise<void> {
        const section = sectionElement();
        if (!section) return;
        section.open = false;
        await tick();
        section.querySelector<HTMLElement>('summary')?.focus();
    }

    /** «Закрыть»: folds at once when nothing changed, asks first when something did. */
    function close(): void {
        if (changed) asking = true;
        else void fold();
    }

    /** «Закрыть без сохранения»: the boxes go back to the stored values, then the section folds. */
    function discard(): void {
        asking = false;
        stores?.reset({ newState: structuredClone(stored) });
        void fold();
    }

    // A successful save says «Сохранено» and folds the section; a refusal keeps it open as it is.
    let lastMessage: unknown = untrack(() => get(message));
    $effect(() => {
        const now = $message;
        untrack(() => {
            if (now === lastMessage) return;
            lastMessage = now;
            const saved = answerOf(now)?.saved;
            if (saved === undefined) {
                // A refusal of the server: the cursor goes to the box it names, once the page has drawn its message.
                if (now !== undefined) void tick().then(() => focusFirstRefusedField(document, wrapper ?? document));
                return;
            }
            if (!stores) return;
            stored = structuredClone(get(stores.form));
            savedKey = saved;
            savedShown = true;
            void fold();
        });
    });

    // «Сохранено» stays until the section is changed again.
    $effect(() => {
        if (changed) untrack(() => (savedShown = false));
    });
</script>

<!--
    One folded section of an edit screen as ONE form of its own: its
    fields, «Сохранить» once a field of the section changed, and «Закрыть» — which folds the section at once, or asks
    «Изменения не сохранены. Закрыть без сохранения?» first when something changed. A successful save says «Сохранено» in
    the heading row and folds the section. On a create screen the section is only a part of the screen's one form.
    `src/lib/components/ui/section-form-policy.test.ts` keeps every edit screen on this rule.
-->
{#snippet heading()}
    {#if summary}{@render summary()}{/if}
    {#if savedShown}
        <span role="status" class="ml-2 text-ok-800" data-test="{id}-saved">
            {t(savedKey === 'section' ? 'common.disclosure.saved' : savedKey, lang)}
        </span>
    {/if}
{/snippet}

<CollapsibleSection
    {lang}
    {id}
    {dataTour}
    {title}
    {help}
    {helpTestId}
    {headingLevel}
    open={create}
    class={className}
    summary={summary || savedShown ? heading : undefined}
>
    {#if create || !stores}
        <div class={contentClass}>{@render children()}</div>
    {:else}
        <div bind:this={wrapper}>
            <GuardedForm superform={{ enhance: stores.enhance, submitting }} class={contentClass} dataTest="{id}-form">
                <!-- The values the section was drawn with, as the wyo server's stale-form check compares them. -->
                <input type="hidden" name={DRAWN_FIELD} value={String($values[DRAWN_FIELD] ?? '')} />
                {@render children()}
                <FormButtons>
                    {#if canWrite && changed}
                        <Button type="submit" dataTest="{id}-save">{saveLabel ?? t('app.common.save', lang)}</Button>
                        {#if saveHelp}
                            <HelpTooltip
                                tooltip={saveHelp}
                                ariaLabel={saveLabel ?? t('app.common.save', lang)}
                                testId="{id}-save-help"
                            />
                        {/if}
                    {/if}
                    <Button variant="secondary" dataTest="{id}-close" onclick={close}>{t('common.close', lang)}</Button>
                    {#snippet below()}
                        <FormAnswer error={refusal} dataTest={id} />
                    {/snippet}
                </FormButtons>
            </GuardedForm>
        </div>
    {/if}
</CollapsibleSection>

<ConfirmationModal
    bind:open={asking}
    title={t('common.disclosure.unsavedTitle', lang)}
    message={t('common.disclosure.unsavedMessage', lang)}
    confirmText={t('common.disclosure.discard', lang)}
    cancelText={t('common.disclosure.back', lang)}
    dataTest="{id}-unsaved"
    onConfirm={discard}
/>
