<script lang="ts">
    import { localizePath, t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import { NavGroupsState } from '$lib/nav/nav-groups.svelte';
    import { NAV_ITEMS, type INavCounter } from '$lib/nav/nav-items';
    import type { INotificationLine } from '$lib/notifications/types';
    import { untrack, type Snippet } from 'svelte';
    import AppNav from './AppNav.svelte';
    import LanguageSwitcher from './LanguageSwitcher.svelte';
    import NotificationBell from './NotificationBell.svelte';
    import ShellHeader from './ShellHeader.svelte';
    import ShellUserBlock from './ShellUserBlock.svelte';

    /**
     * The frame of every page of a signed-in owner: the top bar in the brand colour — the program's name at the left, the language
     * switch (drawn only while more than one language exists), the bell and the owner's login at the right — the menu, and the
     * page between them. The menu is ONE list (`src/lib/nav/nav-items.ts`) drawn twice: a column at the left from 1024 pixels, a
     * strip along the bottom of a phone below it; the same state of its groups is shared by both. The page keeps room under it for the strip,
     * so a fixed element never covers the page.
     */
    interface IProps {
        lang: TLocale;
        /** The owner's login, which opens «Мой профиль». */
        login: string;
        /** Path with the language prefix already stripped, e.g. `/settings`. */
        currentPath: string;
        /** The lines of the bell's panel. */
        lines: INotificationLine[];
        /** The version line at the bottom of the menu; null draws none. */
        version: string | null;
        /** The counts beside menu items, by `testId`; none today. */
        counters?: Readonly<Record<string, INavCounter>>;
        children: Snippet;
    }

    let { lang, login, currentPath, lines, version, counters = {}, children }: IProps = $props();

    // One state of the menu's groups for both layouts of it: the left column and the «Ещё» sheet.
    const groups = untrack(() => new NavGroupsState(currentPath, NAV_ITEMS));
</script>

<header class="sticky top-0 z-40 bg-brand-600 text-white" data-test="app-header">
    <ShellHeader>
        {#snippet left()}
            <a
                href={localizePath('/', lang)}
                class="inline-flex min-h-11 min-w-11 items-center text-xl font-semibold text-white"
                data-test="app-brand">wyo</a
            >
        {/snippet}
        <LanguageSwitcher {lang} {currentPath} />
        <NotificationBell {lang} {lines} helpText={t('shell.notifications.help', lang)} />
        <ShellUserBlock {login} {lang} profileHref={localizePath('/account', lang)} />
    </ShellHeader>
</header>

<div class="px-4 lg:grid lg:grid-cols-[14rem_minmax(0,1fr)] lg:gap-6">
    <AppNav {lang} {currentPath} variant="sidebar" {version} {counters} {groups} />
    <main class="min-w-0 py-4 pb-[calc(4.5rem+env(safe-area-inset-bottom))] lg:pb-6" data-test="app-main">
        {@render children()}
    </main>
</div>
<AppNav {lang} {currentPath} variant="tabstrip" {version} {counters} {groups} />
