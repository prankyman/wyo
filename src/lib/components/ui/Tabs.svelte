<script lang="ts">
    import { goto } from '$app/navigation';
    import { page } from '$app/stores';
    import { cn } from '$lib/utils/cn';
    import HelpTooltip from './HelpTooltip.svelte';

    interface Tab {
        id: string;
        label: string;
        /** The text of the tab's «?», drawn beside the tab. Without it the tab stands alone. */
        help?: string;
        /** A number shown after the label: «Label · 4». */
        count?: number;
    }

    interface IProps {
        tabs: Tab[];
        activeTab: string;
        /**
         * Names the tabs: each one carries `<dataTest>-tab-<id>` and its «?» `<dataTest>-tab-<id>-help`. Without it a test
         * can only find a tab by its translated label, which changes with the language the page was asked for — the
         * project's rule is that anything a test touches carries an id.
         */
        dataTest?: string;
        /** The `data-test` of the strip itself. */
        stripTest?: string;
        /** The `data-tour` anchor a learning step points at, on the strip. */
        dataTour?: string;
        /** The strip's name for a screen reader; with it the strip is a navigation landmark. */
        ariaLabel?: string;
        /** The query parameter the open tab is kept in (`?tab=` by default). */
        param?: string;
        /** The address of a tab, for tabs that are links; the page's address with `param` set to the tab by default. */
        hrefOf?: (tabId: string) => string;
        /**
         * `link`: every tab is an `<a href>` (a tab can be bookmarked, and the back button returns to it); `button`: every tab is
         * a `<button>` that moves the page with `goto`, or calls `onselect`. Links when `hrefOf` is given, buttons otherwise.
         */
        variant?: 'link' | 'button';
        /** For `button` tabs: what a press does instead of moving the page, e.g. a list that resets its own page number. */
        onselect?: (tabId: string) => void;
        /** Classes of the strip's place on the page. */
        class?: string;
    }

    let {
        tabs,
        activeTab,
        dataTest,
        stripTest,
        dataTour,
        ariaLabel,
        param = 'tab',
        hrefOf,
        variant,
        onselect,
        class: className
    }: IProps = $props();

    const asLinks = $derived((variant ?? (hrefOf ? 'link' : 'button')) === 'link');

    function addressOf(tabId: string): string {
        if (hrefOf) return hrefOf(tabId);
        const url = new URL($page.url);
        url.searchParams.set(param, tabId);
        return `${url.pathname}${url.search}`;
    }

    const textOf = (tab: Tab): string => (tab.count === undefined ? tab.label : `${tab.label} · ${tab.count}`);

    /**
     * The tab whose page change is running. A tab moves by `goto`, which the page-change guard
     * does not lock, so the strip guards itself (constitution, press feedback and single
     * submission): the pressed tab shows that it is working and no tab takes a press until the
     * change has ended.
     */
    let opening = $state<string | null>(null);

    async function setActiveTab(tabId: string) {
        if (opening !== null) return;
        if (onselect) {
            onselect(tabId);
            return;
        }
        const url = new URL($page.url);
        url.searchParams.set(param, tabId);
        opening = tabId;
        try {
            await goto(url.toString(), { replaceState: true, noScroll: true });
        } finally {
            opening = null;
        }
    }

    const TAB_CLASS = 'tab-button min-h-11 min-w-11 grow sm:shrink-0 sm:grow-0 sm:whitespace-nowrap';
    const openClass = (tabId: string) => activeTab === tabId && 'max-sm:bg-ink-100!';
</script>

<!--
    Below `sm:` (640 px) the tabs wrap into rows so every one is on the screen — five tabs of a connection can be about 540 px of words against 358 px of phone — each at least 44 × 44 px and stretched to
    fill its row, the open one marked by a filled background as well as its underline.
    The fill is `!important` because this file's own `.tab-button` rules are not in a Tailwind layer and would
    otherwise beat it: a tab a finger has just left keeps `:hover` on a touch screen and would wash the fill out.
    From `sm:` up it is the single strip that scrolls sideways inside its own panel, as it always was. Every tab
    without a «?» stays a direct child of `.tabs-list`: the connection page centres the open tab by scrolling that element.
