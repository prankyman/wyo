<script lang="ts">
    import { hasMessage, t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';

    /**
     * A marketplace as the interface shows it: a small coloured dot and the marketplace's name. Both come from the adapter's
     * description (`GET /api/v1/marketplaces`): the dot's colour is the theme's mark the adapter names (`--color-mark-<name>` in
     * `src/app.css`), the name is the text `marketplace.<code>.name`. No screen writes a marketplace's name or colour itself.
     */
    interface IProps {
        lang: TLocale;
        /** The adapter's code. */
        code: string;
        /** The name of the adapter's colour mark; a neutral grey when it is not known. */
        mark?: string;
        /** The mark is `<dataTest>`, its dot `<dataTest>-dot`. */
        dataTest?: string;
    }

    let { lang, code, mark, dataTest }: IProps = $props();

    /** The marks the theme holds, each a whole class so the styles find it. */
    const MARK_DOT_CLASS: Readonly<Record<string, string>> = {
        plum: 'bg-mark-plum',
        azure: 'bg-mark-azure',
        yellow: 'bg-mark-yellow'
    };

    const dotClass = $derived((mark !== undefined && MARK_DOT_CLASS[mark]) || 'bg-ink-400');
    const name = $derived(hasMessage(`marketplace.${code}.name`, lang) ? t(`marketplace.${code}.name`, lang) : code);
</script>

<span class="inline-flex min-w-0 items-center gap-1.5" data-test={dataTest}>
    <span
        class="inline-block size-2.5 shrink-0 rounded-full {dotClass}"
        aria-hidden="true"
        data-test={dataTest ? `${dataTest}-dot` : undefined}
    ></span>
    <span class="min-w-0 break-words">{name}</span>
</span>
