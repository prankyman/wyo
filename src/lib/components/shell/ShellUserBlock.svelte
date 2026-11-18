<script lang="ts">
    import IconTooltip from '$lib/components/ui/IconTooltip.svelte';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import { loginInitials } from './user-initials';

    /**
     * The owner's block of the top bar: the login that opens «Мой профиль». Below 768 px the login is an avatar, a circle with its
     * first letter, with a tooltip that says what it opens; from 768 px up the login is written. «Выйти» is not here: it sits on
     * «Мой профиль».
     */
    interface IProps {
        login: string;
        /** The localized address of «Мой профиль». */
        profileHref: string;
        lang: TLocale;
    }

    let { login, profileHref, lang }: IProps = $props();
</script>

<IconTooltip
    name={t('shell.header.profile', lang)}
    description={t('shell.header.profileHint', lang, { login })}
    onlyBelowMd
    testId="app-user-tip"
>
    <a
        href={profileHref}
        class="inline-flex min-h-11 min-w-11 items-center justify-center text-sm text-white underline-offset-2 md:max-w-[14rem] md:justify-start md:py-3 md:hover:underline"
        aria-label={login}
        data-test="app-user-login"
        ><!-- The letter is drawn from the attribute, so the link's text stays the login alone. --><span
            class="inline-flex size-9 items-center justify-center rounded-full bg-white text-xs font-semibold text-brand-700 before:content-[attr(data-initials)] md:hidden"
            aria-hidden="true"
            data-initials={loginInitials(login)}
            data-test="app-user-avatar"
        ></span><span class="hidden min-w-0 truncate md:block" data-test="app-user-name">{login}</span></a
    >
</IconTooltip>
