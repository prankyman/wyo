<script lang="ts">
    import { goto } from '$app/navigation';
    import { api } from '$lib/api/api';
    import AuthShell from '$lib/components/auth/AuthShell.svelte';
    import Button from '$lib/components/ui/Button.svelte';
    import Field from '$lib/components/ui/Field.svelte';
    import FormAnswer from '$lib/components/ui/FormAnswer.svelte';
    import FormButtons from '$lib/components/ui/FormButtons.svelte';
    import GuardedForm from '$lib/components/ui/GuardedForm.svelte';
    import NoteBox from '$lib/components/ui/NoteBox.svelte';
    import PageHeading from '$lib/components/ui/PageHeading.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import TextBox from '$lib/components/ui/TextBox.svelte';
    import { answerLines, answerOf } from '$lib/forms/answer';
    import { spaForm } from '$lib/forms/super-form';
    import { tError } from '$lib/i18n/form-error';
    import { getLocale, localizePath, t } from '$lib/i18n/i18n';
    import { session, type TSession } from '$lib/stores/session.svelte';
    import { signInSchema } from '$lib/validation/sign-in';

    /**
     * The sign-in page: a login and a password, «Войти» (`POST /api/v1/session`), and «Забыли пароль?». The wyo interface never offers
     * to create an account: the installer creates the one owner. A refusal is worded under the button; the password box is emptied
     * after every answer, because a box holding a secret never keeps it. A forgotten password is set again on the server, since
     * wyo sends no e-mail.
     */
    const lang = getLocale();

    const made = spaForm(
        signInSchema,
        { login: '', password: '' },
        {
            id: 'login-form',
            resetForm: false,
            send: async (values) => {
                const result = await api.post<TSession>('/api/v1/session', values);
                if (result.ok) {
                    session.signedIn(result.data);
                    await goto(localizePath(result.data.mustChangePassword ? '/account' : '/', lang));
                }
                return result;
            },
            // After every refusal the password box is empty again, the login stays.
            onUpdated: ({ form: updated }) => {
                if (answerOf(updated.message)?.error !== undefined)
                    form.update((current) => ({ ...current, password: '' }));
            }
        }
    );
    const { form, errors, message, enhance, submitting } = made;
    const answer = $derived(answerLines($message, lang, ''));

    let forgot = $state(false);
</script>

<AuthShell>
    <PageHeading
        title={t('login.title', lang)}
        help={t('login.help', lang)}
        headingTestId="login-heading"
        helpTestId="login-help"
    />
    <GuardedForm id="login-form" dataTest="login-form" superform={{ enhance, submitting }}>
        <Stack>
            <Field
                label={t('login.login', lang)}
                help={t('login.loginHelp', lang)}
                for="login-login-box"
                testId="login-login"
                error={tError($errors.login, lang)}
            >
                <TextBox
                    id="login-login-box"
                    name="login"
                    autocomplete="username"
                    autocapitalize="none"
                    bind:value={$form.login}
                    invalid={Boolean($errors.login)}
                    dataTest="login-login"
                />
            </Field>
            <Field
                label={t('login.password', lang)}
                help={t('login.passwordHelp', lang)}
                for="login-password-box"
                testId="login-password"
                error={tError($errors.password, lang)}
            >
                <TextBox
                    id="login-password-box"
                    name="password"
                    type="password"
                    autocomplete="current-password"
                    bind:value={$form.password}
                    invalid={Boolean($errors.password)}
                    dataTest="login-password"
                />
            </Field>
            <FormButtons>
                <Button type="submit" dataTest="login-send">{t('login.send', lang)}</Button>
                {#snippet below()}
                    <FormAnswer error={answer.error} dataTest="login-answer" />
                {/snippet}
            </FormButtons>
        </Stack>
    </GuardedForm>
    <div>
        <Button variant="link" dataTest="login-forgot" aria-expanded={forgot} onclick={() => (forgot = !forgot)}>
            {t('login.forgot', lang)}
        </Button>
    </div>
    {#if forgot}
        <NoteBox tone="info" role="note" dataTest="login-forgot-text">{t('login.forgotText', lang)}</NoteBox>
    {/if}
</AuthShell>
