<script lang="ts">
    import PageHeading from '$lib/components/ui/PageHeading.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import { t } from '$lib/i18n/i18n';
    import ShowcaseBasics from './ShowcaseBasics.svelte';
    import ShowcaseForms from './ShowcaseForms.svelte';
    import ShowcaseLists from './ShowcaseLists.svelte';
</script>

<Stack as="section" dataTest="showcase">
    <PageHeading title={t('showcase.title')} help={t('showcase.help')} helpTestId="showcase-help" />
    <ShowcaseBasics />
    <ShowcaseForms />
    <ShowcaseLists />
</Stack>
