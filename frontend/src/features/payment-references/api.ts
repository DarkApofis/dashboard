import "server-only";

import { PAGE_SIZE } from "@/lib/config";
import { apiRequest } from "@/lib/http-client";

import { STATUSES } from "./status";
import type {
  CreatePaymentReferenceBody,
  PaymentReference,
  PaymentReferenceListResponse,
  ReferenceQuery,
  StatusCounts,
} from "./types";

/** GET /payment-references */
export async function listPaymentReferences(query: ReferenceQuery): Promise<PaymentReferenceListResponse> {
  return apiRequest<PaymentReferenceListResponse>("/payment-references", {
    searchParams: {
      status: query.status,
      business_id: query.businessId,
      external_reference: query.externalReference,
      limit: query.limit,
      offset: query.offset,
    },
  });
}

/** GET /payment-references/{id} */
export async function getPaymentReference(id: number): Promise<PaymentReference> {
  return apiRequest<PaymentReference>(`/payment-references/${id}`);
}

/** POST /payment-references */
export async function createPaymentReference(body: CreatePaymentReferenceBody): Promise<PaymentReference> {
  return apiRequest<PaymentReference>("/payment-references", { method: "POST", body });
}

/** POST /payment-references/{id}/cancel */
export async function cancelPaymentReference(id: number): Promise<PaymentReference> {
  return apiRequest<PaymentReference>(`/payment-references/${id}/cancel`, { method: "POST" });
}

/**
 * Counts references per status, honouring the other active filters so the numbers
 * match what clicking a filter would show.
 *
 * The API has no aggregates endpoint, so this asks for one row per status and reads
 * the `total` the listing already returns. Counting the rows of the current page
 * instead would be wrong: a page holds at most PAGE_SIZE of them.
 *
 * This is also why the stat cards show counts rather than the money totals the design
 * sketched: a sum cannot be derived from a page, and inventing one would be worse
 * than leaving it out.
 */
export async function countReferencesByStatus(
  filters: Pick<ReferenceQuery, "businessId" | "externalReference">,
): Promise<StatusCounts> {
  const base = { ...filters, limit: 1, offset: 0 };

  const [all, ...perStatus] = await Promise.all([
    listPaymentReferences({ ...base }),
    ...STATUSES.map((status) => listPaymentReferences({ ...base, status })),
  ]);

  const counts = { all: all.total } as StatusCounts;
  STATUSES.forEach((status, index) => {
    counts[status] = perStatus[index]?.total ?? 0;
  });
  return counts;
}

/**
 * POST /webhooks/payments with a made-up provider notification for this reference.
 *
 * A review convenience, not a product feature. The amount is sent as a string so it
 * matches the stored one exactly: the API compares amounts with an exact decimal
 * type, and one cent of drift would be recorded as a mismatch instead of a payment.
 */
export async function simulateProviderPayment(reference: PaymentReference): Promise<void> {
  const suffix = Date.now().toString(36);
  await apiRequest<unknown>("/webhooks/payments", {
    method: "POST",
    body: {
      event_id: `evt_sim_${suffix}`,
      external_reference: reference.external_reference,
      amount: String(reference.amount),
      transaction_id: `tx_sim_${suffix}`,
      paid_at: new Date().toISOString(),
    },
  });
}

/** Builds the query the listing page was rendered from, for the request hint line. */
export function toQueryString(query: ReferenceQuery): string {
  const params = new URLSearchParams();
  if (query.businessId) {
    params.set("business_id", String(query.businessId));
  }
  if (query.status) {
    params.set("status", query.status);
  }
  if (query.externalReference) {
    params.set("external_reference", query.externalReference);
  }
  if (query.offset > 0) {
    params.set("offset", String(query.offset));
  }
  if (query.limit !== PAGE_SIZE) {
    params.set("limit", String(query.limit));
  }
  const rendered = params.toString();
  return rendered ? `?${rendered}` : "";
}
