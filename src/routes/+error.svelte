<script lang="ts">
    import { page } from '$app/state';
    import Button from '$lib/components/ui/Button.svelte';
    import { getLocale, localizePath, t } from '$lib/i18n/i18n';

    /**
     * The root error boundary: it replaces everything below `src/routes/+layout.svelte`, so the page draws its own frame. Its two
     * actions are the shared `Button`, whose look comes from `src/app.css`. It answers an address no page has (404) and a page that
     * failed while it was drawn (every other status); the wyo server's own refusals of calls are worded by the pages that make them.
     */
    interface IErrorCopy {
        /** The large figure above the heading. `null` means "use the translated word instead". */
        statusLabel: string | null;
        titleKey: string;
        bodyKey: string;
    }

    function copyFor(status: number): IErrorCopy {
        if (status === 404) {
            return { statusLabel: '404', titleKey: 'error.404.title', bodyKey: 'error.404.body' };
        }
        return { statusLabel: null, titleKey: 'error.generic.title', bodyKey: 'error.generic.body' };
    }

    const lang = getLocale();
    const copy = $derived(copyFor(page.status));
    const headline = $derived(copy.statusLabel ?? t('error.generic.headline', lang));
    const homeHref = $derived(localizePath('/', lang));
</script>

<div
    class="fixed inset-0 flex items-center justify-center overflow-y-auto bg-ink-50 p-6 text-ink-900"
    data-test="error-page"
>
    <div class="m-auto w-full max-w-xl text-center">
        <p class="mb-2 text-6xl leading-none font-semibold" data-test="error-status">{headline}</p>
        <div class="mx-auto mb-6 h-px w-24 bg-ink-300"></div>
        <h1 class="mb-3 text-2xl font-semibold" data-test="error-title">{t(copy.titleKey, lang)}</h1>
        <p class="mb-8 text-lg text-ink-600" data-test="error-body">{t(copy.bodyKey, lang)}</p>
        <div class="flex flex-col gap-3 sm:flex-row sm:justify-center">
            <Button href={homeHref} dataTest="error-home" class="px-8">{t('error.home', lang)}</Button>
            <Button variant="secondary" dataTest="error-back" class="px-8" onclick={() => window.history.back()}>
                {t('error.back', lang)}
            </Button>
        </div>
    </div>
</div>
