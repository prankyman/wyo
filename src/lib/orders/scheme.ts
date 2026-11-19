import type { TOrderScheme } from '$lib/components/orders/types';

/**
 * Whether the seller packs an order of this scheme: FBS and Express, whose orders the seller packs in a warehouse of its own and
 * hands over. The other schemes (FBO, FBY) are packed by the marketplace, so such an order has no warehouse of the seller's and
 * nothing to hand over.
 */
export function sellerPacks(scheme: TOrderScheme): boolean {
    return scheme === 'fbs' || scheme === 'express';
}
