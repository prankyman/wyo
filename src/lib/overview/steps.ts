import type { TExternalApiState } from '$lib/components/external-api/types';
import { stageSteps, type IStepStripStep } from '$lib/components/ui/step-strip';
import { t } from '$lib/i18n/i18n';
import type { TLocale } from '$lib/i18n/types';

/** The seller's four steps, in order: the installation, the marketplaces' keys, the orders, and — only if wanted — the external API. */
export const OVERVIEW_STEP_KEYS = ['install', 'keys', 'orders', 'api'] as const;
export type TOverviewStep = (typeof OVERVIEW_STEP_KEYS)[number];

/**
 * The step the seller stands at: the keys until an account is added; then the orders until «Внешний API» is filled in — the step is
 * optional, so an empty setting never holds the seller back; then the external API. A setting filled before any account does not
 * skip the keys.
 */
export function currentStep(accounts: number, externalApiState: TExternalApiState): TOverviewStep {
    if (accounts === 0) return 'keys';
    return externalApiState === 'empty' ? 'orders' : 'api';
}

/**
 * The strip of steps of «Обзор» and the line «Сейчас: …» under it. A person who is signed in has finished the installation, so the
 * step they stand at is the next one; a later plan, which builds the screen of that step, moves `current` on when its work is done.
 */
export function overviewSteps(
    lang: TLocale,
    current: TOverviewStep = 'keys'
): { steps: IStepStripStep[]; now: string } {
    return {
        steps: stageSteps(OVERVIEW_STEP_KEYS, current, (key) => t(`overview.step.${key}`, lang)),
        now: t(`overview.now.${current}`, lang)
    };
}
