<script lang="ts">
    import { goto } from '$app/navigation';
    import { page } from '$app/state';
    import CaretDownIcon from 'phosphor-svelte/lib/CaretDownIcon';
    import CheckIcon from 'phosphor-svelte/lib/CheckIcon';
    import GlobeIcon from 'phosphor-svelte/lib/GlobeIcon';
    import { tick } from 'svelte';
    import Button from '$lib/components/ui/Button.svelte';
    import IconTooltip from '$lib/components/ui/IconTooltip.svelte';
    import { dismissable } from '$lib/components/ui/dismissable';
    import { getActiveLocales, localizePath, t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';

    interface IProps {
        lang: TLocale;
        /** The language-free current path, so each link keeps the visitor on the same page. */
        currentPath: string;
    }

    let { lang, currentPath }: IProps = $props();

    /** The languages that exist: one file of `messages/` each. The switch is drawn only when there is a choice. */
    const languages = $derived(getActiveLocales());

    /**
     * One button with the current language's code that shows and hides the list of languages: a disclosure of links, not a menu
     * and not a modal — every choice moves to another address, so each is a real link that a middle click opens in a new tab. A
     * plain click moves inside the program instead: the same page, its query and fragment, in the other language, with no page
     * reload.
     */
    const uid = $props.id();
    const listId = `language-list-${uid}`;
    /** Distance kept between the list and the screen's edge. */
    const VIEWPORT_MARGIN = 8;

    let open = $state(false);
    /** The list hangs from the trigger's left edge when hanging from its right one would cut it off the screen. */
    let alignStart = $state(false);
    /** A `goto` from code does not lock the page: the trigger guards itself while it runs. */
    let switching = $state(false);
    let trigger = $state<HTMLElement | undefined>();
    let list = $state<HTMLElement | undefined>();

    const nameOf = (code: TLocale) => t('language.nativeName', code);
    const triggerLabel = $derived(t('shell.language.trigger', lang, { name: nameOf(lang) }));

    function hrefOf(code: TLocale): string {
        return localizePath(currentPath, code) + page.url.search + page.url.hash;
    }

    function items(): HTMLElement[] {
        return Array.from(list?.querySelectorAll<HTMLElement>('a[href]') ?? []);
    }

    function close(): void {
        open = false;
        alignStart = false;
    }

    async function openAndFocus(at: 'first' | 'last'): Promise<void> {
        open = true;
        await tick();
        const all = items();
        (at === 'first' ? all[0] : all.at(-1))?.focus();
    }

    function onTriggerKeydown(event: KeyboardEvent): void {
        if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
        event.preventDefault();
        void openAndFocus(event.key === 'ArrowDown' ? 'first' : 'last');
    }

    /** Down and Up move through the items, round the ends. */
    function onListKeydown(event: KeyboardEvent): void {
        if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
        event.preventDefault();
        const all = items();
        const at = all.indexOf(document.activeElement as HTMLElement);
        const step = event.key === 'ArrowDown' ? 1 : -1;
        all[(at + step + all.length) % all.length]?.focus();
    }

    /** Tab out of the list closes it. */
    function onListFocusout(event: FocusEvent): void {
        const next = event.relatedTarget;
        if (next instanceof Node && (list?.contains(next) || trigger?.contains(next))) return;
        close();
    }

    async function choose(event: MouseEvent, code: TLocale): Promise<void> {
        // A middle click or a click with a modifier key: the browser opens the link itself, in a new tab or window.
        if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
        event.preventDefault();
        close();
        trigger?.focus();
        if (code === lang || switching) return;
        switching = true;
        try {
            await goto(hrefOf(code), { invalidateAll: true, keepFocus: true, noScroll: true });
        } finally {
            switching = false;
        }
    }

    // Escape (the focus back on the trigger) and a click outside close the list.
    $effect(() => {
        if (!open || !list) return;
        return dismissable(list, { onDismiss: close, trigger: () => trigger });
    });

    // Kept on screen: hanging from the trigger's right edge, a list near the left edge of a phone would run off it.
    $effect(() => {
        if (!open || !list) return;
        if (list.getBoundingClientRect().left < VIEWPORT_MARGIN) alignStart = true;
    });
</script>

{#if languages.length > 1}
    <nav aria-label={t('shell.language.nav', lang)} data-test="language-switcher" class="relative">
        <!-- Below 768 px a globe with the language's code, with a tooltip. -->
        <IconTooltip
            name={triggerLabel}
            description={t('shell.language.hint', lang)}
            onlyBelowMd
            testId="language-trigger-tip"
        >
            <Button
                bind:element={trigger}
                variant="ghost-inverse"
                class="gap-1 px-2"
                pending={switching}
                ariaLabel={triggerLabel}
                title={triggerLabel}
                aria-expanded={open}
                aria-controls={listId}
                dataTest="language-trigger"
                onclick={() => (open ? close() : (open = true))}
                onkeydown={onTriggerKeydown}
            >
                <GlobeIcon class="size-5 md:hidden" aria-hidden="true" />
                {lang}
                {#if !switching}
                    <CaretDownIcon class="hidden md:inline" size="0.875rem" aria-hidden="true" />
                {/if}
            </Button>
        </IconTooltip>
        <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
        <ul
            bind:this={list}
            id={listId}
            hidden={!open}
            class="absolute top-full z-50 mt-1 w-max min-w-44 rounded border border-ink-200 bg-white py-1 text-ink-800 shadow-md {alignStart
                ? 'left-0'
                : 'right-0'}"
            data-test="language-list"
            onkeydown={onListKeydown}
            onfocusout={onListFocusout}
        >
            {#each languages as code (code)}
                {@const current = code === lang}
                <li>
                    <Button
                        href={hrefOf(code)}
                        hreflang={code}
                        lang={code}
                        variant="ghost"
                        fullWidth
                        class="justify-start gap-3 rounded-none px-4 {current ? 'font-semibold text-ink-900' : ''}"
                        aria-current={current ? 'true' : undefined}
                        dataTest="language-option-{code}"
                        onclick={(event: MouseEvent) => choose(event, code)}
                    >
                        <span class="w-6 text-xs uppercase">{code}</span><span class="flex-1 text-left text-sm"
                            >{nameOf(code)}</span
                        >
                        {#if current}
                            <CheckIcon size="1rem" aria-hidden="true" />
                        {/if}
                    </Button>
                </li>
            {/each}
        </ul>
    </nav>
{/if}