-->
<svelte:element
    this={ariaLabel ? 'nav' : 'div'}
    class={cn('tabs', className)}
    aria-label={ariaLabel}
    data-test={stripTest}
    data-tour={dataTour}
>
    <div class="tabs-list flex-wrap sm:flex-nowrap sm:overflow-x-auto">
        {#each tabs as tab (tab.id)}
            {#snippet item()}
                {#if asLinks}
                    <a
                        href={addressOf(tab.id)}
                        class={[TAB_CLASS, openClass(tab.id)]}
                        class:active={activeTab === tab.id}
                        aria-current={activeTab === tab.id ? 'page' : undefined}
                        data-test={dataTest ? `${dataTest}-tab-${tab.id}` : undefined}
                    >
                        {textOf(tab)}
                    </a>
                {:else}
                    <button
                        class={[TAB_CLASS, openClass(tab.id)]}
                        class:active={activeTab === tab.id}
                        aria-current={activeTab === tab.id ? 'page' : undefined}
                        aria-busy={opening === tab.id || undefined}
                        aria-disabled={opening !== null || undefined}
                        onclick={() => setActiveTab(tab.id)}
                        data-test={dataTest ? `${dataTest}-tab-${tab.id}` : undefined}
                        type="button"
                    >
                        {textOf(tab)}
                    </button>
                {/if}
            {/snippet}
            {#if tab.help}
                <span class="inline-flex items-center gap-1">
                    {@render item()}
                    <HelpTooltip
                        tooltip={tab.help}
                        ariaLabel={tab.label}
                        testId={dataTest ? `${dataTest}-tab-${tab.id}-help` : undefined}
                    />
                </span>
            {:else}
                {@render item()}
            {/if}
        {/each}
    </div>
</svelte:element>

<style>
    .tabs {
        border-bottom: 1px solid var(--color-ink-200);
    }

    .tabs-list {
        display: flex;
        gap: 0;
    }

    .tab-button {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        text-decoration: none;
        padding: 0.75rem 1.5rem;
        background: none;
        border: none;
        border-bottom: 2px solid transparent;
        cursor: pointer;
        font-size: 0.875rem;
        font-weight: 500;
        color: var(--color-ink-500);
        transition: all 0.2s;
    }

    /*
     * Hover and pressed only on a tab that can act: while another tab's page is opening every tab is
     * `aria-disabled`, and a tab that is not going to do anything must not look pressed (constitution,
     * press feedback; the same guard as the `pressable:` variant in src/app.css). Inside `:where()`,
     * so the guard adds no weight and the current tab's own colour still wins as before.
     */
    .tab-button:where(:not(:disabled, [aria-disabled='true'])):hover {
        color: var(--color-ink-700);
        background-color: var(--color-ink-50);
    }

    /*
     * Pressed feedback (constitution, press feedback and single submission): a darker ground under
     * the finger, after `:hover` so it wins on a mouse too. The tab strip keeps its own look, so it
     * is an allowlisted exception in press-feedback-policy.test.ts rather than a row of Buttons.
     * `touch-action` drops the double-tap-to-zoom wait; the tap highlight is replaced by this state.
     */
    .tab-button {
        touch-action: manipulation;
        -webkit-tap-highlight-color: transparent;
    }

    .tab-button:where(:not(:disabled, [aria-disabled='true'])):active {
        color: var(--color-ink-900);
        background-color: var(--color-ink-200);
    }

    .tab-button:focus-visible {
        outline: 2px solid var(--color-brand-600);
        outline-offset: -2px;
    }

    .tab-button.active {
        color: var(--color-brand-700);
        border-bottom-color: var(--color-brand-600);
    }

    /* Working: the pressed tab is already underlined, lighter, until its page arrives; the others wait. */
    .tab-button[aria-busy='true'] {
        color: var(--color-brand-700);
        border-bottom-color: var(--color-brand-300);
        cursor: wait;
    }

    .tab-button[aria-disabled='true']:not([aria-busy='true']) {
        cursor: not-allowed;
        opacity: 0.6;
    }
</style>
