import "server-only";

import { apiRequest } from "@/lib/http-client";

import type { Business, BusinessListResponse } from "./types";

/**
 * Lists every business. The endpoint is not paginated because there is one row per
 * client company, so the whole table fits in the selector of the create form.
 */
export async function listBusinesses(): Promise<Business[]> {
  const response = await apiRequest<BusinessListResponse>("/businesses");
  return response.items;
}

/** Indexes businesses by id so a table row can show a name instead of a number. */
export function indexBusinesses(businesses: Business[]): Map<number, string> {
  return new Map(businesses.map((business) => [business.id, business.name]));
}
