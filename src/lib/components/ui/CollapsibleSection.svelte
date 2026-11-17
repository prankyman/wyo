<script lang="ts">
    import { browser } from '$app/environment';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import { onMount, type Snippet } from 'svelte';
    import { focusFirstRefusedField, openedBeforeStart } from './collapsible-section';
    import HelpTooltip from './HelpTooltip.svelte';
    import { FOCUS_RING, PRESS_FEEDBACK } from './press';
    import type { IHelpLink } from './types';

    interface IProps {
        /** The language of the hidden «нажмите, чтобы открыть / закрыть» text. */
        lang: TLocale;
        /**
         * The section's id, also its `data-test` and the address fragment that opens it (`#<id>`). Its heading row is
         * `<id>-heading`, its arrow `<id>-arrow`.
         */
        id: string;
        /** The guided tours' anchor of the section; the heading row is `<dataTour>-heading`. Default: the id. */
        dataTour?: string;
        /** The section's visible name. */
        title: string;
        /**
         * The section's «?», drawn in the heading row right after the title: a «?» sits immediately to the
         * right of the words it explains, never on a line of its own under them; without it there is none.
         */
        help?: string;
        /** Links under the «?»'s text, each opening in a new tab. */
        helpLinks?: IHelpLink[];
        /** `data-test` prefix of the «?» (`<helpTestId>-button`, `<helpTestId>-tooltip`); `<id>-help` by default. */
        helpTestId?: string;
        /** Open on arrival. Every section arrives closed, except on the create screens. */
        open?: boolean;
        /** `h2` for a screen's own sections, `h3` or `h4` for a section inside a card that has its own heading. */
        headingLevel?: 'h2' | 'h3' | 'h4';
        /** Classes of the outer box: its place in a grid. Never a margin: the `Stack` it stands in spaces it. */
        class?: string;
        /** Classes of the body (a column with a gap between its parts). */
        contentClass?: string;
        /** A short count or badge drawn in the heading row after the title — text only, never a button. */
        summary?: Snippet;
        children: Snippet;
    }

    let {
        lang,
        id,
        dataTour,
        title,
        help,
        helpLinks,
        helpTestId,
        open = false,
        headingLevel = 'h2',
        class: className = '',
        contentClass = '',
        summary,
        children
    }: IProps = $props();

    // The state is the native <details> `open`; the prop only says how it arrives. A section the person opened before the
    // page's script started stays open: the start-up would otherwise write the arrival state over it.
    // svelte-ignore state_referenced_locally
    let isOpen = $state(open || (browser && openedBeforeStart(id)));
    let section: HTMLDetailsElement | undefined = $state();
    let body: HTMLDivElement | undefined = $state();
    const anchor = $derived(dataTour ?? id);

    /** When a section looks for a refused box after its form was sent: at once, and once the form's own check has answered. */
    const SEND_CHECKS_MS = [0, 150, 600];
    /** How long a section's own send waits for the server's answer to refuse a box (a slow answer comes after 600 ms). */
    const OWN_SEND_WAIT_MS = 15_000;
    /** Until when a refusal drawn in the section answers its own send: the box it names then takes the cursor. */
    let ownSendUntil = 0;
    /** The timed looks of the section's own send, cancelled when the person types again. */
    let ownLooks: ReturnType<typeof setTimeout>[] = [];

    /** A `<summary>` cannot be a `Button`: it takes the pressed look and the focus ring from `press.ts`. */
    const summaryClass = `flex min-h-11 cursor-pointer list-none items-center gap-2 px-4 py-2 select-none ${PRESS_FEEDBACK} ${FOCUS_RING} [&::-webkit-details-marker]:hidden`;

    /** A press on the «?» in the heading row must not fold or unfold the section: the row's own action is cancelled. */
    function keepSectionOnHelpPress(event: MouseEvent): void {
        if (event.target instanceof Element && event.target.closest('[data-help-icon]')) event.preventDefault();
    }

    /** An address ending in the section's id opens it and scrolls to it. */
    function openWhenNamed() {
        if (window.location.hash !== `#${id}`) return;
        isOpen = true;
        section?.scrollIntoView?.({ block: 'start' });
    }

    /**
     * A refused field inside opens the section. The browser's own check fires `invalid` at each refused
     * control without bubbling, so it is caught on the way down (the capture phase); the section opens before the
     * browser points at the first refused box. A refusal of the server is drawn by `Field`, whose box then carries
     * `data-field-error`.
     */
    function openForInvalid() {
        // Written to the element at once, not through the state: the browser points at the refused box right after
        // these events, before the page would redraw. The `toggle` event then brings the state along.
        if (section) section.open = true;
    }

    /** Only a closed section opens and points at its box: a box refused while the person types in it is in view already. */
    function openForRefusal() {
        if (isOpen || !body?.querySelector('[data-field-error]')) return;
        isOpen = true;
        focusFirstRefusedField();
    }

    /**
     * A send of the form around the section: a box refused before (its message shown while the section was still open, then
     * the section closed) changes nothing on the page, so the section looks again once the form's own check has run.
     */
    function checkAfterSend(event: SubmitEvent): void {
        if (!(event.target instanceof HTMLFormElement) || !section) return;
        if (event.target.contains(section)) {
            for (const delay of SEND_CHECKS_MS) setTimeout(openForRefusal, delay);
        } else if (section.contains(event.target)) {
            // The section is one form of its own and is open while it is sent; a box its answer refuses takes
            // the cursor, as a refused box of a card that spans its sections does.
            ownSendUntil = Date.now() + OWN_SEND_WAIT_MS;
            ownLooks = SEND_CHECKS_MS.map((delay) => setTimeout(focusOwnRefusal, delay));
        }
    }

    /** The section's own refused box takes the cursor — looked for inside the section only. */
    function focusOwnRefusal(): void {
        if (body?.querySelector('[data-field-error]')) focusFirstRefusedField(document, body);
    }

    /**
     * The person types or chooses in the section again: the send is behind them, and a refusal drawn now — a box
     * they left — must not take the cursor out of the box they are typing in. Before this, a text typed in the 15 seconds
     * after a save went into another box or nowhere when such a refusal appeared.
     */
    function onBodyInput(): void {
        ownSendUntil = 0;
        for (const look of ownLooks) clearTimeout(look);
        ownLooks = [];
    }

    /** A box refused inside the section: a closed section opens; an answer to the section's own send takes the cursor. */
    function onBodyChange(): void {
        openForRefusal();
        if (Date.now() >= ownSendUntil || !body?.querySelector('[data-field-error]')) return;
        ownSendUntil = 0;
        focusFirstRefusedField(document, body);
    }

    onMount(() => {
        openWhenNamed();
        openForRefusal();
        window.addEventListener('hashchange', openWhenNamed);
        document.addEventListener('submit', checkAfterSend, true);
        body?.addEventListener('input', onBodyInput);
        const watcher = new MutationObserver(onBodyChange);
        if (body) watcher.observe(body, { subtree: true, childList: true, attributeFilter: ['data-field-error'] });
        return () => {
            window.removeEventListener('hashchange', openWhenNamed);
            document.removeEventListener('submit', checkAfterSend, true);
            body?.removeEventListener('input', onBodyInput);
            watcher.disconnect();
        };
    });
