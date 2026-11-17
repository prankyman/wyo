<script lang="ts">
    import { FOCUS_RING, PRESS_FEEDBACK } from '$lib/components/ui/press';
    import { getLocale, t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import { toast } from '$lib/stores/toast.svelte';
    import X from 'phosphor-svelte/lib/X';

    /** The close button is named in the reader's language: the layouts pass theirs. */
    let { lang = getLocale() }: { lang?: TLocale } = $props();

    const toasts = $derived(toast.toasts);
    const timeouts = new Map<string, ReturnType<typeof setTimeout>>();

    function getVariantClass(variant: string = 'default'): string {
        const variants = {
            default: 'bg-ink-900 text-ink-50 border-ink-800',
            destructive: 'bg-stop-600 text-white border-stop-700',
            success: 'bg-ok-600 text-ink-50 border-ok-700',
            warning: 'bg-warn-500 text-ink-900 border-warn-600'
        };
        return variants[variant as keyof typeof variants] || variants.default;
    }

    function scheduleAutoDismiss(toastItem: (typeof toast.toasts)[0]) {
        if (toastItem.timeout && toastItem.timeout > 0) {
            const timeoutId = setTimeout(() => {
                toast.remove(toastItem.id);
                timeouts.delete(toastItem.id);
            }, toastItem.timeout * 1000);
            timeouts.set(toastItem.id, timeoutId);
        }
    }

    function clearTimeout(toastId: string) {
        const timeoutId = timeouts.get(toastId);
        if (timeoutId) {
            window.clearTimeout(timeoutId);
            timeouts.delete(toastId);
        }
    }

    $effect(() => {
        // Schedule auto-dismiss for new toasts
        for (const toastItem of toasts) {
            if (!timeouts.has(toastItem.id)) {
                scheduleAutoDismiss(toastItem);
            }
        }

        // Clear timeouts for removed toasts
        for (const [id] of timeouts) {
            if (!toasts.find((t) => t.id === id)) {
                clearTimeout(id);
            }
        }
    });

    function handleClose(toastId: string) {
        clearTimeout(toastId);
        toast.remove(toastId);
    }
</script>

<!--
    The messages stand under the page's header: the layout gives the header's height as `--header-height`, so a
    message never covers the header's icons.
-->
<div class="fixed top-[calc(var(--header-height,0px)+1rem)] right-4 z-50 flex flex-col gap-2">
    {#each toasts as toastItem (toastItem.id)}
        <div
            class="flex items-start gap-3 px-4 py-3 rounded-lg border shadow-lg min-w-75 max-w-md {getVariantClass(
                toastItem.variant
            )}"
        >
            <div class="flex-1">
                <p class="text-sm font-medium text-inherit!">{toastItem.title}</p>
                {#if toastItem.description}
                    <p class="text-xs mt-1 opacity-90 text-inherit!">{toastItem.description}</p>
                {/if}
            </div>
            <!-- A 44 × 44 tap box around the 16 px cross. -->
            <button
                onclick={() => handleClose(toastItem.id)}
                class="flex size-11 shrink-0 items-center justify-center opacity-70 pressable:hover:opacity-100 pressable:active:opacity-100 text-inherit! {PRESS_FEEDBACK} {FOCUS_RING}"
                aria-label={t('common.close', lang)}
                data-test="toast-close"
            >
                <X size={16} />
            </button>
        </div>
    {/each}
</div>
