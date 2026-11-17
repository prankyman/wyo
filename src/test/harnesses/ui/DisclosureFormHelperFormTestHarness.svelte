<script lang="ts">
    import DisclosureForm from '$lib/components/ui/DisclosureForm.svelte';
    import DisclosureFormHelperFormTestBody from './DisclosureFormHelperFormTestBody.svelte';

    /** A panel with its own form and a helper form inside. */
</script>

<DisclosureForm label="Прибыла" lang="ru" openerTestId="edit" testId="edit-form">
    <DisclosureFormHelperFormTestBody />
</DisclosureForm>
