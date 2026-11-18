import { SvelteSet } from 'svelte/reactivity';
import { initialOpenSections, navGroups, NAV_SECTIONS, sectionOfPath } from './nav-items';
import type { INavItem, TNavSection } from './types';

/**
 * Which groups of the menu are open — one state per browser tab, shared by the left menu and the «Ещё» sheet, so a window resized
 * across the width where the left menu appears keeps it. The layout makes it, so it lives exactly as long as the layout: a reload or
 * a new tab starts afresh, and nothing is kept in the browser's storage, in a cookie or on the server.
 *
 * - On a new visit the open page's group is open, and on the home page the first group, the rest closed (`initialOpenSections`).
 * - Moving to another page opens that page's group and closes nothing: what the person opened or closed stays.
 * - Any group, the open page's included, opens and closes by hand.
 */
export class NavGroupsState {
    readonly #open = new SvelteSet<TNavSection>();
    /** The page the menu last followed. */
    #path: string;

    /** The new visit: the menu first drawn on `path` with `items`, folded into `sections`. */
    constructor(path: string, items: readonly INavItem[], sections: readonly TNavSection[] = NAV_SECTIONS) {
        this.#path = path;
        const drawn = navGroups(items, sections).groups.map((group) => group.section);
        for (const section of initialOpenSections(drawn, sectionOfPath(items, path), path === '/')) {
            this.#open.add(section);
        }
    }

    isOpen(section: TNavSection): boolean {
        return this.#open.has(section);
    }

    /** The person opened or closed a group. */
    setOpen(section: TNavSection, open: boolean): void {
        if (open) this.#open.add(section);
        else this.#open.delete(section);
    }

    /**
     * The menu is drawn for `path`: another page than the one followed last opens that page's group (none for a page of no group
     * or outside the menu). The same path again changes nothing, so both menus may report it.
     */
    follow(path: string, items: readonly INavItem[]): void {
        if (path === this.#path) return;
        this.#path = path;
        const current = sectionOfPath(items, path);
        if (current) this.#open.add(current);
    }
}
