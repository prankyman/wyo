<script lang="ts">
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import type { Snippet } from 'svelte';
    import Button from './Button.svelte';
    import CollapsibleSection from './CollapsibleSection.svelte';
    import RemovableChip from './RemovableChip.svelte';
    import type { IFilterChip, IHelpLink } from './types';

    /**
     * The «Фильтр» section of a list: a `CollapsibleSection`, closed on arrival, holding the page's filter
     * boxes. Its heading shows how many filters are in use ("Фильтр · 2"), and under it — while the section is closed AND
     * while it is open — sits one removable chip per filter in use and «Сбросить все», so a person sees that the list is
     * filtered without opening the filter. With no filter in use there is no number and no chips. The chips' words come from
     * the page, already translated; the boxes are the `children`.
     *
     * Each chip and «Сбросить все» is a link when the filters live in the address (`href` / `resetHref`) or a button when the
     * screen holds them (`onRemove` / `onResetAll`).
     */
    interface IProps {
        lang: TLocale;
        /** The section's id and `data-test`; its chips are `<id>-chips`, `<id>-chip-<key>`, its reset `<id>-reset`. */
        id?: string;
        /** The filters in use. */
        chips: IFilterChip[];
        /** Where «Сбросить все» goes (the address without the filters); or `onResetAll` for a screen that holds them. */
        resetHref?: string;
        onResetAll?: () => void;
        /** Open on arrival; the section arrives closed by default. */
        open?: boolean;
        help?: string;
        helpLinks?: IHelpLink[];
        /** Classes of the outer box: its place in a grid. Never a margin: the `Stack` it stands in spaces it. */
        class?: string;
        children: Snippet;
    }

    let {
        lang,
        id = 'filter-section',
        chips,
        resetHref,
        onResetAll,
        open = false,
        help,
        helpLinks,
        class: className = '',
        children
    }: IProps = $props();

    const title = $derived(
        chips.length > 0 ? t('app.filter.titleCount', lang, { count: chips.length }) : t('app.filter.title', lang)
    );
</script>

<div class={className} data-test="{id}-root">
    <CollapsibleSection {lang} {id} {title} {open} {help} {helpLinks} class="" contentClass="flex flex-col gap-3">
        {@render children()}
    </CollapsibleSection>

    {#if chips.length > 0}
        <div class="mt-2 flex flex-wrap items-center gap-2" data-test="{id}-chips">
            <ul class="flex flex-wrap gap-1.5" aria-label={t('app.filter.chipsLabel', lang)}>
                {#each chips as chip (chip.key)}
                    <li>
                        <RemovableChip
                            label={chip.label}
                            ariaLabel={t('app.filter.removeChip', lang, { name: chip.label })}
                            href={chip.href}
                            onRemove={chip.onRemove}
                            dataTest="{id}-chip-{chip.key}"
                        />
                    </li>
                {/each}
            </ul>
            <Button
                variant="ghost"
                href={resetHref}
                onclick={onResetAll}
                class="px-2 text-ink-500 pressable:hover:bg-transparent"
                dataTest="{id}-reset"
            >
                {t('app.filter.resetAll', lang)}
            </Button>
        </div>
    {/if}
</div>
