<script lang="ts">
    import { getLocale, t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import Button from './Button.svelte';

    interface IProps {
        page: number;
        perPage: number;
        total: number;
        lang?: TLocale;
        /** Moves to `page`; a returned promise (SvelteKit's `goto`) keeps the strip working until it settles. */
        onNavigate: (page: number) => void | Promise<void>;
        dataTest?: string;
    }

    let { page, perPage, total, lang = getLocale(), onNavigate, dataTest = 'pagination' }: IProps = $props();

    const totalPages = $derived(Math.max(1, Math.ceil(total / perPage)));
    const from = $derived(total === 0 ? 0 : (page - 1) * perPage + 1);
    const to = $derived(Math.min(page * perPage, total));

    type TPageItem = number | 'ellipsis';

    const pageItems = $derived.by((): TPageItem[] => {
        const items: TPageItem[] = [];
        const window = 1;
        for (let p = 1; p <= totalPages; p++) {
            if (p === 1 || p === totalPages || (p >= page - window && p <= page + window)) {
                items.push(p);
            } else if (items[items.length - 1] !== 'ellipsis') {
                items.push('ellipsis');
            }
        }
        return items;
    });

    /** Square 44 px targets; the arrows in a larger glyph, the numbers in the body font. */
    const arrowClass = 'min-w-11 px-2 text-base';
    const pageClass = 'min-w-11 px-2';

    /**
     * The button whose page change is running. The strip moves by `goto`, which the page-change
     * guard does not lock, so it guards itself: the pressed button works, and every button refuses
     * a press until the change has ended — a second tap on "next" would otherwise start a second one.
     */
    let pressed = $state<string | null>(null);
    const busy = $derived(pressed !== null);

    async function go(target: number, button: string) {
        if (busy || target < 1 || target > totalPages || target === page) return;
        pressed = button;
        try {
            await onNavigate(target);
        } finally {
            pressed = null;
        }
    }
</script>

{#if total > perPage}
    <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 py-4" data-test={dataTest}>
        <p class="text-xs text-ink-400" data-test="{dataTest}-summary">
            {t('pagination.showing', lang, { from, to, total })}
        </p>
        <!--
            `Button`s: 44 px tall and wide, so a thumb hits one page and not its neighbour, with the
            pressed look. While a page change runs, the pressed one works and none takes a press.
        -->
        <nav class="flex flex-wrap items-center gap-1" aria-label={t('pagination.page', lang)}>
            <Button
                variant="ghost"
                class={arrowClass}
                disabled={page <= 1}
                ariaLabel={t('pagination.first', lang)}
                dataTest="{dataTest}-first"
                pending={pressed === 'first'}
                aria-disabled={busy || undefined}
                onclick={() => go(1, 'first')}
            >
                «
            </Button>
            <Button
                variant="ghost"
                class={arrowClass}
                disabled={page <= 1}
                ariaLabel={t('pagination.previous', lang)}
                dataTest="{dataTest}-prev"
                pending={pressed === 'prev'}
                aria-disabled={busy || undefined}
                onclick={() => go(page - 1, 'prev')}
            >
                ‹
            </Button>

            {#each pageItems as item, i (i)}
                {#if item === 'ellipsis'}
                    <span class="px-2 py-1 text-xs text-ink-300">…</span>
                {:else}
                    <Button
                        variant={item === page ? 'primary' : 'ghost'}
                        class={pageClass}
                        aria-current={item === page ? 'page' : undefined}
                        dataTest="{dataTest}-page-{item}"
                        pending={pressed === `page-${item}`}
                        aria-disabled={busy || undefined}
                        onclick={() => go(item, `page-${item}`)}
                    >
                        {item}
                    </Button>
                {/if}
            {/each}

            <Button
                variant="ghost"
                class={arrowClass}
                disabled={page >= totalPages}
                ariaLabel={t('pagination.next', lang)}
                dataTest="{dataTest}-next"
                pending={pressed === 'next'}
                aria-disabled={busy || undefined}
                onclick={() => go(page + 1, 'next')}
            >
                ›
            </Button>
            <Button
                variant="ghost"
                class={arrowClass}
                disabled={page >= totalPages}
                ariaLabel={t('pagination.last', lang)}
                dataTest="{dataTest}-last"
                pending={pressed === 'last'}
                aria-disabled={busy || undefined}
                onclick={() => go(totalPages, 'last')}
            >
                »
            </Button>
        </nav>
    </div>
{/if}
