#!/usr/bin/env node

/**
 * Coverage enforcement for the interface.
 *
 * `THRESHOLD` below is the single place the floor is declared. The constitution names this constant rather than repeating the
 * figure, so changing the number is a one-line edit here and the rule can never disagree with the gate.
 *
 * The gate has two tiers, and both are hard:
 *
 *   1. AGGREGATE.      The whole interface must stay at or above THRESHOLD statements.
 *   2. EVERY FILE.     Every file the report measures must reach THRESHOLD. No category is exempt for being a route, a component
 *                      or a script.
 *
 * There is no advisory tier: a warning nobody has to act on is technical debt with a progress bar. A file that genuinely cannot
 * be tested is removed from MEASUREMENT, in `coverage.exclude` in vitest.config.ts, with a written reason saying what a test
 * would be asserting and why that is worthless. It is not left sitting below the bar.
 *
 * The Go side has its own gate (`tools/covercheck`, run by `scripts/go/coverage-enforce.sh`) with the same two tiers.
 */
const fs = require('fs');
const path = require('path');

const SUMMARY_PATH = path.join(process.cwd(), 'coverage', 'coverage-summary.json');
const FINAL_PATH = path.join(process.cwd(), 'coverage', 'coverage-final.json');
const THRESHOLD = 84;

/**
 * Files excluded from this gate on top of whatever `vitest.config.ts` already keeps out of the report. Empty on purpose:
 * `coverage.exclude` in vitest.config.ts is the single place exclusions are declared, and every entry there carries its own
 * written reason. Before adding anything here, check it is not already covered by a test you have not run.
 */
const EXCLUDED_FILES = [];

const coveragePath = fs.existsSync(SUMMARY_PATH) ? SUMMARY_PATH : fs.existsSync(FINAL_PATH) ? FINAL_PATH : null;

if (!coveragePath) {
    console.error(`Coverage file not found. Expected either:\n  - ${SUMMARY_PATH}\n  - ${FINAL_PATH}`);
    process.exit(1);
}

let data;
try {
    data = JSON.parse(fs.readFileSync(coveragePath, 'utf8'));
} catch (err) {
    console.error('Failed to parse coverage JSON:', err);
    process.exit(1);
}

/** Statement counters for one file, from either report shape. */
const statementCounts = (metrics) => {
    if (metrics?.statements?.total != null) {
        return { covered: metrics.statements.covered, total: metrics.statements.total };
    }
    const statements = metrics?.s ? Object.values(metrics.s) : [];
    return { covered: statements.filter((n) => n > 0).length, total: statements.length };
};

/**
 * Exact percentage, NOT rounded: rounding would quietly buy every file up to half a point of slack. Round for display, never
 * for the comparison.
 */
const computePct = (metrics) => {
    const linePct = metrics?.lines?.pct;
    if (linePct != null) return linePct;
    const { covered, total } = statementCounts(metrics);
    if (total === 0) return null;
    return (covered / total) * 100;
};

const failures = [];
let aggCovered = 0;
let aggTotal = 0;

for (const [filePath, metrics] of Object.entries(data)) {
    if (filePath === 'total') continue;
    if (EXCLUDED_FILES.some((excluded) => filePath.includes(excluded))) continue;

    const { covered, total } = statementCounts(metrics);
    aggCovered += covered;
    aggTotal += total;

    const pct = computePct(metrics);
    if (pct == null) continue;
    if (pct >= THRESHOLD) continue;

    const relative = filePath.replace(process.cwd() + '/', '');
    failures.push({ filePath: relative, pct });
}

const aggregatePct = aggTotal === 0 ? 100 : (aggCovered / aggTotal) * 100;

let failed = false;

if (aggregatePct < THRESHOLD) {
    console.error(`Aggregate statement coverage ${aggregatePct.toFixed(2)}% is below the required ${THRESHOLD}%.`);
    failed = true;
}

if (failures.length > 0) {
    console.error(`${failures.length} file(s) below ${THRESHOLD}%:`);
    failures
        .sort((a, b) => a.pct - b.pct)
        .forEach(({ filePath, pct }) => console.error(`  - ${filePath}: ${pct.toFixed(1)}% (required: ${THRESHOLD}%)`));
    console.error(
        '\nCover the behaviour a user would notice, or - if the file genuinely has no testable\n' +
            'surface - exclude it in vitest.config.ts `coverage.exclude` with a written reason.\n' +
            'There is no warning tier to fall back on.'
    );
    failed = true;
}

if (failed) process.exit(1);

console.log(
    `Coverage enforcement passed: aggregate ${aggregatePct.toFixed(2)}% (>= ${THRESHOLD}%), ` +
        `every measured file >= ${THRESHOLD}%`
);
