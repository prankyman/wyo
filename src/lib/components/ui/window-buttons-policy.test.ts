import { describe, expect, it } from 'vitest';
import { markupTree, readMarkup, svelteFilesUnder, walkMarkup, type IMarkupNode } from '$test/support/svelte-markup';

/**
 * Enforces "Windows" — the window's buttons: in the maintainer's other repository 25 windows had once drawn their
 * buttons three ways — in the window's bottom band at the right, at the end of the content at the right, at the end of the content
 * at the left with the action first. Every window's buttons are `WindowButtons` (`src/lib/components/ui/WindowButtons.svelte`) in the `footer` of `ModalWindow`: the band that stands still under
 * the content, at the right, «Отмена» or «Закрыть» first and the window's own action last.
 *
 * It fails, in every `.svelte` file under `src/` (the test harnesses excepted): a button (`Button` or `<button>`) drawn in the
 * content of a card window (`ModalWindow` without `frame="none"`) — the heading row's controls (`heading`, the «?») excepted; a
 * window footer holding anything but one `WindowButtons`; and a `WindowButtons` drawn anywhere but a window's footer. A button that
 * a component inside the window draws (a search field's own «Найти») is the component's and is not seen here.
 */

const isButton = (node: IMarkupNode) => node.name === 'Button' || (node.kind === 'element' && node.name === 'button');
const isCardWindow = (node: IMarkupNode) => node.name === 'ModalWindow' && node.attrs.frame !== 'none';
const drawnIn = (snippet: string, chain: readonly IMarkupNode[]) => chain.some((node) => node.snippet === snippet);

/** `<file>:<line> <what>` of every place a window draws a button, or its buttons, against the rule. */
function offences(file: string, tree: readonly IMarkupNode[]): string[] {
    const found: string[] = [];
    for (const { node: window } of walkMarkup(tree)) {
        if (!isCardWindow(window)) continue;
        for (const part of window.children.filter((child) => child.snippet === 'footer')) {
            if (part.name !== 'WindowButtons') found.push(`${file}:${part.line} footer holds ${part.name}`);
        }
        for (const { node, ancestors } of walkMarkup(window.children)) {
            const chain = [...ancestors, node];
            if (isButton(node) && !drawnIn('footer', chain) && !drawnIn('heading', chain)) {
                found.push(`${file}:${node.line} button in the window's content`);
            }
        }
    }
    for (const { node, ancestors } of walkMarkup(tree)) {
        if (node.name !== 'WindowButtons') continue;
        const parent = ancestors.at(-1);
        if (!parent || !isCardWindow(parent) || node.snippet !== 'footer') {
            found.push(`${file}:${node.line} WindowButtons outside a window's footer`);
        }
    }
    return found;
}

const FILES = svelteFilesUnder('src').filter((file) => !file.startsWith('src/test/'));

// Every page of the interface is parsed: under a whole run's load that takes longer than a single test's 5 seconds.
describe('window buttons policy ("Windows": the buttons in the bottom band, at the right)', { timeout: 60_000 }, () => {
    it('draws every window button in WindowButtons in the window footer', () => {
        expect(FILES.flatMap((file) => offences(file, readMarkup(file)))).toEqual([]);
    });

    it('finds the defects it guards against and lets the heading and a side panel be', () => {
        const tree = markupTree(`
            <ModalWindow>
                {#snippet heading()}<HelpTooltip /><Button>?</Button>{/snippet}
                <div class="flex justify-end">{#if a}<Button type="submit">Сохранить</Button>{/if}</div>
                {#snippet footer()}<div><Button>Отмена</Button></div>{/snippet}
            </ModalWindow>
            <ModalWindow frame="none"><Button>Закрыть панель</Button></ModalWindow>
            <WindowButtons><Button>Печать</Button></WindowButtons>`);

        expect(offences('demo.svelte', tree)).toEqual([
            'demo.svelte:5 footer holds div',
            "demo.svelte:4 button in the window's content",
            "demo.svelte:8 WindowButtons outside a window's footer"
        ]);
    });
});
