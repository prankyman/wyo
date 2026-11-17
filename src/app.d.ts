// See https://svelte.dev/docs/kit/types#app.d.ts for information about these interfaces.
declare global {
    namespace App {
        interface PageState {
            /**
             * The row opened in a page's side panel (`RowDetailsPanel`) by a press, so the list is not loaded again — the address
             * parameter that names it, its value, and the record the page read for that address.
             */
            details?: { param: string; id: string; value: unknown };
        }
    }
}

export {};
