export const page = {
    url: { pathname: '/', search: '', hash: '', searchParams: { get: (): string | null => null } },
    params: {},
    route: { id: null },
    /** No shallow history entry (`pushState`): a page's side panel opens only from its load's data. */
    state: {} as App.PageState
};

/** No navigation in progress - what `$app/state` says outside a navigation. */
export const navigating = { from: null, to: null, type: null, willUnload: null, delta: null, complete: null };
