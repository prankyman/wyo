<script lang="ts" generics="TRecord">
    import { preloadData, replaceState } from '$app/navigation';
    import { page } from '$app/state';
    import type { TLocale } from '$lib/i18n/types';
    import type { Snippet } from 'svelte';
    import { untrack } from 'svelte';
    import SlideOverPanel from './SlideOverPanel.svelte';
    import { closeShallowDetails, openShallowLink } from './shallow-details';

    /**
     * A picked row's details or edit form in a side panel — from the right on a computer, from the bottom on a
     * phone. The row keeps its ordinary link (`?<param>=<id>`); a plain press opens the panel without reloading the page, so the
     * list keeps its place and scroll (`openShallowLink`), and closing returns to the same place (`closeShallowDetails`). A direct
     * address (a reload, a copied link) opens the panel from the record the page's load returned for it.
     */
    interface IProps {
        lang: TLocale;
        /** The address parameter that names the opened row: `edit`, `item`. */
        param: string;
        /** The record the page's load returned for a direct address (`?<param>=`), or null. */
        fromLoad: TRecord | null;
        /** The record's id, as the address carries it. */
        idOf: (record: TRecord) => string;
        /** The record out of what the load returned for the pressed address, or nothing when it does not exist. */
        pick: (data: Record<string, unknown>) => TRecord | null | undefined;
        /**
         * The pressed record as the page's data has it now (a save inside the panel re-reads the page's data, not the pressed
         * copy). Without it the pressed copy is shown until the panel closes.
         */
        refresh?: (id: string) => TRecord | null | undefined;
        /** The list's own address, without the parameter: where a panel opened by a direct address returns on close. */
        listHref: string;
        title: (record: TRecord) => string;
        /** `data-test` of the panel: `<dataTest>-panel`, `<dataTest>-close`. */
        dataTest: string;
        widthClass?: string;
        /** Drawn right after the title, e.g. the panel's one «?». */
        heading?: Snippet;
        children: Snippet<[TRecord]>;
    }

    let {
        lang,
        param,
        fromLoad,
        idOf,
        pick,
        refresh,
        listHref,
        title,
        dataTest,
        widthClass,
        heading,
        children
    }: IProps = $props();

    const pressed = $derived(page.state.details?.param === param ? page.state.details : undefined);

    /** The load's copy wins when it is the same row (a save re-read it); otherwise the pressed row, as fresh as the data has it. */
    const opened = $derived.by((): TRecord | null => {
        if (!pressed || (fromLoad !== null && idOf(fromLoad) === pressed.id)) return fromLoad;
        return refresh ? (refresh(pressed.id) ?? null) : (pressed.value as TRecord);
    });

    type TEntry = { param: string; id: string; value: unknown };

    /**
     * A save inside the panel re-reads the page's data for the list's address, not for the pressed row's (a shallow entry does
     * not change the page's address), and SvelteKit clears `page.state` on every such re-read (`invalidateAll`, which a page's code
     * may call after a successful answer; `_invalidate` in `@sveltejs/kit/src/runtime/client/client.js` resets the page state, only
     * `refreshAll` keeps it). While the address still names the pressed row, its entry is put back and the row's own address is
     * read again, so the panel stays open and shows the server's fresh copy (limits, counts, the on/off state).
     */
    let last: TEntry | undefined;
    let seenData: unknown = untrack(() => page.data);
    $effect(() => {
        const entry = page.state.details;
        const data = page.data;
        const dataChanged = data !== seenData;
        seenData = data;
        if (entry?.param === param) {
            last = entry;
            if (dataChanged) void reread(entry);
            return;
        }
        const kept = last;
        if (entry === undefined && kept && untrack(() => new URL(location.href).searchParams.get(param)) === kept.id) {
            replaceState(location.pathname + location.search, { details: kept });
            void reread(kept);
            return;
        }
        last = undefined;
    });

    async function reread(entry: TEntry): Promise<void> {
        const href = location.pathname + location.search;
        const result = await preloadData(href);
        if (result.type !== 'loaded' || result.status !== 200) return;
        const value = pick(result.data);
        if (value == null || page.state.details?.id !== entry.id) return;
        replaceState(href, { details: { ...entry, value } });
    }

    const open = (event: MouseEvent) => openShallowLink(event, { param, pick });
    const close = () => closeShallowDetails(param, listHref);
</script>

<svelte:window onclickcapture={open} />

{#if opened !== null}
    <SlideOverPanel
        open
        placement="responsive"
        title={title(opened)}
        {lang}
        onClose={close}
        {dataTest}
        {widthClass}
        {heading}
    >
        {@render children(opened)}
    </SlideOverPanel>
{/if}
