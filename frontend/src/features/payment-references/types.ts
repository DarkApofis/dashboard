/** The lifecycle states the API reports. */
export type PaymentReferenceStatus = "pending" | "paid" | "expired" | "cancelled";

/**
 * A payment reference exactly as the API returns it.
 *
 * `status` is the effective status: a pending reference past its expiration date comes
 * back as "expired" even before the periodic job writes it, so nothing here has to
 * recompute it.
 *
 * `amount` is a JSON number, and it is only ever formatted. See formatCOP.
 */
export interface PaymentReference {
  id: number;
  business_id: number;
  external_reference: string;
  amount: number;
  description: string;
  status: PaymentReferenceStatus;
  expires_at: string;
  created_at: string;
  updated_at: string;
  paid_at: string | null;
  cancelled_at: string | null;
  transaction_id?: string;
  /** Set when a webhook needed human reconciliation (late payment, wrong amount). */
  requires_review: boolean;
  review_reason?: string;
}

export interface PaymentReferenceListResponse {
  items: PaymentReference[];
  total: number;
  limit: number;
  offset: number;
}

/**
 * The create payload.
 *
 * `amount` is sent as a string on purpose. The API accepts both a JSON number and a
 * decimal string, and a string is the only one that cannot be mangled by the
 * browser: JSON.stringify would turn the amount into a binary float first.
 */
export interface CreatePaymentReferenceBody {
  business_id: number;
  external_reference: string;
  amount: string;
  description?: string;
  expires_at: string;
}

/** The filters the listing supports, mirroring the API query parameters. */
export interface ReferenceQuery {
  status?: PaymentReferenceStatus;
  businessId?: number;
  externalReference?: string;
  limit: number;
  offset: number;
}

/** How many references match each status, for the filter counters and the stats. */
export type StatusCounts = Record<PaymentReferenceStatus | "all", number>;
