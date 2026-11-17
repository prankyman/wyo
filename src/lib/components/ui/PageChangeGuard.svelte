<script lang="ts">
    import { getLocale, t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import type { Snippet } from 'svelte';
    import { onMount } from 'svelte';
    import { LOCK_MAX_MS, pageChange } from './page-change.svelte';

    /**
     * While the page is being replaced because the person pressed something — a link, a GET
     * form, a plain form — a 6 px bar in the brand colour runs along the lower edge of the pinned header, or the window's top on a page
     * without one (before the interface has started, the start-up bar in
     * `src/app.html` draws the same) and the page takes no taps (constitution, press feedback and single submission). A `goto` from code (a
     * search box refreshing its list as the person types) locks nothing: see `page-change.svelte.ts`.
     * Mounted once, in the root layout, so it covers every screen.
     *
     * - The bar appears only after `BAR_DELAY_MS`: most SvelteKit navigations finish sooner, and a
     *   bar flashing on every tap is noise.
     * - The page is `inert` from the first moment: a second tap on a card or link a few hundred
     *   milliseconds later must not start a second navigation. `inert` also takes the page out of
     *   the keyboard's reach and hides nothing (unlike `aria-hidden`); `aria-busy` tells assistive
     *   technology the content is being replaced.
     * - The page is never left locked: SvelteKit clears `navigating` on every ending (the new page,
     *   an error page, a newer navigation), and a page coming back from the back/forward cache
     *   clears a plain form's submission here. A change still running after `LOCK_MAX_MS` — a
     *   server that does not answer, or a navigation SvelteKit dropped without clearing `navigating`
     *   — gives the page back to the person, so a phone with no reload button in sight cannot be trapped;
     *   the bar keeps running to say the answer is still awaited. The same moment is written to
     *   `pageChange.lockExpired`, so a link stops showing "opening" and swallowing taps too.
     *
     * Overlays rendered through `Portal` live outside this wrapper and are not made inert: their
     * own buttons are `Button`s, which guard themselves.
     */
    interface IProps {
        lang?: TLocale;
        children: Snippet;
    }

    let { lang, children }: IProps = $props();

    /** Longer than a navigation served from the preload cache, shorter than one that makes a person wonder. */
    const BAR_DELAY_MS = 150;

    let barVisible = $state(false);

    const changing = $derived(pageChange.locking);
    const locked = $derived(changing && !pageChange.lockExpired);
    /**
     * Which change is running: a newer navigation starting while one runs — even to the same address,
     * as a press on a link that was left dead is — locks the page afresh, so it is told apart by the
     * navigation's own destination object, not by its text.
     */
    const change = $derived(pageChange.nativeSubmission ? 'form' : changing ? pageChange.destination : null);

    $effect(() => {
        if (change === null) {
            barVisible = false;
            pageChange.lockExpired = false;
            return;
        }
        pageChange.lockExpired = false;
        const bar = setTimeout(() => (barVisible = true), BAR_DELAY_MS);
        const lock = setTimeout(() => (pageChange.lockExpired = true), LOCK_MAX_MS);
        return () => {
            clearTimeout(bar);
            clearTimeout(lock);
        };
    });

    onMount(() => {
        // The interface's guards are live from here on (every form's guard attached as the page was
        // mounted, before this): the start-up guard in src/app.html steps aside. A form sent before this moment was
        // stopped there, not posted (static files have nothing to post to), so no page change is under way.
        window.__startupSubmitGuard?.release();
        // The start-up bar stops watching presses and goes away unless a press is loading a new page: from here this
        // component's bar shows every page change.
        window.__startupPageChangeBar?.release();

        // Back from the back/forward cache: the plain form's answer was never shown here.
        const onPageShow = (event: PageTransitionEvent) => {
            if (event.persisted) pageChange.nativeSubmission = false;
        };
        // Safari on iOS applies `:active` only when a touch listener is registered, so without
        // this no pressed state would ever show on an iPhone. Passive: it never delays scrolling.
        // https://developer.apple.com/library/archive/documentation/AppleApplications/Reference/SafariWebContent/AdjustingtheTextSize/AdjustingtheTextSize.html
        const noop = () => {};
        window.addEventListener('pageshow', onPageShow);
        document.addEventListener('touchstart', noop, { passive: true });
        return () => {
            window.removeEventListener('pageshow', onPageShow);
            document.removeEventListener('touchstart', noop);
        };
    });
</script>

{#if barVisible}
    <div
        role="progressbar"
        aria-label={t('common.pageLoading', lang ?? getLocale())}
        class="page-change-bar pointer-events-none fixed inset-x-0 top-[var(--header-height,0px)] z-[100] h-1.5 overflow-hidden bg-brand-100"
        data-test="page-change-bar"
    >
        <div class="page-change-bar-runner h-full w-1/3 bg-brand-500"></div>
    </div>
{/if}

<div class="contents" inert={locked} aria-busy={changing || undefined} data-test="page-content">
    {@render children()}
</div>

<style>
    .page-change-bar-runner {
        animation: page-change-run 1.1s ease-in-out infinite;
    }

    @keyframes page-change-run {
        from {
            transform: translateX(-100%);
        }
        to {
            transform: translateX(300%);
        }
    }

    /* Less motion asked for: a still bar says the same without sweeping across the screen. */
    @media (prefers-reduced-motion: reduce) {
        .page-change-bar-runner {
            width: 100%;
            animation: none;
            opacity: 0.6;
        }
    }
</style>
