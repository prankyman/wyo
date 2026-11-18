<script lang="ts">
    import { FOCUS_RING, PRESS_FEEDBACK } from '$lib/components/ui/press';
    import SlideOverPanel from '$lib/components/ui/SlideOverPanel.svelte';
    import { localizePath, t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import {
        isNavItemActive,
        NAV_ITEMS,
        NAV_SECTION_LABEL_KEYS,
        NAV_SECTIONS,
        navGroups,
        phoneStrip,
        sectionOfPath,
        type INavCounter,
        type INavGroup,
        type INavItem,
        type TNavSection
    } from '$lib/nav/nav-items';
    import DotsThreeIcon from 'phosphor-svelte/lib/DotsThreeIcon';
    import { untrack, type Snippet } from 'svelte';
    import CounterPill from './CounterPill.svelte';
    import { NavGroupsState } from '$lib/nav/nav-groups.svelte';

    interface IProps {
        lang: TLocale;
        /** Path with the language prefix already stripped, e.g. `/settings`. */
        currentPath: string;
        /** The items to draw; the menu of `src/lib/nav/nav-items.ts` by default. */
        items?: readonly INavItem[];
        /** The groups the items are folded into, in the order they are drawn; the menu's own by default. */
        sections?: readonly TNavSection[];
        /** The label key of each group; the menu's own by default. */
        sectionLabelKeys?: Readonly<Record<TNavSection, string>>;
        /**
         * Which of the two layouts to render. They are separate renders rather than one component drawing both, because the left
         * column belongs INSIDE the page's flex row while the strip is fixed to the window — so the shell mounts them in two
         * different places, and a component that drew both would put two of each in the document.
         */
        variant: 'sidebar' | 'tabstrip';
        /** The count beside an item and the sentence that says what it counts, by the item's `testId`; a count of 0 is not drawn. */
        counters?: Readonly<Record<string, INavCounter>>;
        /** The version line at the bottom of the left column and of the «Ещё» sheet; null draws none. */
        version: string | null;
        /**
         * Which menu groups are open: the shell hands the SAME state to the left column and to the «Ещё» sheet. Left out (a test),
         * the menu keeps its own.
         */
        groups?: NavGroupsState;
    }

    let {
        lang,
        currentPath,
        items = NAV_ITEMS,
        sections = NAV_SECTIONS,
        sectionLabelKeys = NAV_SECTION_LABEL_KEYS,
        variant,
        counters = {},
        version,
        groups = untrack(() => new NavGroupsState(currentPath, items, sections))
    }: IProps = $props();

    let moreOpen = $state(false);

    /** What fits across the bottom of a phone (five at most), and everything that goes into the «Ещё» sheet instead. */
    const split = $derived(phoneStrip(items));
    const strip = $derived(split.strip);
    const secondary = $derived(split.secondary);
    /** The left column folded into its groups, and the «Ещё» sheet's — the strip's items left out of it. */
    const sidebarMenu = $derived(navGroups(items, sections));
    const sheetMenu = $derived(navGroups(secondary, sections));

    // A page change opens the new page's group before the menu is drawn again; the first page was opened when the state was made.
    $effect.pre(() => {
        const path = currentPath;
        const shown = items;
        untrack(() => groups.follow(path, shown));
    });

    /** Where the left column appears (Tailwind's `lg`, 1024 px): from there the «Ещё» sheet is not needed. */
    const LEFT_MENU_QUERY = '(min-width: 64rem)';

    // An open «Ещё» sheet slides away by itself when the window grows wide enough for the left column.
    $effect(() => {
        if (!moreOpen || typeof window.matchMedia !== 'function') return;
        const wide = window.matchMedia(LEFT_MENU_QUERY);
        const closeWhenWide = () => {
            if (wide.matches) moreOpen = false;
        };
        closeWhenWide();
        wide.addEventListener('change', closeWhenWide);
        return () => wide.removeEventListener('change', closeWhenWide);
    });

    /** «Ещё» opens with the open page's group open, when the page's item is in the sheet. */
    function openMore(): void {
        const section = sectionOfPath(secondary, currentPath);
        if (section) groups.setOpen(section, true);
        moreOpen = true;
    }

    /** The number on an item's pill, or 0 for an item without one: a pill at 0 is not drawn. */
    const countOf = (item: INavItem): number => counters[item.testId]?.count ?? 0;
    const sentenceOf = (item: INavItem): string => counters[item.testId]?.sentence ?? '';
    /** `nav-group-orders` in the left column, `nav-more-group-orders` in the sheet. */
    const groupId = (group: INavGroup, where: 'sidebar' | 'sheet'): string =>
        `nav-${where === 'sheet' ? 'more-' : ''}group-${group.section}`;
    /** A closed group's counter: the sum of its items' counters, one sentence for each item that has a number. */
    const groupCount = (group: INavGroup): number => group.items.reduce((sum, item) => sum + countOf(item), 0);
    const groupSentences = (group: INavGroup): string[] =>
        group.items.filter((item) => countOf(item) > 0).map(sentenceOf);
    const versionLine = $derived(version === null ? null : t('shell.nav.version', lang, { version }));

    const href = (path: string) => localizePath(path, lang);

    /** «Ещё» shows also for the version line alone, so a phone with every item on the strip still reads the version. */
    const showMoreTab = $derived(secondary.length > 0 || versionLine !== null);

    /**
     * Pressed feedback on every destination: the ground darkens under the finger at once — `touch-manipulation` drops the
     * double-tap wait, and the phone's grey tap flash is replaced by this.
     */
    const press = `touch-manipulation [-webkit-tap-highlight-color:transparent] transition-colors duration-100 ${FOCUS_RING}`;
    /** A group's heading row: a `<summary>` cannot be a `Button`, so it takes the pressed look from `press.ts`. */
    const groupHeadingClass = `flex min-h-11 cursor-pointer list-none items-center gap-2 rounded px-3 py-2 text-sm font-bold text-ink-500 select-none pressable:hover:bg-ink-100 pressable:hover:text-ink-900 pressable:active:bg-ink-200 ${PRESS_FEEDBACK} ${FOCUS_RING} [&::-webkit-details-marker]:hidden`;
</script>

<!--
    One item list, two layouts (src/lib/nav/nav-items.ts). The left column appears at `lg` and up; below it the same destinations
    are a fixed strip across the bottom of the phone, where a thumb reaches them.
-->

<!--
    One menu group: a native <details>, so the browser gives the open and closed state and the keyboard. The heading row is at
    least 44 px tall; a closed group shows the sum of its items' counters.
-->
{#snippet menuGroup(group: INavGroup, where: 'sidebar' | 'sheet', item: Snippet<[INavItem]>)}
    {@const open = groups.isOpen(group.section)}
    {@const count = groupCount(group)}
    <details
        class="group"
        {open}
        ontoggle={(event) => groups.setOpen(group.section, event.currentTarget.open)}
        data-test={groupId(group, where)}
    >
        <summary class={groupHeadingClass} data-test="{groupId(group, where)}-heading">
            <span
                class="inline-flex shrink-0 transition-transform duration-200 ease-in group-open:rotate-90 group-open:duration-250 group-open:ease-out motion-reduce:transition-none"
                aria-hidden="true"
            >
                <svg viewBox="0 0 16 16" class="size-4" fill="currentColor"><path d="M5 3l7 5-7 5z" /></svg>
            </span>
            <span>{t(sectionLabelKeys[group.section], lang)}</span>
            {#if !open && count > 0}
                <CounterPill
                    {count}
                    sentences={groupSentences(group)}
                    testId="{groupId(group, where)}-count"
                    class="-my-2 -mr-3 ml-auto"
                />
            {/if}
            <span class="sr-only">
                {open ? t('shell.nav.pressToClose', lang) : t('shell.nav.pressToOpen', lang)}
            </span>
        </summary>
        <ul class="space-y-1 pb-1 pl-4">
            {#each group.items as entry (entry.href)}
                <li class="relative">{@render item(entry)}</li>
            {/each}
        </ul>
    </details>
{/snippet}

{#snippet sidebarItem(item: INavItem)}
    {@const Icon = item.icon}
    {@const active = isNavItemActive(item, currentPath, items)}
    <!-- The counter stands beside the link, over its right end, not inside it: a button in a link is invalid HTML. The item's <li> is `relative`. -->
    <a
        href={href(item.href)}
        data-test={item.testId}
        aria-current={active ? 'page' : undefined}
        class="flex min-h-11 items-center gap-3 rounded py-2 pl-3 text-sm {countOf(item) > 0 ? 'pr-12' : 'pr-3'} {press}
        {active
            ? 'bg-brand-600 text-white pressable:active:bg-brand-800'
            : 'text-ink-700 pressable:hover:bg-ink-100 pressable:hover:text-ink-900 pressable:active:bg-ink-200'}"
    >
        <Icon size="1.125rem" weight={active ? 'fill' : 'regular'} />
        <span>{t(item.labelKey, lang)}</span>
    </a>
    {#if countOf(item) > 0}
        <CounterPill
            count={countOf(item)}
            sentences={[sentenceOf(item)]}
            testId="{item.testId}-count"
            class="absolute inset-y-0 right-0 my-auto"
        />
    {/if}
{/snippet}

<!-- The open page's item is drawn as in the left column: on the brand colour. -->
{#snippet sheetItem(item: INavItem)}
    {@const Icon = item.icon}
    {@const active = isNavItemActive(item, currentPath, items)}
    <a
        href={href(item.href)}
        data-test="{item.testId}-more"
        aria-current={active ? 'page' : undefined}
        onclick={() => (moreOpen = false)}
        class="flex min-h-11 items-center gap-3 rounded py-3 pl-3 text-sm {countOf(item) > 0 ? 'pr-12' : 'pr-3'} {press}
        {active
            ? 'bg-brand-600 text-white pressable:active:bg-brand-800'
            : 'text-ink-700 pressable:hover:bg-ink-100 pressable:active:bg-ink-200'}"
    >
        <Icon size="1.125rem" weight={active ? 'fill' : 'regular'} />
        <span>{t(item.labelKey, lang)}</span>
    </a>
    {#if countOf(item) > 0}
        <CounterPill
            count={countOf(item)}
            sentences={[sentenceOf(item)]}
            testId="{item.testId}-more-count"
            class="absolute inset-y-0 right-0 my-auto"
        />
    {/if}
{/snippet}

<!--
    Computer: a column down the left. It stays under the pinned top bar (4 rem tall), as high as the window below it, and scrolls on
    its own when its items do not fit.
-->
{#if variant === 'sidebar'}
    <nav
        class="hidden lg:sticky lg:top-16 lg:block lg:max-h-[calc(100dvh-4rem)] lg:overflow-y-auto lg:overscroll-contain lg:py-4"
        aria-label={t('shell.nav.menu', lang)}
        data-test="app-sidebar"
    >
        <ul class="space-y-1">
            {#each sidebarMenu.top as item (item.href)}
                <li class="relative">{@render sidebarItem(item)}</li>
            {/each}
        </ul>
        {#each sidebarMenu.groups as group (group.section)}
            {@render menuGroup(group, 'sidebar', sidebarItem)}
        {/each}
        {#if versionLine}
            <p class="mt-4 px-3 text-xs text-ink-500" data-test="nav-version">{versionLine}</p>
        {/if}
    </nav>
{/if}

<!-- Phone and tablet: a strip along the bottom. `pb-[env(safe-area-inset-bottom)]` keeps it clear of the home indicator. -->
{#if variant === 'tabstrip'}
    <nav
        class="fixed inset-x-0 bottom-0 z-40 border-t border-ink-200 bg-white pb-[env(safe-area-inset-bottom)] lg:hidden"
        aria-label={t('shell.nav.menu', lang)}
        data-test="app-tabstrip"
    >
        <ul class="grid" style:grid-template-columns="repeat({strip.length + (showMoreTab ? 1 : 0)}, minmax(0, 1fr))">
            {#each strip as item (item.href)}
                {@const Icon = item.icon}
                {@const active = isNavItemActive(item, currentPath, items)}
                <li class="relative">
                    <a
                        href={href(item.href)}
                        data-test="{item.testId}-tab"
                        aria-current={active ? 'page' : undefined}
                        class="flex min-h-14 flex-col items-center justify-center gap-0.5 px-1 py-2 text-[0.625rem] pressable:active:bg-ink-200 pressable:active:text-ink-900 {press}
                        {active ? 'text-brand-700' : 'text-ink-500'}"
                    >
                        <span>
                            <Icon size="1.25rem" weight={active ? 'fill' : 'regular'} />
                        </span>
                        <!-- `max-w-full`: in a centred column the label is as wide as its text, so without a limit `truncate` never cuts and neighbouring labels overlap. -->
                        <span class="max-w-full truncate" data-test="nav-tab-label">{t(item.labelKey, lang)}</span>
                    </a>
                    <!-- Over the icon's top right corner, beside the link: 24 × 24 px on the strip (CounterPill, `compact`). -->
                    {#if countOf(item) > 0}
                        <CounterPill
                            compact
                            count={countOf(item)}
                            sentences={[sentenceOf(item)]}
                            testId="{item.testId}-tab-count"
                            class="absolute top-0 left-1/2 ml-0.5"
                        />
                    {/if}
                </li>
            {/each}
            {#if showMoreTab}
                <li>
                    <button
                        type="button"
                        data-test="nav-more-tab"
                        aria-expanded={moreOpen}
                        onclick={openMore}
                        class="flex min-h-14 w-full flex-col items-center justify-center gap-0.5 px-1 py-2 text-[0.625rem] text-ink-500 pressable:active:bg-ink-200 pressable:active:text-ink-900 {press}"
                    >
                        <DotsThreeIcon size="1.25rem" />
                        <span class="max-w-full truncate" data-test="nav-tab-label">{t('shell.nav.more', lang)}</span>
                    </button>
                </li>
            {/if}
        </ul>
    </nav>

    <SlideOverPanel
        open={moreOpen}
        title={t('shell.nav.more', lang)}
        {lang}
        placement="responsive"
        dataTest="nav-more-sheet"
        onClose={() => (moreOpen = false)}
    >
        <div class="p-2">
            {#if sheetMenu.top.length > 0}
                <ul>
                    {#each sheetMenu.top as item (item.href)}
                        <li class="relative">{@render sheetItem(item)}</li>
                    {/each}
                </ul>
            {/if}
            {#each sheetMenu.groups as group (group.section)}
                {@render menuGroup(group, 'sheet', sheetItem)}
            {/each}
        </div>
        {#if versionLine}
            <p class="px-5 pt-2 pb-4 text-xs text-ink-500" data-test="nav-more-version">{versionLine}</p>
        {/if}
    </SlideOverPanel>
{/if}
