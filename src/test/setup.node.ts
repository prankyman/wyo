// Setup for the `node` Vitest project (see vitest.config.ts).
//
// Deliberately minimal: no `@testing-library/jest-dom`, no `@testing-library/svelte`
// configure(), no ResizeObserver/EventSource shims. Those exist only to serve a DOM,
// and loading them here is what made every non-component test pay ~740ms of jsdom
// construction plus the testing-library import graph.
//
// Mock lifecycle is NOT handled globally — vitest.config.ts sets neither `clearMocks` nor
// `restoreMocks`, so a file whose assertions depend on call counts must call
// `vi.clearAllMocks()` in its own `beforeEach`, as most of the suite already does.
//

export {};
