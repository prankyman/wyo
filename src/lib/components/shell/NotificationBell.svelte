<script lang="ts">
    import HeadingWithHelp from '$lib/components/ui/HeadingWithHelp.svelte';
    import HelpTooltip from '$lib/components/ui/HelpTooltip.svelte';
    import IconTooltip from '$lib/components/ui/IconTooltip.svelte';
    import SlideOverPanel from '$lib/components/ui/SlideOverPanel.svelte';
    import { dismissable } from '$lib/components/ui/dismissable';
    import { focusableElements } from '$lib/components/ui/overlay-a11y';
    import { FOCUS_RING, PRESS_FEEDBACK } from '$lib/components/ui/press';
    import { createReducedMotion } from '$lib/components/ui/reduced-motion.svelte';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import type { INotificationLine } from '$lib/notifications/types';
    import BellIcon from 'phosphor-svelte/lib/BellIcon';
    import BellRingingIcon from 'phosphor-svelte/lib/BellRingingIcon';
    import { tick, untrack } from 'svelte';
    import NotificationPanel from './NotificationPanel.svelte';
    import { announcement, attentionKeys, bellNumber, bellTone, panelKeys, shouldPulse } from './panel-rules';

    /**
     * The bell in the top bar: a 44 × 44 px button left of the owner's login that opens the panel of every notice. Grey with
     * nothing; yellow with a yellow number for attention or information only; red, ringing, with a red number while any urgent
     * line remains; and pulsing — a ring spreading from it, only where the device does not ask for less motion — while the panel
     * holds an urgent line the person has not seen since the panel was last opened. A polite live region says «Новое
     * уведомление: …» when a line appears (never assertive, WCAG 4.1.3). On a computer the panel opens under the bell (Escape, a
     * click outside or the bell close it; focus moves in and back); below 768 px it fills the screen in the sheet every phone
     * panel uses. The bell stands on the brand-coloured top bar, so it is drawn as a white round chip that keeps its tone colours
     * readable.
     */
    interface IProps {
        lang: TLocale;
        lines: INotificationLine[];
        /** The «?» beside the panel's heading. */
        helpText: string;
    }

    let { lang, lines, helpText }: IProps = $props();

    /** Tailwind's `md`: below it the panel is a full-screen sheet. */
    const MD_BREAKPOINT = 768;

    let open = $state(false);
    let innerWidth = $state(0);
    let bell = $state<HTMLButtonElement | null>(null);
    let popover = $state<HTMLDivElement | null>(null);
    const reducedMotion = createReducedMotion();
    const isPhone = $derived(innerWidth > 0 && innerWidth < MD_BREAKPOINT);

    /** What the person saw when the panel was last opened (the keys of the urgent lines). */
    let seen = $state<Set<string>>(new Set());
    const current = $derived(attentionKeys(lines));
    const pulsing = $derived(shouldPulse(current, seen) && !open);
    const tone = $derived(bellTone(lines));
    const urgentCount = $derived(lines.filter((line) => line.urgency === 'urgent').length);
    const number = $derived(bellNumber(lines.length));
    const label = $derived(
        lines.length === 0
            ? t('shell.notifications.bellLabelEmpty', lang)
            : t('shell.notifications.bellLabel', lang, { count: lines.length, urgent: urgentCount })
    );

    /** The live region's sentence, and the keys it was decided from (null until the first lines arrive). */
    let said = $state('');
    let known: Set<string> | null = null;
    $effect(() => {
        const keys = panelKeys(lines);
        const text = announcement(known, lines);
        untrack(() => {
            if (text !== null) said = t('shell.notifications.announce', lang, { text });
        });
        known = keys;
    });

    async function show(): Promise<void> {
        open = true;
        seen = new Set(current);
        await tick();
        if (!isPhone && popover) (focusableElements(popover)[0] ?? popover).focus();
    }

    function hide(returnFocus = true): void {
        open = false;
        if (returnFocus) bell?.focus();
    }

    // While open, the panel keeps what it shows "seen", so a line that arrives while the person reads does not pulse afterwards.
    $effect(() => {
        if (!open) return;
        const keys = current;
        untrack(() => (seen = new Set([...seen, ...keys])));
    });

    // On a computer: Escape or a click outside closes the popover (`dismissable` returns focus to the bell on Escape).
    $effect(() => {
        if (!open || isPhone || !popover) return;
        return dismissable(popover, { onDismiss: () => hide(false), trigger: () => bell });
    });

    const toneClass = $derived(
        tone === 'urgent' ? 'text-stop-600' : tone === 'attention' ? 'text-warn-600' : 'text-ink-500'
    );
    const numberClass = $derived(tone === 'urgent' ? 'bg-stop-600' : 'bg-warn-600');
