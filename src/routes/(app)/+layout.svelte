<script lang="ts">
    import { page } from '$app/state';
    import { api } from '$lib/api/api';
    import { refusalKey } from '$lib/api/refusal-key';
    import AppShell from '$lib/components/shell/AppShell.svelte';
    import Button from '$lib/components/ui/Button.svelte';
    import NoteBox from '$lib/components/ui/NoteBox.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import { getLocale, stripLocale, t } from '$lib/i18n/i18n';
    import { NoticesFeed } from '$lib/notifications/notices-feed.svelte';
    import { session } from '$lib/stores/session.svelte';
    import { onMount, type Snippet } from 'svelte';

    /**
     * The layout of every page of a signed-in owner: it reads who is signed in (`GET /api/v1/session`) and draws the shell around
     * the page only for a person who is. A call answered `signed_out` anywhere sends the person to the sign-in page
     * (`sendToSignIn`), so this layout draws nothing for a person who is out. It also reads the program's version for the menu and
     * keeps the bell's notices fresh, every minute, as a background call that does not prolong the session.
     */
    let { children }: { children: Snippet } = $props();

    const lang = $derived(getLocale());
    const currentPath = $derived(stripLocale(page.url.pathname).path);
    const feed = new NoticesFeed();
    let version = $state<string | null>(null);

    async function begin(): Promise<void> {
        await session.load();
        if (session.status !== 'in') return;
        feed.start(getLocale());
        const meta = await api.getMeta();
        if (meta.ok) version = meta.data.version;
    }

    onMount(() => {
        void begin();
        return () => feed.stop();
    });
</script>

{#if session.status === 'in' && session.current}
    <AppShell {lang} login={session.current.login} {currentPath} lines={feed.lines} {version}>
        {@render children()}
    </AppShell>
{:else if session.status === 'failed'}
    <main class="p-4">
        <Stack>
            <NoteBox tone="error" role="alert" dataTest="app-failed">
                {t(refusalKey(session.refusal ?? 'network'), lang)}
            </NoteBox>
            <div>
                <Button dataTest="app-retry" onclick={begin}>{t('common.retry', lang)}</Button>
            </div>
        </Stack>
    </main>
{:else}
    <main class="p-4">
        <p class="text-ink-600" data-test="app-loading">{t('common.openingApp', lang)}</p>
    </main>
{/if}
