<script lang="ts">
    import type { Snippet } from 'svelte';
    import { tick, untrack } from 'svelte';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import Button from './Button.svelte';
    import ConfirmationModal from './ConfirmationModal.svelte';
    import DisclosureClose from './DisclosureClose.svelte';
    import { changedSince, fieldValues, firstField, setDisclosureForm, type TFieldValues } from './disclosure-form';
    import type { TButtonSizeEnum } from './types';

    /**
     * A form opened by a press. Closed, it is its
     * opening button, in the secondary look, the same on every screen. Open, the opening button is gone and «Закрыть»
     * stands beside the form's main button (`DisclosureClose`, which the form puts there; without one it is drawn under
     * the form). «Закрыть» closes at once when nothing was changed and asks first when something was; an edit form closes
     * after a successful save (`after="close"`) and says so beside the opening button, an add form stays open and empty for
     * the next row (`after="stay"`), and a refused save keeps the form open with everything typed.
     */
    interface IProps {
        /** The opening button's words, already translated: «Изменить», «Добавить принтер». */
        label: string;
        lang: TLocale;
        /** Open now; bindable, so a screen can open the form from code (an «Добавить» that fills a found address). */
        open?: boolean;
        /** `close` — an edit form or a one-off decision closes after a successful answer; `stay` — an add form stays. */
        after?: 'close' | 'stay';
        /**
         * The form is opened from somewhere else (a row's link that puts `?edit=` into the address): no opening button is
         * drawn, and `onClose` takes the person back.
         */
        showOpener?: boolean;
        /** The opening button's accessible name when its words alone do not say which form it opens. */
        openerAriaLabel?: string;
        /**
         * The opening button's size, so a row of buttons of one size can hold it; `md`, the
         * `Button`'s own default, when left out.
         */
        size?: TButtonSizeEnum;
        /** The opening button takes the whole width of its place, «Сохранено.» beside it. */
        fullWidth?: boolean;
        /**
         * «Сохранено.» stands beside the opening button. The form sets it after a successful answer
         * of its own form; a screen whose form is sent from code sets it when it has closed the form
         * itself. It goes out when the form opens again.
         */
        saved?: boolean;
        openerTestId?: string;
        openerTour?: string;
        /** `data-test` of the open form's box; `<testId>-close` is its «Закрыть». */
        testId?: string;
        tour?: string;
        /** Called after the form closed — by «Закрыть» or after a successful answer. */
        onClose?: () => void;
        /** Called when the form opens. */
        onOpen?: () => void;
        class?: string;
        children: Snippet;
    }

    let {
        label,
        lang,
        open = $bindable(false),
        after = 'close',
        showOpener = true,
        openerAriaLabel,
        size = 'md',
        fullWidth = false,
        saved = $bindable(false),
        openerTestId,
        openerTour,
        testId,
        tour,
        onClose,
        onOpen,
        class: className = '',
        children
    }: IProps = $props();

    const bodyId = `disclosure-form-${Math.random().toString(36).slice(2, 9)}`;
    let body = $state<HTMLDivElement | null>(null);
    let opener = $state<HTMLElement | undefined>();
    let asking = $state(false);
    /** The form put its own «Закрыть» beside its main button: read from the page, so a form that changes steps is followed. */
    let ownClose = $state(true);
    /** The values the form opened with; null until they are read — nothing can have been changed before then. */
    let before: TFieldValues | null = null;

    /** The values the form opened with, read once the form and anything it fills itself are on the page. */
    async function remember(): Promise<void> {
        await tick();
        await tick();
        if (body) before = fieldValues(body);
        else before = null;
    }

    async function opened(byPerson: boolean): Promise<void> {
        saved = false;
        before = null;
        onOpen?.();
        await remember();
        if (byPerson && body) firstField(body)?.focus();
    }

    async function closed(): Promise<void> {
        onClose?.();
        await tick();
        opener?.focus();
    }

    let wasOpen = untrack(() => open);
    if (wasOpen) void remember();
    $effect(() => {
        const now = open;
        untrack(() => {
            if (now && !wasOpen) void opened(true);
            if (!now && wasOpen) void closed();
            wasOpen = now;
        });
    });

    // The form's «Закрыть» is looked for whenever the open form changes; the observer answers before the page is painted.
    $effect(() => {
        if (!body) return;
        const root = body;
        const look = () => (ownClose = root.querySelector('[data-disclosure-close="form"]') !== null);
        look();
        const observer = new MutationObserver(look);
        observer.observe(root, { childList: true, subtree: true });
        return () => observer.disconnect();
    });

    function shut(): void {
        asking = false;
        open = false;
    }

    setDisclosureForm({
        close() {
            if (body && before && changedSince(before, body)) asking = true;
            else shut();
        },
        answered(ok) {
            if (!ok) return;
            if (after === 'close') {
                shut();
                saved = true;
            } else void remember();
        },
        get lang() {
            return lang;
        },
        get testId() {
            return testId;
        }
    });
</script>

<!-- Closed without an opener of its own, the form draws nothing — an empty block would still take a gap in its column. -->
{#if open || showOpener}
    <div class={className} data-disclosure-form={open ? 'open' : 'closed'}>
        {#if open}
            <div id={bodyId} bind:this={body} data-test={testId} data-tour={tour}>
                {@render children()}
                {#if !ownClose}
                    <div class="mt-3"><DisclosureClose fallback /></div>
                {/if}
            </div>
        {:else if showOpener}
            <div class={fullWidth ? 'flex items-center gap-3' : 'flex flex-wrap items-center gap-3'}>
                <Button
                    variant="secondary"
                    {size}
                    {fullWidth}
                    bind:element={opener}
                    ariaLabel={openerAriaLabel}
                    aria-expanded="false"
                    aria-controls={bodyId}
                    dataTest={openerTestId}
                    data-tour={openerTour}
                    onclick={() => (open = true)}
                >
                    {label}
                </Button>
                {#if saved}
                    <span role="status" class="text-sm text-ok-800" data-test={testId ? `${testId}-saved` : undefined}>
                        {t('common.disclosure.saved', lang)}
                    </span>
                {/if}
            </div>
        {/if}
    </div>
{/if}

<ConfirmationModal
    bind:open={asking}
    title={t('common.disclosure.unsavedTitle', lang)}
    message={t('common.disclosure.unsavedMessage', lang)}
    confirmText={t('common.disclosure.discard', lang)}
    cancelText={t('common.disclosure.back', lang)}
    dataTest={testId ? `${testId}-unsaved` : 'disclosure-unsaved'}
    onConfirm={shut}
/>
