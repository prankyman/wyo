<script lang="ts">
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';

    /**
     * A box's refusal, drawn under the box: the first message key superforms holds for the
     * field, worded in the page's language. Nothing is drawn while the box is accepted.
     */
    interface IProps {
        lang: TLocale;
        /** The field's messages from the superform's `$errors` (message keys). */
        messages: string[] | undefined;
        dataTest?: string;
    }

    let { lang, messages, dataTest }: IProps = $props();
</script>

{#if messages?.length}
    <span role="alert" class="text-xs text-stop-800" data-test={dataTest}>{t(messages[0], lang)}</span>
{/if}
