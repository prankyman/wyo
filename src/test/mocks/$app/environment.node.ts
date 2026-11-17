// The node-project counterpart of ./environment.ts.
//
// `$lib/i18n/i18n.ts` runs `document.documentElement.dataset.activeLocales` at module
// load time inside an `if (browser)` block. Tests that never touch a DOM run in the
// `node` Vitest project, where `document` does not exist, so importing anything that
// transitively pulls in i18n would throw `ReferenceError: document is not defined`.
//
// `browser = false` is also the semantically correct value for those tests: they cover
// server and isomorphic code, which must pass an explicit
// `serverLocale` to `t()` rather than infer one from the URL.
export const browser = false;
// A test is not the development server: the version line reads the deployment's number or none
// (`src/lib/server/app-version.ts`). A test that needs `dev` true mocks `$app/environment` itself.
export const dev = false;
