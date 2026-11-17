/**
 * Puts `text` on the clipboard: the one copy of the interface, used by `CopyButton`. It answers whether the
 * browser took it — the clipboard is refused outside a secure page, without the person's permission, or in an old browser —
 * so the screen can say «Скопировано» or ask the person to copy by hand.
 */
export async function copyToClipboard(text: string): Promise<boolean> {
    try {
        await navigator.clipboard.writeText(text);
        return true;
    } catch {
        return false;
    }
}
