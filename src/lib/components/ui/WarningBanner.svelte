<script lang="ts">
    import Button from './Button.svelte';
    import NoteBox from './NoteBox.svelte';

    /**
     * The advisory banner: something is not wrong yet, but it will block a later step if left alone — a `NoteBox`
     * with a title, a sentence and one action inside.
     *
     * `action` is either a link (`actionHref`) or a button (`onAction`), never both — a banner that both navigates and
     * mutates is two controls wearing one label.
     *
     * `variant: 'danger'` is the red form, for a risk being run NOW rather than one that will block a later
     * step.
     */
    interface IProps {
        title: string;
        message: string;
        actionLabel?: string;
        actionHref?: string;
        onAction?: () => void;
        dataTest?: string;
        variant?: 'warning' | 'danger';
    }

    let { title, message, actionLabel, actionHref, onAction, dataTest, variant = 'warning' }: IProps = $props();

    const showAction = $derived(!!actionLabel && (!!actionHref || !!onAction));
    const tone = $derived(variant === 'danger' ? 'error' : 'warning');
    // Whole class strings per variant rather than a colour spliced into one: Tailwind only generates classes it can read.
    const actionClass = $derived(
        variant === 'danger'
            ? 'text-stop-800 pressable:hover:text-stop-900 pressable:active:bg-stop-100'
            : 'text-warn-800 pressable:hover:text-warn-900 pressable:active:bg-warn-100'
    );
</script>

<!-- The banner stands on the page from the start, so it has no live role: nothing has just happened. -->
<NoteBox {tone} role="none" {dataTest}>
    <div class="flex">
        <div class="shrink-0">
            <svg class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                <path
                    fill-rule="evenodd"
                    d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z"
                    clip-rule="evenodd"
                />
            </svg>
        </div>
        <div class="ml-3">
            <h3 class="text-sm font-medium">{title}</h3>
            <p class="mt-2 text-sm">{message}</p>
            {#if showAction}
                <div class="mt-3">
                    <!-- A `Button` either way: pressed feedback, and the link shows it is opening. -->
                    <Button
                        variant="link"
                        href={actionHref || undefined}
                        onclick={actionHref ? undefined : onAction}
                        class="-ml-1 text-sm font-medium {actionClass}"
                        dataTest={dataTest ? `${dataTest}-action` : undefined}
                    >
                        {actionLabel}
                    </Button>
                </div>
            {/if}
        </div>
    </div>
</NoteBox>
