<script lang="ts">
    import DisclosureForm from '$lib/components/ui/DisclosureForm.svelte';
    import DisclosureFormTestBody from './DisclosureFormTestBody.svelte';

    /** One form opened by a press, as a screen draws it. */
    interface IProps {
        after?: 'close' | 'stay';
        withClose?: boolean;
        open?: boolean;
        onClose?: () => void;
        /** The opening button's size and width. */
        size?: 'md' | 'lg' | 'xl';
        fullWidth?: boolean;
        /** «Сохранено.» set from outside, as a screen whose form is sent from code does. */
        saved?: boolean;
        /** A form opened only from code (a row's «Изменить» elsewhere on the screen) draws no opener of its own. */
        showOpener?: boolean;
    }

    let {
        after = 'close',
        withClose = true,
        open = $bindable(false),
        onClose,
        size,
        fullWidth,
        saved = $bindable(false),
        showOpener = true
    }: IProps = $props();
</script>

<DisclosureForm
    label="Изменить"
    lang="ru"
    bind:open
    bind:saved
    {after}
    {onClose}
    {size}
    {fullWidth}
    {showOpener}
    openerTestId="edit"
    testId="edit-form"
>
    <DisclosureFormTestBody {withClose} />
</DisclosureForm>
<button type="button" data-test="open-from-code" onclick={() => (open = true)}>open</button>
<button
    type="button"
    data-test="close-from-code"
    onclick={() => {
        open = false;
        saved = true;
    }}>close saved</button
>
