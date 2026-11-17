<script lang="ts">
    import { t } from '$lib/i18n/i18n';
    import Button from './Button.svelte';
    import { disclosureFormOf } from './disclosure-form';

    /**
     * «Закрыть» of a form opened by a press: the form puts it beside its main button. Outside a
     * `DisclosureForm` — the same form drawn always open on another screen — it draws nothing.
     */
    interface IProps {
        dataTest?: string;
        /** The one `DisclosureForm` draws under a form that placed none: it does not count as placed by the form. */
        fallback?: boolean;
    }

    let { dataTest, fallback = false }: IProps = $props();

    const disclosure = disclosureFormOf();
</script>

{#if disclosure}
    <Button
        variant="secondary"
        dataTest={dataTest ?? (disclosure.testId ? `${disclosure.testId}-close` : 'disclosure-close')}
        data-disclosure-close={fallback ? 'own' : 'form'}
        onclick={() => disclosure.close()}
    >
        {t('common.close', disclosure.lang)}
    </Button>
{/if}