</script>

<!--
    One collapsible section of a screen. A
    native <details>: the browser gives the open and closed state, the keyboard handling (Enter and Space on the heading
    row) and the toggle event, and the closed contents stay in the page, so «Сохранить» of a form that spans several
    sections still posts the boxes of a closed one. The whole heading row, at least 44 px tall, is the control; the arrow
    ▸ (closed) or ▾ (open) is a triangle drawn as a shape, not a font character (a font without the character would show
    an empty box), turned by the rotation below. `data-collapsible` is the mark the browser tests' helpers look for.
-->
<details
    bind:this={section}
    bind:open={isOpen}
    {id}
    class="group rounded border border-ink-200 bg-white {className}"
    data-collapsible
    data-test={id}
    data-tour={anchor}
    oninvalidcapture={openForInvalid}
>
    <summary
        class={summaryClass}
        data-test="{id}-heading"
        data-tour="{anchor}-heading"
        onclick={keepSectionOnHelpPress}
    >
        <span
            class="inline-flex shrink-0 text-ink-500 transition-transform group-open:rotate-90"
            aria-hidden="true"
            data-test="{id}-arrow"
        >
            <svg viewBox="0 0 16 16" class="size-4" fill="currentColor"><path d="M5 3l7 5-7 5z" /></svg>
        </span>
        <svelte:element this={headingLevel} class="min-w-0 text-sm font-medium text-ink-900">{title}</svelte:element>
        {#if help}
            <HelpTooltip tooltip={help} links={helpLinks} ariaLabel={title} testId={helpTestId ?? `${id}-help`} />
        {/if}
        {#if summary}
            <span class="ml-auto shrink-0 text-xs text-ink-500" data-test="{id}-summary">{@render summary()}</span>
        {/if}
        <span class="sr-only">
            {isOpen ? t('app.settings.group.pressToClose', lang) : t('app.settings.group.pressToOpen', lang)}
        </span>
    </summary>
    <div bind:this={body} class="border-t border-ink-100 px-4 pt-2 pb-4 {contentClass}">
        {@render children()}
    </div>
</details>
