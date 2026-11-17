import { fireEvent, render, screen } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ShowOnceSecret from './ShowOnceSecret.svelte';

vi.mock('$lib/i18n/i18n.js', async () => (await import('$test/support/i18n')).createI18nMock());
vi.mock('$lib/i18n/i18n', async () => (await import('$test/support/i18n')).createI18nMock());

/**
 * The box of a secret shown ONCE: the callers — a token, a key, an address — test only what they put into it; the box, its note and its copy buttons are proved here.
 */
afterEach(() => {
    vi.unstubAllGlobals();
});

const NOTE = 'Copy it now: it is shown once and cannot be shown again.';

const base = {
    lang: 'ru' as const,
    dataTest: 'once',
    note: NOTE,
    items: [{ value: 'secret-1' }]
};

describe('ShowOnceSecret', () => {
    it('draws one value in the green box with the note, a copy button and the shared copy words', () => {
        render(ShowOnceSecret, { props: base });

        const box = screen.getByTestId('once');
        expect(box).toHaveAttribute('data-tone', 'success');
        expect(box).toHaveAttribute('role', 'status');
        expect(screen.getByTestId('once-once')).toHaveTextContent(NOTE);
        expect(screen.getByTestId('once-once')).toHaveClass('text-stop-800');
        expect(screen.getByTestId('once-value-0')).toHaveTextContent('secret-1');
        expect(screen.getByTestId('once-value-0')).toHaveClass('font-mono', 'select-all');
        expect(screen.getByTestId('once-value-0-copy')).toHaveTextContent('Скопировать');
        expect(screen.queryByTestId('once-help-button')).not.toBeInTheDocument();
    });

    it('draws several values, each with its label, its own test id and its own copy button', async () => {
        const writeText = vi.fn(async () => undefined);
        vi.stubGlobal('navigator', { clipboard: { writeText } });
        render(ShowOnceSecret, {
            props: {
                ...base,
                items: [
                    { label: 'Login', value: 'login-abcd1234', testId: 'made-login' },
                    { label: 'Password', value: 'fake-password-0001', testId: 'made-password' }
                ]
            }
        });

        expect(screen.getByText('Login')).toBeInTheDocument();
        expect(screen.getByText('Password')).toBeInTheDocument();
        expect(screen.getByTestId('made-login')).toHaveTextContent('login-abcd1234');
        expect(screen.getByTestId('made-password')).toHaveTextContent('fake-password-0001');
        await fireEvent.click(screen.getByTestId('made-password-copy'));
        expect(writeText).toHaveBeenCalledTimes(1);
        expect(writeText).toHaveBeenCalledWith('fake-password-0001');
        expect(await screen.findByText('Скопировано.')).toBeInTheDocument();
    });

    it('says when the browser refused the clipboard, in words that tell to copy by hand', async () => {
        vi.stubGlobal('navigator', {
            clipboard: { writeText: vi.fn(async () => Promise.reject(new Error('denied'))) }
        });
        render(ShowOnceSecret, { props: base });

        await fireEvent.click(screen.getByTestId('once-value-0-copy'));

        expect(await screen.findByText(/Браузер не дал скопировать/)).toBeInTheDocument();
    });

    it('gives a value its own copy button words and a «?» beside its label when the caller names them', async () => {
        render(ShowOnceSecret, {
            props: {
                ...base,
                items: [
                    {
                        label: 'Linux',
                        value: 'sudo ./agent',
                        testId: 'cmd',
                        copyLabel: 'Copy the command',
                        help: 'Run it as root.'
                    }
                ]
            }
        });

        expect(screen.getByTestId('cmd-copy')).toHaveTextContent('Copy the command');
        await fireEvent.click(screen.getByTestId('cmd-help-button'));
        expect(screen.getByTestId('cmd-help-tooltip')).toHaveTextContent('Run it as root.');
    });

    it('draws a title with a «?» when given, and the live role and note test id the caller names', async () => {
        render(ShowOnceSecret, {
            props: { ...base, title: 'Copy it now', help: 'Why it is shown once.', role: 'none', noteTestId: 'my-once' }
        });

        expect(screen.getByRole('heading', { name: 'Copy it now' })).toBeInTheDocument();
        await fireEvent.click(screen.getByTestId('once-help-button'));
        expect(screen.getByTestId('once-help-tooltip')).toHaveTextContent('Why it is shown once.');
        expect(screen.getByTestId('once')).not.toHaveAttribute('role');
        expect(screen.getByTestId('my-once')).toHaveTextContent(NOTE);
        expect(screen.queryByTestId('once-once')).not.toBeInTheDocument();
    });

    it('draws the caller’s own content after the values', () => {
        const next = createRawSnippet(() => ({ render: () => '<p data-test="own-next">Then press Next.</p>' }));
        render(ShowOnceSecret, { props: { ...base, children: next } });

        const value = screen.getByTestId('once-value-0');
        const own = screen.getByTestId('own-next');
        expect(own).toHaveTextContent('Then press Next.');
        expect(value.compareDocumentPosition(own) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    });

    it('draws only the note when there are no values', () => {
        render(ShowOnceSecret, { props: { ...base, items: [] } });

        expect(screen.getByTestId('once-once')).toHaveTextContent(NOTE);
        expect(screen.queryByTestId('once-value-0')).not.toBeInTheDocument();
    });
});
