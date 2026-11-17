<script lang="ts">
    import { FOCUS_RING, PRESS_FEEDBACK } from '$lib/components/ui/press';
    import { dismissable } from './dismissable';
    import Portal from './Portal.svelte';
    import PopupCloseButton from './PopupCloseButton.svelte';

    /**
     * The amber warning triangle used to flag a row that needs attention.
     *
     * Interaction follows `HelpTooltip`: it opens and closes by a **click or a tap**, never by
     * hovering, and closes on Escape, on a click outside it and on the white cross in its corner.
     * It used to be a CSS `group-hover` overlay, which simply does not open on a phone — the
     * warning was visible and its explanation was unreachable. It is a real `<button>` for
     * the same reason: hover is not an input method every device has.
     *
     * The popup is portaled to `document.body` and rendered `fixed` at `z-10000`, per
     * the constitution's rule for overlays. This icon sits inside table rows and cards that establish their own
     * stacking contexts, which a z-index alone cannot climb out of. `HelpTooltip` shares this ceiling — keep both in
     * sync if it ever changes.
     *
     * Pass `issues` for several problems at once, or `tooltip` for a single line. The popup
     * wraps rather than `whitespace-nowrap`: more than one issue never fit on one line.
     */
    interface IProps {
        size?: string;
        tooltip?: string;
        issues?: string[];
        ariaLabel?: string;
        testId?: string;
    }

    let { size = 'w-4 h-4', tooltip, issues, ariaLabel, testId = 'attention-icon' }: IProps = $props();

    const lines = $derived(issues && issues.length > 0 ? issues : tooltip ? [tooltip] : []);

    let triggerEl = $state<HTMLButtonElement | null>(null);
    let tooltipEl = $state<HTMLDivElement | null>(null);
    let visible = $state(false);
    let tooltipStyle = $state('');

    function updatePosition() {
        if (!triggerEl) return;
        const rect = triggerEl.getBoundingClientRect();
        tooltipStyle =
            `bottom: ${window.innerHeight - rect.top + 8}px;` +
            ` left: ${rect.left + rect.width / 2}px; transform: translateX(-50%);`;
    }

    function hide() {
        visible = false;
    }

    function closeFromButton() {
        hide();
        triggerEl?.focus();
    }

    function toggle() {
        if (lines.length === 0) return;
        visible = !visible;
        if (visible) updatePosition();
    }

    $effect(() => {
        if (!visible || !tooltipEl) return;

        return dismissable(tooltipEl, { onDismiss: hide, trigger: () => triggerEl });
    });

    $effect(() => {
        if (!visible) return;

        window.addEventListener('scroll', updatePosition, true);
        window.addEventListener('resize', updatePosition);

        return () => {
            window.removeEventListener('scroll', updatePosition, true);
            window.removeEventListener('resize', updatePosition);
        };
    });
</script>

<div class="relative inline-block">
    <button
        bind:this={triggerEl}
        type="button"
        class="flex min-h-11 min-w-11 items-center justify-center p-0 border-0 bg-transparent pressable:active:opacity-60 {PRESS_FEEDBACK} {FOCUS_RING} {lines.length >
        0
            ? 'cursor-pointer'
            : 'cursor-default'}"
        aria-label={ariaLabel ?? lines.join('. ')}
        data-test="{testId}-button"
        onclick={toggle}
    >
        <svg
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
            class="text-warn-500 {size}"
            aria-hidden="true"
        >
            <path
                vector-effect="non-scaling-stroke"
                d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"
            />
            <line vector-effect="non-scaling-stroke" x1="12" x2="12" y1="9" y2="13" />
            <line vector-effect="non-scaling-stroke" x1="12" x2="12.01" y1="17" y2="17" />
        </svg>
    </button>

    {#if visible && lines.length > 0}
        <Portal>
            <div
                bind:this={tooltipEl}
                role="tooltip"
                data-test="{testId}-tooltip"
                class="fixed z-10000 w-56 bg-ink-900 text-white text-[14px] px-2 py-1 rounded shadow-lg
                    normal-case tracking-normal"
                style={tooltipStyle}
            >
                <PopupCloseButton onclick={closeFromButton} testId="{testId}-close" />
                {#if lines.length === 1}
                    {lines[0]}
                {:else}
                    <ul class="list-disc list-inside flex flex-col gap-0.5">
                        {#each lines as line, index (index)}
                            <li>{line}</li>
                        {/each}
                    </ul>
                {/if}
            </div>
        </Portal>
    {/if}
</div>