</script>

<svelte:window bind:innerWidth />

<div class="relative" data-test="notification-bell-root">
    <IconTooltip
        name={t('shell.notifications.title', lang)}
        description={t('shell.notifications.bellHint', lang)}
        testId="notification-bell-tip"
    >
        <button
            bind:this={bell}
            type="button"
            class="relative inline-flex size-11 items-center justify-center rounded-full bg-white pressable:hover:bg-ink-100 {toneClass} {PRESS_FEEDBACK} {FOCUS_RING}"
            aria-label={label}
            aria-expanded={open}
            aria-haspopup="dialog"
            data-test="notification-bell"
            data-tone={tone}
            data-unseen={pulsing ? 'true' : 'false'}
            data-pulsing={pulsing && !reducedMotion.current ? 'true' : 'false'}
            onclick={() => (open ? hide() : void show())}
        >
            {#if pulsing && !reducedMotion.current}
                <span
                    class="absolute inset-1 rounded-full {tone === 'urgent'
                        ? 'bg-stop-400'
                        : 'bg-warn-300'} opacity-60 motion-safe:animate-ping"
                    aria-hidden="true"
                    data-test="notification-bell-pulse"
                ></span>
            {/if}
            {#if tone === 'urgent'}
                <BellRingingIcon class="relative size-6" weight="fill" aria-hidden="true" />
            {:else}
                <BellIcon class="relative size-6" weight={tone === 'quiet' ? 'regular' : 'fill'} aria-hidden="true" />
            {/if}
            {#if number !== null}
                <span
                    class="absolute -top-0.5 -right-0.5 inline-flex h-5 min-w-5 items-center justify-center rounded-full px-1 text-[11px] leading-none font-semibold text-white {numberClass}"
                    aria-hidden="true"
                    data-test="notification-bell-number">{number}</span
                >
            {/if}
        </button>
    </IconTooltip>
    <span role="status" class="sr-only" data-test="notification-bell-live">{said}</span>

    {#if open && !isPhone}
        <div
            bind:this={popover}
            role="dialog"
            aria-label={t('shell.notifications.title', lang)}
            tabindex="-1"
            class="absolute top-full right-0 z-50 mt-2 flex max-h-[70vh] w-[26rem] max-w-[calc(100vw-2rem)] flex-col overflow-hidden rounded-lg border border-ink-200 bg-white text-ink-800 shadow-xl"
            data-test="notification-panel"
            data-form="popover"
        >
            <HeadingWithHelp
                text={t('shell.notifications.title', lang)}
                help={helpText}
                look="title"
                testId="notification-panel-help"
                headingTestId="notification-panel-heading"
                class="shrink-0 border-b border-ink-200 px-4 py-3"
            />
            <div class="min-h-0 flex-1 overflow-y-auto overscroll-contain">
                <NotificationPanel {lang} {lines} onNavigate={() => hide(false)} />
            </div>
        </div>
    {/if}
</div>

{#if isPhone}
    <SlideOverPanel
        {open}
        title={t('shell.notifications.title', lang)}
        {lang}
        onClose={() => hide()}
        dataTest="notification-panel"
        widthClass="w-full"
    >
        {#snippet heading()}
            <HelpTooltip
                tooltip={helpText}
                ariaLabel={t('shell.notifications.title', lang)}
                testId="notification-panel-help"
            />
        {/snippet}
        <NotificationPanel {lang} {lines} onNavigate={() => hide(false)} />
    </SlideOverPanel>
{/if}
