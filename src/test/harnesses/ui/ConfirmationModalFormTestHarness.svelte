<script lang="ts">
    import ConfirmationModal from '$lib/components/ui/ConfirmationModal.svelte';
    import GuardedForm from '$lib/components/ui/GuardedForm.svelte';

    /** The confirm-then-submit pattern: a form elsewhere on the page. */
    let open = $state(true);
</script>

<GuardedForm id="task-cancel-form" dataTest="task-cancel-form">
    <input type="hidden" name="taskId" value="t-1" />
</GuardedForm>

<ConfirmationModal
    bind:open
    title="Cancel the task?"
    message="The reserved stock is released."
    confirmType="submit"
    confirmForm="task-cancel-form"
    dataTest="task-cancel-modal"
/>
