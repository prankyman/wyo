import type { components } from '$lib/api/wyo-api';

/** One marketplace adapter as the wyo server describes it (`GET /api/v1/marketplaces`): its code, mark, key form and capabilities. */
export type TMarketplace = components['schemas']['Marketplace'];
export type TMarketplaceList = components['schemas']['MarketplaceList'];
/** One field of an adapter's key form; a `secret` one is drawn as a secret box. */
export type TKeyField = components['schemas']['KeyField'];

/** One seller account on one marketplace, with its key's health (`GET /api/v1/accounts`). The key itself never comes. */
export type TAccount = components['schemas']['Account'];
export type TAccountList = components['schemas']['AccountList'];
export type TAccountState = components['schemas']['AccountState'];
