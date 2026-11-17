<script lang="ts">
    import Button from '$lib/components/ui/Button.svelte';
    import GuardedForm from '$lib/components/ui/GuardedForm.svelte';
    import type { IFormConfirm, IGuardedSuperForm } from '$lib/components/ui/types';

    /**
     * A guarded form the way the screens build one: a field, the submit button that carries a
     * `name`/`value`, a second submit button of the same form, and a button OUTSIDE
     * the form that submits it through `form="…"` — the `ConfirmationModal` pattern.
     */
    interface IProps {
        method?: 'GET' | 'POST';
        target?: string;
        superform?: IGuardedSuperForm;
        confirm?: IFormConfirm;
    }

    let { method = 'POST', target, superform, confirm }: IProps = $props();
</script>

<GuardedForm id="harness-form" {method} {target} {superform} {confirm} dataTest="harness-form">
    <input name="note" value="hello" />
    <Button type="submit" name="intent" value="save" dataTest="save">Save</Button>
    <Button type="submit" variant="secondary" dataTest="other">Other</Button>
</GuardedForm>

<Button type="submit" form="harness-form" variant="danger" dataTest="outside">Confirm</Button>
