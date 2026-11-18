<script lang="ts">
    import '../app.css';
    import type { Snippet } from 'svelte';
    import { onMount } from 'svelte';
    import { setSignedOutHandler } from '$lib/api/api';
    import { sendToSignIn } from '$lib/api/sign-in-redirect';
    import DocumentLanguage from '$lib/components/shell/DocumentLanguage.svelte';
    import PageChangeGuard from '$lib/components/ui/PageChangeGuard.svelte';
    import { getLocale } from '$lib/i18n/i18n';

    let { children }: { children: Snippet } = $props();

    const lang = $derived(getLocale());

    // Any call answered `signed_out`, on any page, sends the person to the sign-in page.
    onMount(() => {
        setSignedOutHandler(sendToSignIn);
        return () => setSignedOutHandler(null);
    });
</script>

<!-- The root draws no frame: a page of a signed-in owner is framed by the shell (`(app)/+layout.svelte`), the sign-in page by `AuthShell`. -->
<DocumentLanguage locale={lang} />
<PageChangeGuard {lang}>
    {@render children()}
</PageChangeGuard>
