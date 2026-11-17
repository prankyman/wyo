<script lang="ts">
    import DisclosureClose from '$lib/components/ui/DisclosureClose.svelte';
    import { superForm } from '$lib/forms/super-form';

    /**
     * A panel holding two forms: its own form and a helper form — a photo upload, say, whose answer is the upload's to read
     * and never the panel's «Сохранено» — both made by `superForm` inside it. The library is mocked by the test, which
     * answers each form itself.
     */
    superForm({ photo: '' }, { id: 'helper-photo', dataType: 'json', disclosure: false });
    superForm({ title: 'Zebra' }, { resetForm: false });
</script>

<input name="title" value="Zebra" data-test="title" />
<DisclosureClose />
