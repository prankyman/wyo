<script lang="ts">
    import { api } from '$lib/api/api';
    import NewPasswordFields from '$lib/components/auth/NewPasswordFields.svelte';
    import type { IPasswordBoxIds } from '$lib/components/auth/types';
    import ButtonWithHelp from '$lib/components/ui/ButtonWithHelp.svelte';
    import CollapsibleSection from '$lib/components/ui/CollapsibleSection.svelte';
    import ConfirmationModal from '$lib/components/ui/ConfirmationModal.svelte';
    import FormAnswer from '$lib/components/ui/FormAnswer.svelte';
    import NoteBox from '$lib/components/ui/NoteBox.svelte';
    import PageHeading from '$lib/components/ui/PageHeading.svelte';
    import SectionForm from '$lib/components/ui/SectionForm.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import { sendToSignIn } from '$lib/api/sign-in-redirect';
    import { ApiAction } from '$lib/forms/api-action.svelte';
    import type { ISectionSuperForm } from '$lib/forms/section-form';
    import { spaForm } from '$lib/forms/super-form';
    import { getLocale, t } from '$lib/i18n/i18n';
    import { session } from '$lib/stores/session.svelte';
    import { PASSWORD_MIN_LENGTH, passwordChangeSchema } from '$lib/validation/password-change';

    /**
     * «Мой профиль»: who is signed in, the change of the password (`PUT /api/v1/account/password`; it ends every other session, so
     * the owner stays signed in only here), «Выйти» of this browser (`DELETE /api/v1/session`) and «Выйти на всех устройствах»
     * (`DELETE /api/v1/sessions`), behind a window that says what it does. The three boxes of the password are
     * `NewPasswordFields`; a box holding a password empties after a save, because the wyo server never sends one back.
     */
    const lang = getLocale();

    const BOXES: { current: IPasswordBoxIds; next: IPasswordBoxIds; confirm: IPasswordBoxIds } = {
        current: { id: 'account-current-box', test: 'account-current', errorTest: 'account-current-error' },
        next: { id: 'account-new-box', test: 'account-new', errorTest: 'account-new-error' },
        confirm: { id: 'account-confirm-box', test: 'account-confirm', errorTest: 'account-confirm-error' }
    };

    const made = spaForm(
        passwordChangeSchema,
        { currentPassword: '', newPassword: '', confirmNewPassword: '' },
        {
            id: 'account-password-form',
            resetForm: true,
            saved: 'section',
            send: ({ currentPassword, newPassword }) =>
                api.put('/api/v1/account/password', { currentPassword, newPassword })
        }
    );
    const { form, errors } = made;

    const signOut = new ApiAction();
    const signOutEverywhere = new ApiAction();
    let asking = $state(false);

    /** Ends this browser's session, or every session, and then the person is on the sign-in page. */
    async function leave(action: ApiAction, call: () => ReturnType<typeof api.delete>): Promise<void> {
        const result = await action.run(call);
        if (result?.ok) sendToSignIn();
    }
</script>

<Stack as="section" dataTest="account">
    <PageHeading
        title={t('account.title', lang)}
        help={t('account.help', lang)}
        headingTestId="account-heading"
        helpTestId="account-help"
    />
    <p class="text-sm text-ink-700" data-test="account-login">
        {t('account.signedInAs', lang, { login: session.current?.login ?? '' })}
    </p>
    {#if session.current?.mustChangePassword}
        <NoteBox tone="warning" role="status" dataTest="account-must-change">{t('account.mustChange', lang)}</NoteBox>
    {/if}

    <SectionForm
        {lang}
        id="account-password"
        title={t('account.password.title', lang)}
        help={t('account.password.help', lang)}
        form={made as unknown as ISectionSuperForm}
    >
        <NewPasswordFields
            {lang}
            withCurrent
            errorLook="field"
            ids={BOXES}
            helps={{
                current: t('account.currentPasswordHelp', lang),
                next: t('account.newPasswordHelp', lang, { min: PASSWORD_MIN_LENGTH }),
                confirm: t('account.confirmPasswordHelp', lang)
            }}
            errors={{
                current: $errors.currentPassword,
                next: $errors.newPassword,
                confirm: $errors.confirmNewPassword
            }}
            validate={(name) => made.validate(name)}
            bind:current={$form.currentPassword}
            bind:next={$form.newPassword}
            bind:confirm={$form.confirmNewPassword}
        />
    </SectionForm>

    <CollapsibleSection
        {lang}
        id="account-signout"
        title={t('account.signOut.title', lang)}
        help={t('account.signOut.help', lang)}
    >
        <Stack>
            <div>
                <ButtonWithHelp
                    variant="secondary"
                    pending={signOut.pending}
                    dataTest="account-sign-out"
                    help={t('account.signOut.here.help', lang)}
                    helpTestId="account-sign-out-help"
                    helpLabel={t('account.signOut.here', lang)}
                    onclick={() => leave(signOut, () => api.delete('/api/v1/session'))}
                >
                    {t('account.signOut.here', lang)}
                </ButtonWithHelp>
                <FormAnswer error={signOut.error} dataTest="account-sign-out-answer" />
            </div>
            <div>
                <ButtonWithHelp
                    variant="secondary"
                    dataTest="account-sign-out-everywhere"
                    help={t('account.signOut.everywhere.help', lang)}
                    helpTestId="account-sign-out-everywhere-help"
                    helpLabel={t('account.signOut.everywhere', lang)}
                    onclick={() => (asking = true)}
                >
                    {t('account.signOut.everywhere', lang)}
                </ButtonWithHelp>
            </div>
        </Stack>
    </CollapsibleSection>

    <ConfirmationModal
        bind:open={asking}
        title={t('account.everywhere.title', lang)}
        message={t('account.everywhere.message', lang)}
        confirmText={t('account.everywhere.confirm', lang)}
        cancelText={t('app.common.cancel', lang)}
        dataTest="account-everywhere-window"
        onConfirm={() => {
            void leave(signOutEverywhere, () => api.delete('/api/v1/sessions'));
            // The window stays open while the call runs, so a refusal is read inside it; it goes with the page when the person signs out.
            return false;
        }}
    >
        <FormAnswer error={signOutEverywhere.error} dataTest="account-everywhere" />
    </ConfirmationModal>
</Stack>
