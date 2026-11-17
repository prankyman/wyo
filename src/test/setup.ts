import '@testing-library/jest-dom/vitest';
import { configure } from '@testing-library/svelte';
import { vi } from 'vitest';

configure({ testIdAttribute: 'data-test' });

// Mock ResizeObserver for jsdom environment
class MockResizeObserver {
    observe = vi.fn();
    disconnect = vi.fn();
    unobserve = vi.fn();
}

(global as typeof globalThis).ResizeObserver = MockResizeObserver;

// jsdom has no EventSource implementation. Components that subscribe to server-sent events
// open one on mount; tests that don't specifically exercise SSE behavior just need it to exist
// and be inert. Tests that DO care about SSE events stub their own richer FakeEventSource.
class MockEventSource {
    onmessage: ((event: { data: string }) => void) | null = null;
    onerror: (() => void) | null = null;
    constructor(public url: string) {}
    close() {}
}

(global as typeof globalThis).EventSource = MockEventSource as unknown as typeof EventSource;

// jsdom implements no Web Animations API, so `element.animate()` is simply absent. Svelte's
// built-in transitions (`fly`, `slide`, `fade`) call it on every enter and leave, so ANY test
// that opens a component with a transition dies with "element.animate is not a function" — a
// failure about the browser, not about the component under test.
//
// The stub returns the smallest object Svelte reads back: something cancellable with an
// `onfinish` hook, which it calls immediately so the transition is over by the time the test
// asserts. That is the right shape here — jsdom cannot show an animation, and a test asserting
// the state DURING one would be asserting a number nobody can see (see the sveltekit-testing
// skill, §10.1, on stubbing the browser rather than softening the code).
if (typeof Element !== 'undefined' && !Element.prototype.animate) {
    Element.prototype.animate = function stubAnimate(): Animation {
        const animation = {
            cancel: () => {},
            finish: () => {},
            play: () => {},
            pause: () => {},
            currentTime: 0,
            playState: 'finished',
            startTime: 0,
            finished: Promise.resolve(),
            set onfinish(handler: (() => void) | null) {
                handler?.();
            },
            get onfinish() {
                return null;
            }
        };
        return animation as unknown as Animation;
    };
}

// jsdom has no layout and therefore no scrolling: window.scrollTo() is an unimplemented
// stub that logs on every call. A spy-able no-op instead, which is also what lets a test
// assert `expect(window.scrollTo).toHaveBeenCalledWith({ top: 0, ... })`.
if (typeof window !== 'undefined') {
    window.scrollTo = vi.fn();
}

// An element's own scrollTo() is missing from jsdom altogether (a menu that scrolls itself smoothly). The
// stub sets the position at once, as the browser ends up after the smooth scroll.
if (typeof Element !== 'undefined' && !Element.prototype.scrollTo) {
    Element.prototype.scrollTo = function stubScrollTo(this: Element, options?: ScrollToOptions | number): void {
        if (typeof options === 'object' && options.top !== undefined) this.scrollTop = options.top;
    };
}

// jsdom doesn't implement real anchor navigation, so clicking a same-app <a href>
// in a test logs "Not implemented: navigation to another Document". Prevent the
// default action so jsdom never attempts it - matches real usage, where SvelteKit's
// router intercepts these clicks before the browser navigates.
if (typeof document !== 'undefined') {
    document.addEventListener('click', (event) => {
        const anchor = (event.target as HTMLElement | null)?.closest('a[href]');
        if (anchor) {
            event.preventDefault();
        }
    });

    // jsdom doesn't implement real form submission either, so a form without its own
    // preventDefault (e.g. submitted via requestSubmit()) falls through to jsdom's
    // unimplemented submit/navigation stubs, logging "Not implemented: HTMLFormElement's
    // requestSubmit() method" / "Not implemented: navigation to another Document".
    // Calling preventDefault() here is a no-op when a component's own handler already
    // does it, and only acts as a safety net otherwise.
    document.addEventListener('submit', (event) => {
        event.preventDefault();
    });
}
