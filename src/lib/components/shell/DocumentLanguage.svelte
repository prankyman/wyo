<script lang="ts">
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';

    interface IProps {
        locale: TLocale;
    }

    let { locale }: IProps = $props();

    /**
     * The page's language and its writing direction follow the address. A language switch is a move inside the program that keeps
     * the page, so without this a screen reader would go on reading in the old language and a right-to-left language would keep
     * the old direction. The direction is a text of the language's own file (`language.direction`), so a new language needs no
     * change in code.
     */
    const direction = $derived(t('language.direction', locale) === 'rtl' ? 'rtl' : 'ltr');

    $effect(() => {
        const html = document.documentElement;
        html.lang = locale;
        html.dir = direction;
    });
</script>
