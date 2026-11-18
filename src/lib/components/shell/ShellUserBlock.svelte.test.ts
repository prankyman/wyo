import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import ShellUserBlock from './ShellUserBlock.svelte';

/**
 * The owner's block of the top bar: the login that opens «Мой профиль», and below 768 px the same link as a round avatar with the
 * login's first letters. jsdom applies no media query, so both looks are in the markup, told apart by their classes.
 */

const PROPS = { login: 'owner', profileHref: '/account', lang: 'ru' as const };

describe('ShellUserBlock', () => {
    it('draws a link to «Мой профиль» named by the login, with the avatar below 768 px and the login from there up', () => {
        render(ShellUserBlock, { props: PROPS });

        const profile = screen.getByTestId('app-user-login');
        expect(profile).toHaveAttribute('href', '/account');
        expect(profile).toHaveAccessibleName('owner');
        expect(profile).toHaveClass('min-h-11', 'min-w-11');
        expect(screen.getByTestId('app-user-avatar')).toHaveAttribute('data-initials', 'O');
        expect(screen.getByTestId('app-user-avatar')).toHaveClass('md:hidden');
        expect(screen.getByTestId('app-user-name')).toHaveClass('hidden', 'truncate', 'md:block');
        expect(screen.getByTestId('app-user-name')).toHaveTextContent('owner');
    });

    it('says on the avatar’s tooltip who is signed in and what the link opens', async () => {
        render(ShellUserBlock, { props: PROPS });

        await fireEvent.mouseEnter(screen.getByTestId('app-user-tip'));

        const tip = screen.getByTestId('app-user-tip-tooltip');
        expect(tip).toHaveTextContent('Мой профиль');
        expect(tip).toHaveTextContent('Вы вошли как owner. Нажмите, чтобы открыть «Мой профиль»');
    });
});
