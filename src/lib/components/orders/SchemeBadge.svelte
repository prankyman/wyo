<script lang="ts">
    import StatusBadgeWithHelp from '$lib/components/ui/StatusBadgeWithHelp.svelte';
    import type { TTone } from '$lib/components/ui/status-tones';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import type { TOrderScheme } from './types';

    /**
     * Who packs an order (`api/wyo.yaml`, `OrderScheme`): FBS — the seller packs it and hands it to the marketplace; FBO — the
     * marketplace packs it from its own warehouse, and the order is shown read-only. A badge with a «?» that says which is which.
     */
    interface IProps {
        lang: TLocale;
        scheme: TOrderScheme;
        /** The badge is `<testId>`, its «?» `<testId>-help`. */
        testId?: string;
    }

    let { lang, scheme, testId }: IProps = $props();

    const TONE: Readonly<Record<TOrderScheme, TTone>> = { fbs: 'info', fbo: 'off' };
</script>

<StatusBadgeWithHelp
    label={t(`orders.scheme.${scheme}`, lang)}
    tone={TONE[scheme]}
    help={t(`orders.scheme.${scheme}.help`, lang)}
    {testId}
/>
