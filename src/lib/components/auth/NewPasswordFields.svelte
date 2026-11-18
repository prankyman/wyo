<script lang="ts">
    import Field from '$lib/components/ui/Field.svelte';
    import TextBox from '$lib/components/ui/TextBox.svelte';
    import { tError } from '$lib/i18n/form-error';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import type { IPasswordBoxIds, TPasswordBoxName, TPasswordErrorLook } from './types';

    /**
     * The «new password + confirm» boxes, with the current-password box above them when `withCurrent`. The
     * first-password page, the reset page, the sign-up page and both «Мой профиль» pages draw them through it. The caller
     * binds the three values, passes the refusals as they come out of its superform, and says what to do when a box is
     * left (`validate`). The ids and `data-test` values differ from place to place, so each place passes its own.
     */
    interface IProps {
        lang: TLocale;
        /** Whether the current-password box is drawn first. */
        withCurrent?: boolean;
        /** The ids and test ids of the boxes; `current` is needed only with `withCurrent`. */
        ids: { current?: IPasswordBoxIds; next: IPasswordBoxIds; confirm: IPasswordBoxIds };
        /** The look of the refusal under each box. */
        errorLook?: TPasswordErrorLook;
        /** What the «?» beside each box says; a box without a text has no «?». */
        helps?: { current?: string; next?: string; confirm?: string };
        /** The refusals of the three boxes, as the superform's errors hold them. */
        errors?: { current?: unknown; next?: unknown; confirm?: unknown };
        /** Called with the name of the box the person has just left. */
        validate?: (name: TPasswordBoxName) => void;
        current?: string;
        next?: string;
        confirm?: string;
    }

    let {
        lang,
        withCurrent = false,
        ids,
        errorLook = 'line',
        errors = {},
        helps = {},
        validate = () => {},
        current = $bindable(''),
        next = $bindable(''),
        confirm = $bindable('')
    }: IProps = $props();
</script>

{#snippet refusal(look: TPasswordErrorLook, error: unknown, box: IPasswordBoxIds)}
    {#if look === 'line'}
        <!-- A line kept for the message, so one coming or going never moves the button. -->
        <span class="min-h-4 text-xs text-stop-700">
            {#if error}
                <span data-test={box.errorTest}>{tError(error, lang)}</span>
            {/if}
        </span>
    {:else if look === 'text' && error}
        <span class="text-xs text-stop-700" data-test={box.errorTest}>{tError(error, lang)}</span>
    {/if}
{/snippet}

{#if withCurrent && ids.current}
    <Field
        label={t('account.currentPassword', lang)}
        help={helps.current}
        for={ids.current.id}
        error={errorLook === 'field' ? tError(errors.current, lang) : null}
        errorTestId={ids.current.errorTest}
    >
        <TextBox
            id={ids.current.id}
            name="currentPassword"
            type="password"
            data-test={ids.current.test}
            bind:value={current}
            onfocusout={() => validate('currentPassword')}
            autocomplete="current-password"
            required
        />
        {@render refusal(errorLook, errors.current, ids.current)}
    </Field>
{/if}
<Field
    label={t('account.newPassword', lang)}
    help={helps.next}
    for={ids.next.id}
    error={errorLook === 'field' ? tError(errors.next, lang) : null}
    errorTestId={ids.next.errorTest}
>
    <TextBox
        id={ids.next.id}
        name="newPassword"
        type="password"
        data-test={ids.next.test}
        bind:value={next}
        onfocusout={() => validate('newPassword')}
        autocomplete="new-password"
        required
    />
    {@render refusal(errorLook, errors.next, ids.next)}
</Field>
<Field
    label={t('account.confirmPassword', lang)}
    help={helps.confirm}
    for={ids.confirm.id}
    error={errorLook === 'field' ? tError(errors.confirm, lang) : null}
    errorTestId={ids.confirm.errorTest}
>
    <TextBox
        id={ids.confirm.id}
        name="confirmNewPassword"
        type="password"
        data-test={ids.confirm.test}
        bind:value={confirm}
        onfocusout={() => validate('confirmNewPassword')}
        autocomplete="new-password"
        required
    />
    {@render refusal(errorLook, errors.confirm, ids.confirm)}
</Field>
